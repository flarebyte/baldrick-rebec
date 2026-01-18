package stickie

import (
	"context"
	"encoding/json"
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
	flagStListLimit      int
	flagStListOffset     int
	flagStListOutput     string
	flagStListBlackboard string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stickies (optionally filter by blackboard or topic)",
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
		ss, err := pgdao.ListStickies(ctx, db, strings.TrimSpace(flagStListBlackboard), flagStListLimit, flagStListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "stickies: %d\n", len(ss))
		out := strings.ToLower(strings.TrimSpace(flagStListOutput))
		if out == "json" {
			arr := make([]map[string]any, 0, len(ss))
			for _, s := range ss {
				item := map[string]any{"id": s.ID, "blackboard_id": s.BlackboardID, "edit_count": s.EditCount}
				// topics removed; use labels instead
				if s.Note.Valid && s.Note.String != "" {
					item["note"] = s.Note.String
				}
				if s.Code.Valid && s.Code.String != "" {
					item["code"] = s.Code.String
				}
				if len(s.Labels) > 0 {
					item["labels"] = s.Labels
				}
				if s.Updated.Valid {
					item["updated"] = s.Updated.Time.Format(time.RFC3339Nano)
				}
				if s.Name.Valid && s.Name.String != "" {
					item["name"] = s.Name.String
				}
				if s.Archived {
					item["archived"] = true
				}
				arr = append(arr, item)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		// table default
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "BLACKBOARD", "NAME", "UPDATED", "EDIT#"})
		for _, s := range ss {
			updated := ""
			if s.Updated.Valid {
				updated = s.Updated.Time.Format(time.RFC3339)
			}
			name := ""
			if s.Name.Valid {
				name = s.Name.String
			}
			table.Append([]string{s.ID, s.BlackboardID, name, updated, fmt.Sprintf("%d", s.EditCount)})
		}
		table.Render()
		return nil
	},
}

func init() {
	StickieCmd.AddCommand(listCmd)
	listCmd.Flags().IntVar(&flagStListLimit, "limit", 100, "Max number of rows")
	listCmd.Flags().IntVar(&flagStListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().StringVar(&flagStListOutput, "output", "table", "Output format: table or json")
	listCmd.Flags().StringVar(&flagStListBlackboard, "blackboard", "", "Filter by blackboard UUID")
	// topics removed
}
