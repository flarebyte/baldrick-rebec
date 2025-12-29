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

var revokePrivsCmd = &cobra.Command{
	Use:   "revoke-privileges",
	Short: "Revoke runtime privileges from app role in target database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagYes {
			return errors.New("refusing to revoke without --yes; re-run with --yes to confirm")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Fprintf(os.Stderr, "db:revoke - connecting to target DB %q as admin...\n", cfg.Postgres.DBName)
		db, err := pgdao.OpenAdmin(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		fmt.Fprintf(os.Stderr, "db:revoke - revoking privileges from %q...\n", cfg.Postgres.App.User)
		if err := pgdao.RevokeRuntimePrivileges(ctx, db, cfg.Postgres.App.User); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "db:revoke - done")
		return nil
	},
}

func init() {
	DBCmd.AddCommand(revokePrivsCmd)
}
