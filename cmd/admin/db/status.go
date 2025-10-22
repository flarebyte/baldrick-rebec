package db

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
        type statusOut struct{
            Postgres pgStatus `json:"postgres"`
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
            _ = pgres.QueryRow(ctx, "select current_database(), current_user, version()").Scan(&dbname, &user, &version)
            fmt.Fprintf(os.Stderr, "postgres: ok db=%s user=%s\n", dbname, user)
            st.Postgres.AppConnection = pgAppConn{OK:true, DB:dbname, User:user}
        }

        // From app connection, sanity-check configured admin role existence/superuser
        var adminExists, adminSuper, adminCanLogin bool
        var superCandidates []string
        if st.Postgres.AppConnection.OK {
            // Check configured admin role info
            _ = pgres.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, cfg.Postgres.Admin.User).Scan(&adminExists)
            if adminExists {
                _ = pgres.QueryRow(ctx, `SELECT rolsuper, rolcanlogin FROM pg_roles WHERE rolname=$1`, cfg.Postgres.Admin.User).Scan(&adminSuper, &adminCanLogin)
                fmt.Fprintf(os.Stderr, "postgres: admin role %q: exists=%v superuser=%v can_login=%v (via app view)\n", cfg.Postgres.Admin.User, adminExists, adminSuper, adminCanLogin)
            } else {
                fmt.Fprintf(os.Stderr, "postgres: admin role %q: not found (via app view)\n", cfg.Postgres.Admin.User)
            }
            // Gather candidate superusers for hints
            rows, err := pgres.Query(ctx, `SELECT rolname FROM pg_roles WHERE rolsuper AND rolcanlogin ORDER BY rolname LIMIT 5`)
            if err == nil {
                for rows.Next() {
                    var rn string
                    _ = rows.Scan(&rn)
                    superCandidates = append(superCandidates, rn)
                }
                rows.Close()
                if len(superCandidates) > 0 {
                    fmt.Fprintf(os.Stderr, "postgres: superuser candidates: %v\n", superCandidates)
                }
            }
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
        } else {
            fmt.Fprintf(os.Stderr, "postgres: admin connect to system DB failed: %v\n", err)
            if !adminExists && len(superCandidates) > 0 {
                fmt.Fprintf(os.Stderr, "hint: configured admin user %q not found; try one of: %v\n", cfg.Postgres.Admin.User, superCandidates)
            } else if adminExists && (!adminSuper || !adminCanLogin) {
                fmt.Fprintf(os.Stderr, "hint: admin role %q exists but superuser=%v can_login=%v; use a superuser with login\n", cfg.Postgres.Admin.User, adminSuper, adminCanLogin)
            }
        }
        // In target DB: tables and privileges
        if db, err := pgdao.OpenAdmin(ctx, cfg); err == nil {
            defer db.Close()
            var cnt int
            _ = db.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name in ('messages_events','message_profiles')").Scan(&cnt)
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
            // Content table
            var exists bool
            _ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='messages_content_pg')`).Scan(&exists)
            if exists {
                fmt.Fprintln(os.Stderr, "postgres: content table: ok")
            } else {
                fmt.Fprintln(os.Stderr, "postgres: content table: missing (run 'rbc admin db init')")
            }
            // FTS index readiness: rely on index name we create
            var fts bool
            _ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_messages_content_pg_fts')`).Scan(&fts)
            if fts {
                fmt.Fprintln(os.Stderr, "postgres: FTS index: ok")
            } else {
                fmt.Fprintln(os.Stderr, "postgres: FTS index: missing (run 'rbc admin db init')")
            }
        } else {
            fmt.Fprintf(os.Stderr, "postgres: admin connect to target DB failed: %v\n", err)
            if !adminExists && len(superCandidates) > 0 {
                fmt.Fprintf(os.Stderr, "hint: configured admin user %q not found; try one of: %v\n", cfg.Postgres.Admin.User, superCandidates)
            } else if adminExists && (!adminSuper || !adminCanLogin) {
                fmt.Fprintf(os.Stderr, "hint: admin role %q exists but superuser=%v can_login=%v; use a superuser with login\n", cfg.Postgres.Admin.User, adminSuper, adminCanLogin)
            }
        }

        // OpenSearch removed in PG-only path

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
