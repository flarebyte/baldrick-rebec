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
	flagShowSchema string
	flagShowJSON   bool
)

var showCmd = &cobra.Command{
	Use:   "show <backup-id>",
	Short: "Show a backup summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Prefer backup role to access backup metadata
		db, err := pgdao.OpenBackup(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// Load metadata
		rows, err := db.Query(ctx, `SELECT id, created_at, description, tags, initiated_by, retention_until FROM `+flagShowSchema+`.backups WHERE id=$1`, id)
		if err != nil {
			return err
		}
		var meta any
		var found bool
		var idOut string
		var created time.Time
		var desc *string
		var tags map[string]any
		var who *string
		var retain *time.Time
		for rows.Next() {
			if err := rows.Scan(&idOut, &created, &desc, &tags, &who, &retain); err != nil {
				rows.Close()
				return err
			}
			found = true
		}
		rows.Close()
		if !found {
			return fmt.Errorf("backup not found: %s", id)
		}
		counts, err := pgdao.CountPerEntity(ctx, db, flagShowSchema, id)
		if err != nil {
			return err
		}
		meta = map[string]any{
			"id":              idOut,
			"created_at":      created,
			"description":     desc,
			"tags":            tags,
			"initiated_by":    who,
			"retention_until": retain,
			"entities":        counts,
		}
		if flagShowJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(meta)
		}
		fmt.Fprintf(os.Stderr, "id: %s\ncreated_at: %s\n", idOut, created.Format(time.RFC3339))
		if desc != nil {
			fmt.Fprintf(os.Stderr, "description: %s\n", *desc)
		}
		if who != nil {
			fmt.Fprintf(os.Stderr, "initiated_by: %s\n", *who)
		}
		if retain != nil {
			fmt.Fprintf(os.Stderr, "retention_until: %s\n", retain.Format(time.RFC3339))
		}
		tw := tablewriter.NewWriter(os.Stdout)
		tw.SetHeader([]string{"ENTITY", "COUNT"})
		for k, v := range counts {
			tw.Append([]string{k, fmt.Sprintf("%d", v)})
		}
		tw.Render()
		return nil
	},
}

func init() {
	SnapshotCmd.AddCommand(showCmd)
	showCmd.Flags().StringVar(&flagShowSchema, "schema", "backup", "Backup schema name")
	showCmd.Flags().BoolVar(&flagShowJSON, "json", false, "Output JSON")
}
