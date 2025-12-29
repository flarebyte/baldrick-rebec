package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagListSchema string
	flagListLimit  int
	flagListSince  string
	flagListUntil  string
	flagListJSON   bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing backups",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Prefer backup role for listing
		db, err := pgdao.OpenBackup(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		var sincePtr, untilPtr *time.Time
		if flagListSince != "" {
			t, err := time.Parse(time.RFC3339, flagListSince)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			sincePtr = &t
		}
		if flagListUntil != "" {
			t, err := time.Parse(time.RFC3339, flagListUntil)
			if err != nil {
				return fmt.Errorf("--until: %w", err)
			}
			untilPtr = &t
		}
		items, err := pgdao.ListBackups(ctx, db, flagListSchema, sincePtr, untilPtr, flagListLimit)
		if err != nil {
			return err
		}
		if flagListJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(items)
		}
		tw := tablewriter.NewWriter(os.Stdout)
		tw.SetHeader([]string{"ID", "CREATED_AT", "DESCRIPTION", "TAGS"})
		for _, it := range items {
			tags := 0
			if it.Tags != nil {
				tags = len(it.Tags)
			}
			tw.Append([]string{it.ID, it.CreatedAt.Format(time.RFC3339), ptrS(it.Description), fmt.Sprintf("%d", tags)})
		}
		tw.Render()
		return nil
	},
}

func ptrS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func init() {
	SnapshotCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&flagListSchema, "schema", "backup", "Backup schema name")
	listCmd.Flags().IntVar(&flagListLimit, "limit", 50, "Max rows")
	listCmd.Flags().StringVar(&flagListSince, "since", "", "RFC3339 timestamp")
	listCmd.Flags().StringVar(&flagListUntil, "until", "", "RFC3339 timestamp")
	listCmd.Flags().BoolVar(&flagListJSON, "json", false, "Output JSON")
}
