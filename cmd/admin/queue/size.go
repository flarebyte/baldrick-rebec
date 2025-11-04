package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagQSizeStatus string
)

var sizeCmd = &cobra.Command{
    Use:   "size",
    Short: "Return queue size (optionally by status)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        n, err := pgdao.CountQueues(ctx, db, strings.TrimSpace(flagQSizeStatus))
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "queue size=%d\n", n)
        // JSON
        out := map[string]any{"count": n}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    QueueCmd.AddCommand(sizeCmd)
    sizeCmd.Flags().StringVar(&flagQSizeStatus, "status", "", "Filter by status")
}

