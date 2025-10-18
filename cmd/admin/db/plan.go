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

// planCmd prints what actions scaffold would take, without making changes.
var planCmd = &cobra.Command{
    Use:   "plan",
    Short: "Print planned role/DB/privilege/schema actions (no changes)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
        defer cancel()

        // Connect to system DB as admin for role/db checks
        sysdb, err := pgdao.OpenAdminWithDB(ctx, cfg, "postgres")
        if err != nil {
            fmt.Fprintln(os.Stderr, "warning: cannot connect as admin to system DB; role/DB checks may be incomplete:", err)
        }
        if sysdb != nil { defer sysdb.Close() }

        // Roles
        if sysdb != nil {
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.Admin.User); !ok {
                fmt.Printf("PLAN: CREATE ROLE %s LOGIN;\n", cfg.Postgres.Admin.User)
            }
            if ok, _ := pgdao.RoleExists(ctx, sysdb, cfg.Postgres.App.User); !ok {
                fmt.Printf("PLAN: CREATE ROLE %s LOGIN;\n", cfg.Postgres.App.User)
            }
        }

        // Database
        if sysdb != nil {
            if ok, _ := pgdao.DatabaseExists(ctx, sysdb, cfg.Postgres.DBName); !ok {
                fmt.Printf("PLAN: CREATE DATABASE %s OWNER %s;\n", cfg.Postgres.DBName, cfg.Postgres.Admin.User)
            }
        }

        // Connect to target DB for privileges and schema checks
        admdb, err := pgdao.OpenAdmin(ctx, cfg)
        if err != nil {
            fmt.Fprintln(os.Stderr, "warning: cannot connect to target DB as admin; schema/priv checks may be incomplete:", err)
        }
        if admdb != nil { defer admdb.Close() }

        if admdb != nil {
            // Privileges
            usage, _ := pgdao.HasSchemaUsage(ctx, admdb, cfg.Postgres.App.User, "public")
            if !usage {
                fmt.Printf("PLAN: GRANT USAGE ON SCHEMA public TO %s;\n", cfg.Postgres.App.User)
            }
            if missing, _ := pgdao.MissingTableDML(ctx, admdb, cfg.Postgres.App.User, "public"); missing {
                fmt.Printf("PLAN: GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;\n", cfg.Postgres.App.User)
                fmt.Printf("PLAN: ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s;\n", cfg.Postgres.App.User)
                fmt.Printf("PLAN: GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO %s;\n", cfg.Postgres.App.User)
                fmt.Printf("PLAN: ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO %s;\n", cfg.Postgres.App.User)
            }

            // Schema objects
            var cnt int
            _ = admdb.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name in ('messages_events','message_profiles')").Scan(&cnt)
            if cnt < 2 {
                fmt.Println("PLAN: CREATE TABLE messages_events (...);")
                fmt.Println("PLAN: CREATE TABLE message_profiles (...);")
                fmt.Println("PLAN: CREATE TRIGGER message_profiles_set_updated_at ...;")
            }
        }

        return nil
    },
}

func init() {
    DBCmd.AddCommand(planCmd)
}

