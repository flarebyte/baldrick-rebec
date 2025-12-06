package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagPruneSchema string
	flagPruneOlder  string
	flagPruneYes    bool
	flagPruneJSON   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete old backups older than a given age (respects retention_until)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagPruneOlder == "" {
			flagPruneOlder = "90d"
		}
		d, err := parseHumanDuration(flagPruneOlder)
		if err != nil {
			return fmt.Errorf("--older-than: %w", err)
		}
		cutoff := time.Now().Add(-d)

		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		db, err := pgdao.OpenBackup(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		n, err := pgdao.CountBackupsOlderThan(ctx, db, flagPruneSchema, cutoff)
		if err != nil {
			return err
		}
		if !flagPruneYes {
			if flagPruneJSON {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"candidates": n, "cutoff": cutoff})
			}
			fmt.Fprintf(os.Stderr, "prune: candidates=%d cutoff=%s (use --yes to delete)\n", n, cutoff.Format(time.RFC3339))
			return nil
		}
		del, err := pgdao.DeleteBackupsOlderThan(ctx, db, flagPruneSchema, cutoff)
		if err != nil {
			return err
		}
		if flagPruneJSON {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"deleted": del})
		}
		fmt.Fprintf(os.Stderr, "prune: deleted=%d\n", del)
		return nil
	},
}

func init() {
	SnapshotCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().StringVar(&flagPruneSchema, "schema", "backup", "Backup schema name")
	pruneCmd.Flags().StringVar(&flagPruneOlder, "older-than", "90d", "Age threshold, e.g. 90d, 6mo, 1y")
	pruneCmd.Flags().BoolVar(&flagPruneYes, "yes", false, "Confirm deletion of old backups")
	pruneCmd.Flags().BoolVar(&flagPruneJSON, "json", false, "Output JSON")
}
