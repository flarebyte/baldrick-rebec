package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagStoreName       string
	flagStoreRole       string
	flagStoreTitle      string
	flagStoreDesc       string
	flagStoreMotivation string
	flagStoreSecurity   string
	flagStorePrivacy    string
	flagStoreNotes      string
	flagStoreTags       []string
	flagStoreType       string
	flagStoreScope      string
	flagStoreLifecycle  string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a store (by name + role)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagStoreName) == "" {
			return errors.New("--name is required")
		}
		if strings.TrimSpace(flagStoreRole) == "" {
			return errors.New("--role is required")
		}
		if strings.TrimSpace(flagStoreTitle) == "" {
			return errors.New("--title is required")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		s := &pgdao.Store{
			Name:     flagStoreName,
			RoleName: flagStoreRole,
			Title:    flagStoreTitle,
		}
		if flagStoreDesc != "" {
			s.Description = sql.NullString{String: flagStoreDesc, Valid: true}
		}
		if flagStoreMotivation != "" {
			s.Motivation = sql.NullString{String: flagStoreMotivation, Valid: true}
		}
		if flagStoreSecurity != "" {
			s.Security = sql.NullString{String: flagStoreSecurity, Valid: true}
		}
		if flagStorePrivacy != "" {
			s.Privacy = sql.NullString{String: flagStorePrivacy, Valid: true}
		}
		if flagStoreNotes != "" {
			s.Notes = sql.NullString{String: flagStoreNotes, Valid: true}
		}
		if flagStoreType != "" {
			s.StoreType = sql.NullString{String: flagStoreType, Valid: true}
		}
		if flagStoreScope != "" {
			s.Scope = sql.NullString{String: strings.ToLower(flagStoreScope), Valid: true}
		}
		if flagStoreLifecycle != "" {
			s.Lifecycle = sql.NullString{String: strings.ToLower(flagStoreLifecycle), Valid: true}
		}
		if len(flagStoreTags) > 0 {
			s.Tags = parseTags(flagStoreTags)
		}

		if err := pgdao.UpsertStore(ctx, db, s); err != nil {
			return err
		}

		// stderr summary
		fmt.Fprintf(os.Stderr, "store upserted name=%q role=%q id=%s\n", s.Name, s.RoleName, s.ID)
		// stdout JSON
		out := map[string]any{
			"status": "upserted",
			"id":     s.ID,
			"name":   s.Name,
			"role":   s.RoleName,
			"title":  s.Title,
		}
		if s.Created.Valid {
			out["created"] = s.Created.Time.Format(time.RFC3339Nano)
		}
		if s.Updated.Valid {
			out["updated"] = s.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	StoreCmd.AddCommand(setCmd)
	setCmd.Flags().StringVar(&flagStoreName, "name", "", "Store unique name within role (required)")
	setCmd.Flags().StringVar(&flagStoreRole, "role", "", "Role name (required)")
	setCmd.Flags().StringVar(&flagStoreTitle, "title", "", "Title (required)")
	setCmd.Flags().StringVar(&flagStoreDesc, "description", "", "Plain text description")
	setCmd.Flags().StringVar(&flagStoreMotivation, "motivation", "", "Motivation text")
	setCmd.Flags().StringVar(&flagStoreSecurity, "security", "", "Security notes")
	setCmd.Flags().StringVar(&flagStorePrivacy, "privacy", "", "Privacy notes")
	setCmd.Flags().StringVar(&flagStoreNotes, "notes", "", "Markdown-formatted notes")
	setCmd.Flags().StringSliceVar(&flagStoreTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
	setCmd.Flags().StringVar(&flagStoreType, "type", "", "Store type (e.g., journal, testcase, blackboard)")
	setCmd.Flags().StringVar(&flagStoreScope, "scope", "", "Scope: conversation, shared, project, task")
	setCmd.Flags().StringVar(&flagStoreLifecycle, "lifecycle", "", "Lifecycle: permanent, yearly, quarterly, monthly, weekly, daily")
}

// parseTags converts k=v pairs (or bare keys) into a map.
func parseTags(items []string) map[string]any {
	if len(items) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, raw := range items {
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if eq := strings.IndexByte(p, '='); eq > 0 {
				k := strings.TrimSpace(p[:eq])
				v := strings.TrimSpace(p[eq+1:])
				if k != "" {
					out[k] = v
				}
			} else {
				out[p] = true
			}
		}
	}
	return out
}
