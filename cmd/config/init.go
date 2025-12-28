package configcmd

import (
	"fmt"
	"os"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	"github.com/flarebyte/baldrick-rebec/internal/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	flagOverwrite bool
	flagDryRun    bool
	// Server
	flagServerPort int
	// Postgres base
	flagPGHost    string
	flagPGPort    int
	flagPGDBName  string
	flagPGSSLMode string
	// Postgres app creds
	flagPGAppUser     string
	flagPGAppPassword string
	// Postgres admin creds
	flagPGAdminUser     string
	flagPGAdminPassword string
	// (OpenSearch flags removed)
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create or update the global config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := paths.EnsureHome(); err != nil {
			return err
		}
		path := cfgpkg.Path()
		if !flagOverwrite && !flagDryRun {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config already exists at %s (use --overwrite to replace)", path)
			}
		}

		// Start from existing config (or defaults if missing) to preserve secrets
		cfg, _ := cfgpkg.Load()

		// Server
		if cmd.Flags().Changed("server-port") {
			cfg.Server.Port = flagServerPort
		}

		// Postgres base
		if cmd.Flags().Changed("pg-host") {
			cfg.Postgres.Host = flagPGHost
		}
		if cmd.Flags().Changed("pg-port") {
			cfg.Postgres.Port = flagPGPort
		}
		if cmd.Flags().Changed("pg-dbname") {
			cfg.Postgres.DBName = flagPGDBName
		}
		if cmd.Flags().Changed("pg-sslmode") {
			cfg.Postgres.SSLMode = flagPGSSLMode
		}
		// Postgres roles
		if cmd.Flags().Changed("pg-app-user") {
			cfg.Postgres.App.User = flagPGAppUser
		}
		if cmd.Flags().Changed("pg-app-password") {
			cfg.Postgres.App.Password = flagPGAppPassword
		}
		if cmd.Flags().Changed("pg-admin-user") {
			cfg.Postgres.Admin.User = flagPGAdminUser
		}
		if cmd.Flags().Changed("pg-admin-password") {
			cfg.Postgres.Admin.Password = flagPGAdminPassword
		}
		// no temp password; single admin password

		// No feature flags

		b, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if flagDryRun {
			os.Stdout.Write(b)
			if len(b) == 0 || b[len(b)-1] != '\n' {
				fmt.Fprintln(os.Stdout)
			}
			fmt.Fprintf(os.Stderr, "dry-run: not writing %s\n", path)
			return nil
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote config to %s\n", path)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&flagOverwrite, "overwrite", false, "Overwrite existing config.yaml if present")
	initCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print merged config to stdout without writing")

	initCmd.Flags().IntVar(&flagServerPort, "server-port", cfgpkg.DefaultServerPort, "Server port")

	initCmd.Flags().StringVar(&flagPGHost, "pg-host", "127.0.0.1", "Postgres host")
	initCmd.Flags().IntVar(&flagPGPort, "pg-port", 5432, "Postgres port")
	initCmd.Flags().StringVar(&flagPGDBName, "pg-dbname", "rbc", "Postgres database name")
	initCmd.Flags().StringVar(&flagPGSSLMode, "pg-sslmode", "disable", "Postgres SSL mode")

	initCmd.Flags().StringVar(&flagPGAppUser, "pg-app-user", "rbc_app", "Postgres app user (runtime)")
	initCmd.Flags().StringVar(&flagPGAppPassword, "pg-app-password", "", "Postgres app password")
	initCmd.Flags().StringVar(&flagPGAdminUser, "pg-admin-user", "rbc_admin", "Postgres admin user (migrations)")
	initCmd.Flags().StringVar(&flagPGAdminPassword, "pg-admin-password", "", "Postgres admin password")

	// No feature flags
}
