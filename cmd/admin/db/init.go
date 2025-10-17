package db

import (
    "context"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    osdao "github.com/flarebyte/baldrick-rebec/internal/dao/opensearch"
    "github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize or migrate relational and search/vector databases",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Load config
        cfg, err := cfgpkg.Load()
        if err != nil {
            return err
        }

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        // Postgres: open and ensure schema
        fmt.Fprintln(os.Stderr, "db:init - connecting to Postgres...")
        db, err := pgdao.Open(ctx, cfg)
        if err != nil {
            return err
        }
        defer db.Close()
        fmt.Fprintln(os.Stderr, "db:init - ensuring Postgres schema...")
        if err := pgdao.EnsureSchema(ctx, db); err != nil {
            return err
        }

        // OpenSearch: ensure index
        fmt.Fprintln(os.Stderr, "db:init - connecting to OpenSearch...")
        osc := osdao.NewClientFromConfig(cfg)
        fmt.Fprintln(os.Stderr, "db:init - ensuring OpenSearch index 'messages_content'...")
        if err := osc.EnsureMessagesContentIndex(ctx); err != nil {
            return err
        }

        fmt.Fprintln(os.Stderr, "db:init - done")
        return nil
    },
}

