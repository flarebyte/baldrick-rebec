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
    Short: "Use admin credentials from config to create tables/schema",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil {
            return err
        }

        // Require admin credentials to be present
        if cfg.Postgres.Admin.User == "" || (cfg.Postgres.Admin.Password == "" && cfg.Postgres.Admin.PasswordTemp == "") {
            return errors.New("postgres admin credentials missing; set postgres.admin.user and one of postgres.admin.password or postgres.admin.password_temp in config.yaml")
        }

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        fmt.Fprintf(os.Stderr, "db:scaffold - connecting to Postgres as admin user %q...\n", cfg.Postgres.Admin.User)
        db, err := pgdao.OpenAdmin(ctx, cfg)
        if err != nil {
            return err
        }
        defer db.Close()

        fmt.Fprintln(os.Stderr, "db:scaffold - ensuring schema (tables, triggers)...")
        if err := pgdao.EnsureSchema(ctx, db); err != nil {
            return err
        }

        fmt.Fprintln(os.Stderr, "db:scaffold - done")
        return nil
    },
}

func init() {
    DBCmd.AddCommand(scaffoldCmd)
}

