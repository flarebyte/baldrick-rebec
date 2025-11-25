package snapshot

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    bkp "github.com/flarebyte/baldrick-rebec/internal/backup"
    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagRestoreSchema string
    flagRestoreMode   string
    flagRestoreEntity string
    flagRestoreDry    bool
    flagRestoreJSON   bool
)

var restoreCmd = &cobra.Command{
    Use:   "restore <backup-id>",
    Short: "Restore a backup into live tables",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id := args[0]
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute); defer cancel()
        // Prefer backup role; replace mode may require elevated privileges
        db, err := pgdao.OpenBackup(ctx, cfg); if err != nil { return err }
        defer db.Close()
        // validate mode
        mode := strings.ToLower(strings.TrimSpace(flagRestoreMode))
        if mode == "" { mode = "append" }
        if mode != "replace" && mode != "append" { return fmt.Errorf("invalid --mode, want replace|append") }
        ents := bkp.DefaultEntities()
        list := []string{}
        if strings.TrimSpace(flagRestoreEntity) != "" { list = strings.Split(flagRestoreEntity, ",") }
        opt := bkp.RestoreOptions{Schema: flagRestoreSchema, Entities: list, Mode: mode, DryRun: flagRestoreDry}
        if err := bkp.RestoreFromBackup(ctx, db, ents, id, opt); err != nil { return err }
        if flagRestoreJSON {
            return json.NewEncoder(os.Stdout).Encode(map[string]any{"restored": true, "mode": mode})
        }
        fmt.Fprintln(os.Stderr, "restore completed")
        return nil
    },
}

func init() {
    SnapshotCmd.AddCommand(restoreCmd)
    restoreCmd.Flags().StringVar(&flagRestoreSchema, "schema", "backup", "Backup schema name")
    restoreCmd.Flags().StringVar(&flagRestoreMode, "mode", "append", "Restore mode: replace|append")
    restoreCmd.Flags().StringVar(&flagRestoreEntity, "entity", "", "Comma-separated list of entities to restore (default all)")
    restoreCmd.Flags().BoolVar(&flagRestoreDry, "dry-run", false, "Do not apply changes; validate and print summary")
    restoreCmd.Flags().BoolVar(&flagRestoreJSON, "json", false, "Output JSON status")
}
