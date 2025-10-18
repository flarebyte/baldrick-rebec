package db

import (
    "context"
    "encoding/json"
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
        type pgRole struct{ Exists bool }
        type pgAppConn struct {
            OK   bool   `json:"ok"`
            DB   string `json:"db,omitempty"`
            User string `json:"user,omitempty"`
            Error string `json:"error,omitempty"`
        }
        type pgStatus struct {
            AppConnection pgAppConn `json:"app_connection"`
            Roles struct{
                Admin pgRole `json:"admin"`
                App   pgRole `json:"app"`
            } `json:"roles"`
            Database struct{ Exists bool `json:"exists"` } `json:"database"`
            Schema struct{ TablesOK bool `json:"tables_ok"` } `json:"schema"`
            Privileges struct{
                Usage bool `json:"usage"`
                MissingDML bool `json:"missing_dml"`
                OK bool `json:"ok"`
            } `json:"privileges"`
        }
        type osIndex struct{
            Exists bool `json:"exists"`
            ILMPolicy string `json:"ilm_policy,omitempty"`
            ILMPolicyExists *bool `json:"ilm_policy_exists,omitempty"`
            DocCount int64 `json:"doc_count,omitempty"`
        }
        type osStatus struct{
            Health string `json:"health,omitempty"`
            Index osIndex `json:"messages_content"`
            Error string `json:"error,omitempty"`
        }
        type statusOut struct{
            Postgres pgStatus `json:"postgres"`
            OpenSearch osStatus `json:"opensearch"`
        }
        st := statusOut{}
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
            st.Postgres.AppConnection = pgAppConn{OK:false, Error: err.Error()}
        } else {
            defer pgres.Close()
            var dbname, user, version string
            _ = pgres.QueryRowContext(ctx, "select current_database(), current_user, version()").Scan(&dbname, &user, &version)
            fmt.Fprintf(os.Stderr, "postgres: ok db=%s user=%s\n", dbname, user)
            st.Postgres.AppConnection = pgAppConn{OK:true, DB:dbname, User:user}
        }
        // Admin/system DB for role/db checks
        var sysdbOK bool
        if sysdb, err := pgdao.OpenAdminWithDB(ctx, cfg, "postgres"); err == nil {
            defer sysdb.Close()
            sysdbOK = true
            // Roles
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.Admin.User); ok {
                fmt.Fprintf(os.Stderr, "postgres: role %q: ok\n", cfg.Postgres.Admin.User)
                st.Postgres.Roles.Admin.Exists = true
            } else {
                fmt.Fprintf(os.Stderr, "postgres: role %q: missing (scaffold --create-roles)\n", cfg.Postgres.Admin.User)
            }
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.App.User); ok {
                fmt.Fprintf(os.Stderr, "postgres: role %q: ok\n", cfg.Postgres.App.User)
                st.Postgres.Roles.App.Exists = true
            } else {
                fmt.Fprintf(os.Stderr, "postgres: role %q: missing (scaffold --create-roles)\n", cfg.Postgres.App.User)
            }
            // Database
            if ok, _ := pgdao.DatabaseExists(ctx, sysdb, cfg.Postgres.DBName); ok {
                fmt.Fprintf(os.Stderr, "postgres: database %q: ok\n", cfg.Postgres.DBName)
                st.Postgres.Database.Exists = true
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
                st.Postgres.Schema.TablesOK = true
            } else {
                fmt.Fprintln(os.Stderr, "postgres: schema tables: missing/incomplete (run db scaffold or db init)")
            }
            if sysdbOK {
                usage, _ := pgdao.HasSchemaUsage(ctx, db, cfg.Postgres.App.User, "public")
                missingDML, _ := pgdao.MissingTableDML(ctx, db, cfg.Postgres.App.User, "public")
                st.Postgres.Privileges.Usage = usage
                st.Postgres.Privileges.MissingDML = missingDML
                st.Postgres.Privileges.OK = usage && !missingDML
                if usage && !missingDML {
                    fmt.Fprintf(os.Stderr, "postgres: privileges for %q: ok\n", cfg.Postgres.App.User)
                } else {
                    fmt.Fprintf(os.Stderr, "postgres: privileges for %q: missing (scaffold --grant-privileges)\n", cfg.Postgres.App.User)
                }
            }
            if cfg.Features.PGOnly {
                // Content table
                var exists bool
                _ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='messages_content_pg')`).Scan(&exists)
                if exists {
                    fmt.Fprintln(os.Stderr, "postgres: content table: ok")
                } else {
                    fmt.Fprintln(os.Stderr, "postgres: content table: missing (run 'rbc admin db init')")
                }
                // FTS index readiness: rely on index name we create
                var fts bool
                _ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_messages_content_pg_fts')`).Scan(&fts)
                if fts {
                    fmt.Fprintln(os.Stderr, "postgres: FTS index: ok")
                } else {
                    fmt.Fprintln(os.Stderr, "postgres: FTS index: missing (run 'rbc admin db init')")
                }
                // Vector extension presence
                if ok, _ := pgdao.HasVectorExtension(ctx, db); ok {
                    fmt.Fprintln(os.Stderr, "postgres: pgvector extension: present")
                } else {
                    fmt.Fprintln(os.Stderr, "postgres: pgvector extension: not installed (optional)")
                }
                // Embedding column/index check when configured
                if cfg.Features.PGVectorDim > 0 {
                    var hasCol bool
                    _ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='messages_content_pg' AND column_name='embedding')`).Scan(&hasCol)
                    if hasCol {
                        fmt.Fprintln(os.Stderr, "postgres: embedding column: ok")
                    } else {
                        fmt.Fprintln(os.Stderr, "postgres: embedding column: missing (run 'rbc admin db init')")
                    }
                    var hasEmbIdx bool
                    _ = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_messages_content_pg_embedding')`).Scan(&hasEmbIdx)
                    if hasEmbIdx {
                        fmt.Fprintln(os.Stderr, "postgres: embedding index: ok")
                    } else {
                        fmt.Fprintln(os.Stderr, "postgres: embedding index: missing (run 'rbc admin db init')")
                    }
                }
            }
        }

        if cfg.Features.PGOnly {
            fmt.Fprintln(os.Stderr, "db:status - pg_only=true; skipping OpenSearch checks")
            st.OpenSearch.Error = "pg_only=true"
        } else {
            // OpenSearch (app role)
            fmt.Fprintln(os.Stderr, "db:status - checking OpenSearch (app)...")
            osc := osdao.NewClientFromConfigApp(cfg)
            health, err := osc.ClusterHealth(ctx)
            if err != nil {
                fmt.Fprintf(os.Stderr, "opensearch: error: %v\n", err)
                st.OpenSearch.Error = err.Error()
            } else {
                fmt.Fprintf(os.Stderr, "opensearch: cluster health=%s\n", health)
                st.OpenSearch.Health = health
            }
            if exists, err := osc.IndexExists(ctx, "messages_content"); err == nil {
                if exists {
                    fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content': present")
                    st.OpenSearch.Index.Exists = true
                    if name, err := osc.IndexLifecycleName(ctx, "messages_content"); err == nil && name != "" {
                        fmt.Fprintf(os.Stderr, "opensearch: index ILM policy=%q\n", name)
                        st.OpenSearch.Index.ILMPolicy = name
                        // Check ILM policy existence via admin
                        adminOSC := osdao.NewClientFromConfigAdmin(cfg)
                        if _, err := adminOSC.GetILMPolicy(ctx, name); err == nil {
                            fmt.Fprintln(os.Stderr, "opensearch: ILM policy exists: ok")
                            v := true
                            st.OpenSearch.Index.ILMPolicyExists = &v
                        } else {
                            fmt.Fprintf(os.Stderr, "opensearch: ILM policy missing or inaccessible: %v\n", err)
                            v := false
                            st.OpenSearch.Index.ILMPolicyExists = &v
                        }
                    } else if pid, err := osc.IndexISMPolicyID(ctx, "messages_content"); err == nil && pid != "" {
                        fmt.Fprintf(os.Stderr, "opensearch: index ISM policy=%q\n", pid)
                        st.OpenSearch.Index.ILMPolicy = pid
                        adminOSC := osdao.NewClientFromConfigAdmin(cfg)
                        if _, err := adminOSC.GetISMPolicy(ctx, pid); err == nil {
                            fmt.Fprintln(os.Stderr, "opensearch: ISM policy exists: ok")
                            v := true
                            st.OpenSearch.Index.ILMPolicyExists = &v
                        } else {
                            fmt.Fprintf(os.Stderr, "opensearch: ISM policy missing or inaccessible: %v\n", err)
                            v := false
                            st.OpenSearch.Index.ILMPolicyExists = &v
                        }
                    } else {
                        fmt.Fprintln(os.Stderr, "opensearch: index lifecycle policy: not set (use 'rbc admin os ilm ensure --attach-to-index messages_content' or ISM policy)")
                    }
                    if cnt, err := osc.IndexDocCount(ctx, "messages_content"); err == nil {
                        fmt.Fprintf(os.Stderr, "opensearch: index doc count=%d\n", cnt)
                        st.OpenSearch.Index.DocCount = cnt
                    }
                } else {
                    fmt.Fprintln(os.Stderr, "opensearch: index 'messages_content': missing (run 'rbc admin db init')")
                }
            } else {
                fmt.Fprintf(os.Stderr, "opensearch: index check error: %v\n", err)
            }
        }

        if flagStatusJSON {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(st)
        }
        return nil
    },
}

func init() {
    DBCmd.AddCommand(statusCmd)
}

var flagStatusJSON bool

func init() {
    statusCmd.Flags().BoolVar(&flagStatusJSON, "json", false, "Output status as JSON")
}
