package db

import (
    "context"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
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

        // Postgres: prefer admin for schema changes, fallback to app
        fmt.Fprintln(os.Stderr, "db:init - connecting to Postgres (admin/app)...")
        db, err := pgdao.OpenAdmin(ctx, cfg)
        if err != nil {
            // fallback to app
            db, err = pgdao.OpenApp(ctx, cfg)
            if err != nil {
                return err
            }
        }
        defer db.Close()
        fmt.Fprintln(os.Stderr, "db:init - ensuring Postgres schema...")
        if err := pgdao.EnsureSchema(ctx, db); err != nil {
            return err
        }

        // Ensure content table and FTS readiness
        fmt.Fprintln(os.Stderr, "db:init - ensuring PostgreSQL content table...")
        if err := pgdao.EnsureContentSchema(ctx, db); err != nil {
            return err
        }
        if err := pgdao.EnsureFTSIndex(ctx, db); err != nil {
            fmt.Fprintf(os.Stderr, "db:init - warn: ensure FTS index: %v\n", err)
        } else {
            fmt.Fprintln(os.Stderr, "db:init - FTS index: ok")
        }

        fmt.Fprintln(os.Stderr, "db:init - done")
        return nil
    },
}
