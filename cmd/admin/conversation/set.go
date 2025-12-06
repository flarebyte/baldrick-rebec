package conversation

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
	flagConvID    string
	flagConvTitle string
	flagConvDesc  string
	flagConvNotes string
	flagConvProj  string
	flagConvTags  []string
	flagConvRole  string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a conversation (by id)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagConvTitle) == "" {
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
		conv := &pgdao.Conversation{ID: flagConvID, Title: flagConvTitle}
		if strings.TrimSpace(flagConvRole) != "" {
			conv.RoleName = strings.TrimSpace(flagConvRole)
		}
		if flagConvDesc != "" {
			conv.Description = sql.NullString{String: flagConvDesc, Valid: true}
		}
		if flagConvNotes != "" {
			conv.Notes = sql.NullString{String: flagConvNotes, Valid: true}
		}
		if flagConvProj != "" {
			conv.Project = sql.NullString{String: flagConvProj, Valid: true}
		}
		if len(flagConvTags) > 0 {
			conv.Tags = parseTags(flagConvTags)
		}
		if err := pgdao.UpsertConversation(ctx, db, conv); err != nil {
			return err
		}
		// Human line
		fmt.Fprintf(os.Stderr, "conversation upserted id=%s title=%q\n", conv.ID, conv.Title)
		// JSON
		out := map[string]any{
			"status": "upserted",
			"id":     conv.ID,
			"title":  conv.Title,
		}
		if conv.Project.Valid {
			out["project"] = conv.Project.String
		}
		if conv.Created.Valid {
			out["created"] = conv.Created.Time.Format(time.RFC3339Nano)
		}
		if conv.Updated.Valid {
			out["updated"] = conv.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	ConversationCmd.AddCommand(setCmd)
	setCmd.Flags().StringVar(&flagConvID, "id", "", "Conversation UUID (optional; when omitted, a new id is generated)")
	setCmd.Flags().StringVar(&flagConvTitle, "title", "", "Title (required)")
	setCmd.Flags().StringVar(&flagConvDesc, "description", "", "Plain text description")
	setCmd.Flags().StringVar(&flagConvNotes, "notes", "", "Markdown notes")
	setCmd.Flags().StringVar(&flagConvProj, "project", "", "Project name (e.g. GitHub repo)")
	setCmd.Flags().StringSliceVar(&flagConvTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
	setCmd.Flags().StringVar(&flagConvRole, "role", "", "Role name (optional; defaults to 'user')")
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
