package tool

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
	flagToolListRole   string
	flagToolListLimit  int
	flagToolListOffset int
	flagToolListOutput string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tools for a role (paginated)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagToolListRole) == "" {
			return errors.New("--role is required")
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
		items, err := pgdao.ListTools(ctx, db, flagToolListRole, flagToolListLimit, flagToolListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "tools: %d\n", len(items))
		out := strings.ToLower(strings.TrimSpace(flagToolListOutput))
		if out == "json" {
			arr := make([]map[string]any, 0, len(items))
			for _, t := range items {
				m := map[string]any{"name": t.Name, "title": t.Title, "role": t.RoleName}
				if t.Updated.Valid {
					m["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
				}
				if t.ToolType.Valid {
					m["type"] = t.ToolType.String
				}
				arr = append(arr, m)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"NAME", "TITLE", "ROLE", "TYPE", "UPDATED"})
		for _, t := range items {
			updated := ""
			if t.Updated.Valid {
				updated = t.Updated.Time.Format(time.RFC3339)
			}
			typ := ""
			if t.ToolType.Valid {
				typ = t.ToolType.String
			}
			table.Append([]string{t.Name, t.Title, t.RoleName, typ, updated})
		}
		table.Render()
		return nil
	},
}

func init() {
	ToolCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&flagToolListRole, "role", "", "Role name (required)")
	listCmd.Flags().IntVar(&flagToolListLimit, "limit", 100, "Max rows")
	listCmd.Flags().IntVar(&flagToolListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().StringVar(&flagToolListOutput, "output", "table", "Output format: table or json")
}
