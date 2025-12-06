package topic

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
	flagTopicName  string
	flagTopicRole  string
	flagTopicTitle string
	flagTopicDesc  string
	flagTopicNotes string
	flagTopicTags  []string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a topic (by name + role)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTopicName) == "" {
			return errors.New("--name is required")
		}
		if strings.TrimSpace(flagTopicRole) == "" {
			return errors.New("--role is required")
		}
		if strings.TrimSpace(flagTopicTitle) == "" {
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

		t := &pgdao.Topic{Name: flagTopicName, RoleName: flagTopicRole, Title: flagTopicTitle}
		if flagTopicDesc != "" {
			t.Description = sql.NullString{String: flagTopicDesc, Valid: true}
		}
		if flagTopicNotes != "" {
			t.Notes = sql.NullString{String: flagTopicNotes, Valid: true}
		}
		if len(flagTopicTags) > 0 {
			t.Tags = parseTags(flagTopicTags)
		}
		if err := pgdao.UpsertTopic(ctx, db, t); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "topic upserted name=%q role=%q\n", t.Name, t.RoleName)
		out := map[string]any{"status": "upserted", "name": t.Name, "role": t.RoleName, "title": t.Title}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		if t.Updated.Valid {
			out["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TopicCmd.AddCommand(setCmd)
	setCmd.Flags().StringVar(&flagTopicName, "name", "", "Topic unique name within role (required)")
	setCmd.Flags().StringVar(&flagTopicRole, "role", "", "Role name (required)")
	setCmd.Flags().StringVar(&flagTopicTitle, "title", "", "Title (required)")
	setCmd.Flags().StringVar(&flagTopicDesc, "description", "", "Plain text description")
	setCmd.Flags().StringVar(&flagTopicNotes, "notes", "", "Markdown-formatted notes")
	setCmd.Flags().StringSliceVar(&flagTopicTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
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
