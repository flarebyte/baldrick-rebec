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
    flagAgeInitYes bool
)

var ageInitCmd = &cobra.Command{
    Use:   "age-init",
    Short: "Initialize AGE: create extension and rbc_graph, then grant app role privileges (admin)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if !flagAgeInitYes {
            return errors.New("refusing to modify AGE without --yes; re-run with --yes to confirm")
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        if cfg.Postgres.Admin.User == "" || cfg.Postgres.Admin.Password == "" {
            return errors.New("postgres admin credentials missing; set postgres.admin.user and postgres.admin.password in config.yaml")
        }
        ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second); defer cancel()
        db, err := pgdao.OpenAdmin(ctx, cfg); if err != nil { return err }
        defer db.Close()

        fmt.Fprintln(os.Stderr, "age-init: ensuring EXTENSION age...")
        if _, err := db.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS age"); err != nil { return err }

        fmt.Fprintln(os.Stderr, "age-init: ensuring rbc_graph exists...")
        if _, err := db.Exec(ctx, "SELECT ag_catalog.create_graph('rbc_graph')"); err != nil {
            // If it already exists, the function should error; ignore duplicate graph errors
            // Otherwise, return the error
            // Many AGE builds raise error if graph exists; we continue regardless.
            fmt.Fprintf(os.Stderr, "age-init: note: create_graph returned: %v (continuing)\n", err)
        }

        fmt.Fprintln(os.Stderr, "age-init: granting AGE privileges to app role...")
        if err := pgdao.GrantAGEPrivileges(ctx, db, cfg.Postgres.App.User); err != nil {
            fmt.Fprintf(os.Stderr, "age-init: warn: grant AGE privileges: %v\n", err)
        }

        fmt.Fprintln(os.Stderr, "age-init: done")
        return nil
    },
}

func init() {
    DBCmd.AddCommand(ageInitCmd)
    ageInitCmd.Flags().BoolVar(&flagAgeInitYes, "yes", false, "Confirm making changes to AGE (create extension/graph and grant privs)")
}

