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

        // Try admin (for richer checks) and app (for runtime)
        fmt.Fprintln(os.Stderr, "db:status - checking Postgres (app/admin)...")
        pgres, err := pgdao.OpenApp(ctx, cfg)
        if err != nil {
            fmt.Fprintf(os.Stderr, "postgres: error: %v\n", err)
        } else {
            defer pgres.Close()
            var dbname, user, version string
            _ = pgres.QueryRowContext(ctx, "select current_database(), current_user, version()").Scan(&dbname, &user, &version)
            fmt.Fprintf(os.Stderr, "postgres: ok db=%s user=%s\n", dbname, user)
        }
        // Admin/system DB for role/db checks
        var sysdbOK bool
        if sysdb, err := pgdao.OpenAdminWithDB(ctx, cfg, "postgres"); err == nil {
            defer sysdb.Close()
            sysdbOK = true
            // Roles
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.Admin.User); ok {
                fmt.Fprintf(os.Stderr, "postgres: role %q: ok\n", cfg.Postgres.Admin.User)
            } else {
                fmt.Fprintf(os.Stderr, "postgres: role %q: missing (scaffold --create-roles)\n", cfg.Postgres.Admin.User)
            }
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.App.User); ok {
                fmt.Fprintf(os.Stderr, "postgres: role %q: ok\n", cfg.Postgres.App.User)
            } else {
                fmt.Fprintf(os.Stderr, "postgres: role %q: missing (scaffold --create-roles)\n", cfg.Postgres.App.User)
            }
            // Database
            if ok, _ := pgdao.DatabaseExists(ctx, sysdb, cfg.Postgres.DBName); ok {
                fmt.Fprintf(os.Stderr, "postgres: database %q: ok\n", cfg.Postgres.DBName)
            } else {
                fmt.Fprintf(os.Stderr, "postgres: database %q: missing (scaffold --create-db)\n", cfg.Postgres.DBName)
            }
            // Grant connect
            // We cannot directly test CONNECT privilege easily cross-db here; implied by ability to connect as app.
        }
        // In target DB: tables and privileges
        if db, err := pgdao.OpenAdmin(ctx, cfg); err == nil {
            defer db.Close()
            var cnt int
            _ = db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name in ('messages_events','message_profiles')").Scan(&cnt)
            if cnt == 2 {
                fmt.Fprintln(os.Stderr, "postgres: schema tables: ok")
            } else {
                fmt.Fprintln(os.Stderr, "postgres: schema tables: missing/incomplete (run db scaffold or db init)")
            }
            if sysdbOK {
                usage, _ := pgdao.HasSchemaUsage(ctx, db, cfg.Postgres.App.User, "public")
                missingDML, _ := pgdao.MissingTableDML(ctx, db, cfg.Postgres.App.User, "public")
                if usage && !missingDML {
                    fmt.Fprintf(os.Stderr, "postgres: privileges for %q: ok\n", cfg.Postgres.App.User)
                } else {
                    fmt.Fprintf(os.Stderr, "postgres: privileges for %q: missing (scaffold --grant-privileges)\n", cfg.Postgres.App.User)
                }
            }
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
                fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content': present")
                if name, err := osc.IndexLifecycleName(ctx, "messages_content"); err == nil && name != "" {
                    fmt.Fprintf(os.Stderr, "opensearch: index ILM policy=%q\n", name)
                    // Check ILM policy existence via admin
                    adminOSC := osdao.NewClientFromConfigAdmin(cfg)
                    if _, err := adminOSC.GetILMPolicy(ctx, name); err == nil {
                        fmt.Fprintln(os.Stderr, "opensearch: ILM policy exists: ok")
                    } else {
                        fmt.Fprintf(os.Stderr, "opensearch: ILM policy missing or inaccessible: %v\n", err)
                    }
                } else {
                    fmt.Fprintln(os.Stderr, "opensearch: index ILM policy: not set (use 'rbc admin os ilm ensure --attach-to-index messages_content')")
                }
                if cnt, err := osc.IndexDocCount(ctx, "messages_content"); err == nil {
                    fmt.Fprintf(os.Stderr, "opensearch: index doc count=%d\n", cnt)
                }
            } else {
                fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content': missing (run 'rbc admin db init')")
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
