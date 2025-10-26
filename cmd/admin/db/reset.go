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

var (
    flagResetForce       bool
    flagResetDropDB      bool
    flagResetDropAppRole bool
    flagResetDropAdmin   bool
)

var resetCmd = &cobra.Command{
    Use:   "reset",
    Short: "DANGEROUS: Drop DB objects, roles, and optionally database (developer reset)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if !flagResetForce {
            return errors.New("refusing to perform destructive reset without --force")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        if cfg.Postgres.Admin.User == "" || cfg.Postgres.Admin.Password == "" {
            return errors.New("admin credentials required: set postgres.admin.user and postgres.admin.password")
        }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        fmt.Fprintf(os.Stderr, "db:reset - starting destructive reset (db=%q, app role=%q)\n", cfg.Postgres.DBName, cfg.Postgres.App.User)

        // Connect to system DB as admin for role/db operations
        sysdb, err := pgdao.OpenAdminWithDB(ctx, cfg, "postgres")
        if err != nil { return fmt.Errorf("connect system DB as admin: %w", err) }
        defer sysdb.Close()

        // If not dropping the database, reset objects by connecting to target DB
        if !flagResetDropDB {
            fmt.Fprintln(os.Stderr, "db:reset - resetting public schema in target DB (no drop-db)")
            admdb, err := pgdao.OpenAdmin(ctx, cfg)
            if err != nil { return err }
            if err := pgdao.ResetPublicSchema(ctx, admdb); err != nil { admdb.Close(); return err }
            admdb.Close()
        }

        // Drop database if requested
        if flagResetDropDB {
            fmt.Fprintln(os.Stderr, "db:reset - terminating connections to target DB...")
            _ = pgdao.TerminateConnections(ctx, sysdb, cfg.Postgres.DBName)
            fmt.Fprintln(os.Stderr, "db:reset - dropping database...")
            if err := pgdao.DropDatabase(ctx, sysdb, cfg.Postgres.DBName); err != nil { return err }
        }

        // Drop app role if requested
        if flagResetDropAppRole && cfg.Postgres.App.User != "" {
            fmt.Fprintf(os.Stderr, "db:reset - dropping app role %q...\n", cfg.Postgres.App.User)
            if err := pgdao.DropRole(ctx, sysdb, cfg.Postgres.App.User); err != nil { return err }
        }

        // Optionally drop admin role (only if not current user)
        if flagResetDropAdmin && cfg.Postgres.Admin.User != "" {
            fmt.Fprintf(os.Stderr, "db:reset - dropping admin role %q...\n", cfg.Postgres.Admin.User)
            if err := pgdao.DropRole(ctx, sysdb, cfg.Postgres.Admin.User); err != nil { return err }
        }

        fmt.Fprintln(os.Stderr, "db:reset - done. You can now run 'rbc admin db scaffold --create-roles --create-db --grant-privileges --yes' to reinitialize.")
        return nil
    },
}

func init() {
    DBCmd.AddCommand(resetCmd)
    resetCmd.Flags().BoolVar(&flagResetForce, "force", false, "Required: confirm destructive reset")
    resetCmd.Flags().BoolVar(&flagResetDropDB, "drop-db", true, "Drop the target database")
    resetCmd.Flags().BoolVar(&flagResetDropAppRole, "drop-app-role", true, "Drop the app role")
    resetCmd.Flags().BoolVar(&flagResetDropAdmin, "drop-admin-role", false, "Drop the admin role (requires connecting as a different superuser)")
}

