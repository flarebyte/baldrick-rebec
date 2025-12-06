package tag

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
	flagTagListLimit  int
	flagTagListOffset int
	flagTagListOutput string
	flagTagListRole   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags (paginated)",
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
		if strings.TrimSpace(flagTagListRole) == "" {
			return errors.New("--role is required")
		}
		ts, err := pgdao.ListTags(ctx, db, flagTagListRole, flagTagListLimit, flagTagListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "tags: %d\n", len(ts))
		out := strings.ToLower(strings.TrimSpace(flagTagListOutput))
		if out == "json" {
			arr := make([]map[string]any, 0, len(ts))
			for _, t := range ts {
				item := map[string]any{"name": t.Name, "title": t.Title}
				if t.Created.Valid {
					item["created"] = t.Created.Time.Format(time.RFC3339Nano)
				}
				if t.Updated.Valid {
					item["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
				}
				if t.Description.Valid && t.Description.String != "" {
					item["description"] = t.Description.String
				}
				if t.Notes.Valid && t.Notes.String != "" {
					item["notes"] = t.Notes.String
				}
				arr = append(arr, item)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		// table default
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"NAME", "TITLE", "UPDATED"})
		for _, t := range ts {
			updated := ""
			if t.Updated.Valid {
				updated = t.Updated.Time.Format(time.RFC3339)
			}
			table.Append([]string{t.Name, t.Title, updated})
		}
		table.Render()
		return nil
	},
}

func init() {
	TagCmd.AddCommand(listCmd)
	listCmd.Flags().IntVar(&flagTagListLimit, "limit", 100, "Max number of rows")
	listCmd.Flags().IntVar(&flagTagListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().StringVar(&flagTagListOutput, "output", "table", "Output format: table or json")
	listCmd.Flags().StringVar(&flagTagListRole, "role", "", "Role name (required)")
}
