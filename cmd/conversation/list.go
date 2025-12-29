package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagConvListProject string
	flagConvListLimit   int
	flagConvListOffset  int
	flagConvListMax     int
	flagConvListOutput  string
	flagConvListRole    string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List conversations (optionally filter by project)",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		effLimit := flagConvListMax
		if effLimit <= 0 {
			effLimit = flagConvListLimit
		}
		if strings.TrimSpace(flagConvListRole) == "" {
			return errors.New("--role is required")
		}
		rows, err := pgdao.ListConversations(ctx, db, flagConvListProject, flagConvListRole, effLimit, flagConvListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "conversations: %d\n", len(rows))
		if strings.ToLower(strings.TrimSpace(flagConvListOutput)) == "json" {
			arr := make([]map[string]any, 0, len(rows))
			for _, c := range rows {
				item := map[string]any{"id": c.ID, "title": c.Title}
				if c.Project.Valid {
					item["project"] = c.Project.String
				}
				if c.Created.Valid {
					item["created"] = c.Created.Time.Format(time.RFC3339Nano)
				}
				if len(c.Tags) > 0 {
					item["tags"] = c.Tags
				}
				arr = append(arr, item)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "TITLE", "PROJECT", "UPDATED"})
		for _, c := range rows {
			project := ""
			if c.Project.Valid {
				project = c.Project.String
			}
			updated := ""
			if c.Updated.Valid {
				updated = c.Updated.Time.Format(time.RFC3339)
			}
			table.Append([]string{c.ID, c.Title, project, updated})
		}
		table.Render()
		return nil
	},
}

func init() {
	ConversationCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&flagConvListProject, "project", "", "Filter by project")
	listCmd.Flags().IntVar(&flagConvListLimit, "limit", 100, "Max rows (deprecated; prefer --max-results)")
	listCmd.Flags().IntVar(&flagConvListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().IntVar(&flagConvListMax, "max-results", 20, "Max results to return (default 20)")
	listCmd.Flags().StringVar(&flagConvListOutput, "output", "table", "Output format: table or json")
	listCmd.Flags().StringVar(&flagConvListRole, "role", "", "Role name (required)")
}
