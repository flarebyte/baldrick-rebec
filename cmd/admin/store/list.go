package store

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
	flagStoreListLimit  int
	flagStoreListOffset int
	flagStoreListOutput string
	flagStoreListRole   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stores for a role (paginated)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagStoreListRole) == "" {
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
		ss, err := pgdao.ListStores(ctx, db, flagStoreListRole, flagStoreListLimit, flagStoreListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "stores: %d\n", len(ss))
		out := strings.ToLower(strings.TrimSpace(flagStoreListOutput))
		if out == "json" {
			arr := make([]map[string]any, 0, len(ss))
			for _, s := range ss {
				item := map[string]any{"id": s.ID, "name": s.Name, "role": s.RoleName, "title": s.Title}
				if s.StoreType.Valid && s.StoreType.String != "" {
					item["type"] = s.StoreType.String
				}
				if s.Scope.Valid && s.Scope.String != "" {
					item["scope"] = s.Scope.String
				}
				if s.Lifecycle.Valid && s.Lifecycle.String != "" {
					item["lifecycle"] = s.Lifecycle.String
				}
				if s.Updated.Valid {
					item["updated"] = s.Updated.Time.Format(time.RFC3339Nano)
				}
				arr = append(arr, item)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		// table default
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"NAME", "TITLE", "TYPE", "SCOPE", "LIFECYCLE", "UPDATED"})
		for _, s := range ss {
			updated := ""
			if s.Updated.Valid {
				updated = s.Updated.Time.Format(time.RFC3339)
			}
			st := ""
			if s.StoreType.Valid {
				st = s.StoreType.String
			}
			sc := ""
			if s.Scope.Valid {
				sc = s.Scope.String
			}
			lc := ""
			if s.Lifecycle.Valid {
				lc = s.Lifecycle.String
			}
			table.Append([]string{s.Name, s.Title, st, sc, lc, updated})
		}
		table.Render()
		return nil
	},
}

func init() {
	StoreCmd.AddCommand(listCmd)
	listCmd.Flags().IntVar(&flagStoreListLimit, "limit", 100, "Max number of rows")
	listCmd.Flags().IntVar(&flagStoreListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().StringVar(&flagStoreListOutput, "output", "table", "Output format: table or json")
	listCmd.Flags().StringVar(&flagStoreListRole, "role", "", "Role name (required)")
}
