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

        // If PG-only feature enabled, ensure content table in PG and skip OpenSearch
        if cfg.Features.PGOnly {
            fmt.Fprintln(os.Stderr, "db:init - pg_only=true; ensuring PostgreSQL content table...")
            if err := pgdao.EnsureContentSchema(ctx, db); err != nil {
                return err
            }
            if err := pgdao.EnsureFTSIndex(ctx, db); err != nil {
                fmt.Fprintf(os.Stderr, "db:init - warn: ensure FTS index: %v\n", err)
            } else {
                fmt.Fprintln(os.Stderr, "db:init - FTS index: ok")
            }
            if err := pgdao.EnsureVectorExtension(ctx, db); err != nil {
                fmt.Fprintf(os.Stderr, "db:init - note: pgvector extension not enabled (%v)\n", err)
            } else {
                fmt.Fprintln(os.Stderr, "db:init - pgvector extension: present")
            }
            // Optional embedding column + index based on feature dim
            if cfg.Features.PGVectorDim > 0 {
                if ok, _ := pgdao.HasVectorExtension(ctx, db); ok {
                    if err := pgdao.EnsureEmbeddingColumn(ctx, db, cfg.Features.PGVectorDim); err != nil {
                        fmt.Fprintf(os.Stderr, "db:init - warn: ensure embedding column: %v\n", err)
                    } else {
                        fmt.Fprintln(os.Stderr, "db:init - embedding column: ok")
                    }
                    if err := pgdao.EnsureEmbeddingIndex(ctx, db); err != nil {
                        fmt.Fprintf(os.Stderr, "db:init - warn: ensure embedding index: %v\n", err)
                    } else {
                        fmt.Fprintln(os.Stderr, "db:init - embedding index: ok")
                    }
                } else {
                    fmt.Fprintln(os.Stderr, "db:init - note: pgvector not present; skipping embedding column/index")
                }
            }
        } else {
            // OpenSearch: ensure index (use admin if available)
            fmt.Fprintln(os.Stderr, "db:init - connecting to OpenSearch (admin/app)...")
            osc := osdao.NewClientFromConfigAdmin(cfg)
            fmt.Fprintln(os.Stderr, "db:init - ensuring OpenSearch index 'messages_content'...")
            if err := osc.EnsureMessagesContentIndex(ctx); err != nil {
                return err
            }
        }

        fmt.Fprintln(os.Stderr, "db:init - done")
        return nil
    },
}
