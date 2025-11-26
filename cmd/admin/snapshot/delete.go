package snapshot

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagDeleteSchema string
    flagDeleteForce  bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete <backup-id>",
    Short: "Delete a backup",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id := args[0]
        if !flagDeleteForce {
            fmt.Fprintf(os.Stderr, "About to delete backup %s. Type 'yes' to confirm: ", id)
            in := bufio.NewReader(os.Stdin)
            s, _ := in.ReadString('\n')
            if strings.TrimSpace(strings.ToLower(s)) != "yes" {
                fmt.Fprintln(os.Stderr, "aborted")
                return nil
            }
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
        // Prefer backup role for deletion (assumes ownership/grants); fallback inside OpenBackup
        db, err := pgdao.OpenBackup(ctx, cfg); if err != nil { return err }
        defer db.Close()
        n, err := pgdao.DeleteBackup(ctx, db, flagDeleteSchema, id)
        if err != nil { return err }
        if n == 0 { fmt.Fprintln(os.Stderr, "not found or already deleted") } else { fmt.Fprintln(os.Stderr, "deleted") }
        return nil
    },
}

func init() {
    SnapshotCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagDeleteSchema, "schema", "backup", "Backup schema name")
    deleteCmd.Flags().BoolVar(&flagDeleteForce, "force", false, "Do not prompt for confirmation")
}
