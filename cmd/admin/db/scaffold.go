package db

import (
    "context"
    "errors"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
    Use:   "scaffold",
    Short: "Use admin credentials to create roles, database, privileges, and schema",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil {
            return err
        }

        // Require admin credentials to be present
        if cfg.Postgres.Admin.User == "" || cfg.Postgres.Admin.Password == "" {
            return errors.New("postgres admin credentials missing; set postgres.admin.user and postgres.admin.password in config.yaml")
        }

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        // Expand --all into individual actions
        effCreateRoles := flagCreateRoles || flagAll
        effCreateDB := flagCreateDB || flagAll
        effGrantPrivs := flagGrantPrivs || flagAll

        // Safety confirmation if making structural changes
        if (effCreateRoles || effCreateDB || effGrantPrivs) && !flagYes {
            return errors.New("refusing to modify roles/databases without --yes; re-run with --yes to confirm")
        }

        // Optionally create roles and database using connection to 'postgres'
        if effCreateRoles || effCreateDB {
            fmt.Fprintf(os.Stderr, "db:scaffold - connecting to Postgres (postgres db) as admin %q...\n", cfg.Postgres.Admin.User)
            sysdb, err := pgdao.OpenAdminWithDB(ctx, cfg, "postgres")
            if err != nil {
                return err
            }
            defer sysdb.Close()

            if effCreateRoles {
                fmt.Fprintln(os.Stderr, "db:scaffold - ensuring roles (admin/app)...")
                if err := pgdao.EnsureRole(ctx, sysdb, cfg.Postgres.Admin.User, cfg.Postgres.Admin.Password); err != nil {
                    return err
                }
                // Prefer app password from config if provided
                if err := pgdao.EnsureRole(ctx, sysdb, cfg.Postgres.App.User, cfg.Postgres.App.Password); err != nil {
                    return err
                }
            }

            if effCreateDB {
                fmt.Fprintf(os.Stderr, "db:scaffold - ensuring database %q (owner %q)...\n", cfg.Postgres.DBName, cfg.Postgres.Admin.User)
                if err := pgdao.EnsureDatabase(ctx, sysdb, cfg.Postgres.DBName, cfg.Postgres.Admin.User); err != nil {
                    return err
                }
                if effGrantPrivs {
                    // Grant CONNECT on database to app role
                    if err := pgdao.GrantConnect(ctx, sysdb, cfg.Postgres.DBName, cfg.Postgres.App.User); err != nil {
                        return err
                    }
                }
            }
        }

        // Connect to target DB as admin
        fmt.Fprintf(os.Stderr, "db:scaffold - connecting to target DB %q as admin...\n", cfg.Postgres.DBName)
        db, err := pgdao.OpenAdmin(ctx, cfg)
        if err != nil {
            return err
        }
        defer db.Close()

        // Grant runtime privileges inside target DB
        if effGrantPrivs {
            fmt.Fprintln(os.Stderr, "db:scaffold - granting runtime privileges to app role...")
            if err := pgdao.GrantRuntimePrivileges(ctx, db, cfg.Postgres.App.User); err != nil {
                return err
            }
        }

        fmt.Fprintln(os.Stderr, "db:scaffold - ensuring schema (tables, triggers)...")
        if err := pgdao.EnsureSchema(ctx, db); err != nil {
            return err
        }

        // If --all is set, also ensure content table and FTS (same as db init)
        if flagAll {
            fmt.Fprintln(os.Stderr, "db:scaffold - ensuring PostgreSQL content table and FTS...")
            if err := pgdao.EnsureContentSchema(ctx, db); err != nil {
                return err
            }
            if err := pgdao.EnsureFTSIndex(ctx, db); err != nil {
                fmt.Fprintf(os.Stderr, "db:scaffold - warn: ensure FTS index: %v\n", err)
            }
        }

        fmt.Fprintln(os.Stderr, "db:scaffold - done")
        return nil
    },
}

func init() {
    DBCmd.AddCommand(scaffoldCmd)
}

var (
    flagCreateRoles bool
    flagCreateDB    bool
    flagGrantPrivs  bool
    flagYes         bool
    flagAll         bool
)

func init() {
    scaffoldCmd.Flags().BoolVar(&flagCreateRoles, "create-roles", false, "Create admin/app roles if missing (requires --yes)")
    scaffoldCmd.Flags().BoolVar(&flagCreateDB, "create-db", false, "Create the target database if missing (requires --yes)")
    scaffoldCmd.Flags().BoolVar(&flagGrantPrivs, "grant-privileges", false, "Grant runtime privileges to app role (requires --yes)")
    scaffoldCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm making structural changes (non-interactive)")
    scaffoldCmd.Flags().BoolVar(&flagAll, "all", false, "Do all: create roles, database, grant privileges, ensure schema + content/FTS")
}
