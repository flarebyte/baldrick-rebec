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

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show database connectivity and index/schema status",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil {
            return err
        }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        // Postgres (app role)
        fmt.Fprintln(os.Stderr, "db:status - checking Postgres (app)...")
        pgres, err := pgdao.OpenApp(ctx, cfg)
        if err != nil {
            fmt.Fprintf(os.Stderr, "postgres: error: %v\n", err)
        } else {
            defer pgres.Close()
            var dbname, user, version string
            _ = pgres.QueryRowContext(ctx, "select current_database(), current_user, version()").Scan(&dbname, &user, &version)
            fmt.Fprintf(os.Stderr, "postgres: ok db=%s user=%s\n", dbname, user)
        }

        // OpenSearch (app role)
        fmt.Fprintln(os.Stderr, "db:status - checking OpenSearch (app)...")
        osc := osdao.NewClientFromConfigApp(cfg)
        health, err := osc.ClusterHealth(ctx)
        if err != nil {
            fmt.Fprintf(os.Stderr, "opensearch: error: %v\n", err)
        } else {
            fmt.Fprintf(os.Stderr, "opensearch: cluster health=%s\n", health)
        }
        if exists, err := osc.IndexExists(ctx, "messages_content"); err == nil {
            if exists {
                fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content' present")
            } else {
                fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content' missing")
            }
        } else {
            fmt.Fprintf(os.Stderr, "opensearch: index check error: %v\n", err)
        }

        return nil
    },
}

func init() {
    DBCmd.AddCommand(statusCmd)
}
