package db

import (
    "fmt"
    "os"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/flarebyte/baldrick-rebec/internal/paths"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

var (
    flagOverwrite  bool
    flagDryRun     bool
    // Server
    flagServerPort int
    // Postgres base
    flagPGHost     string
    flagPGPort     int
    flagPGDBName   string
    flagPGSSLMode  string
    // Postgres app creds
    flagPGAppUser     string
    flagPGAppPassword string
    // Postgres admin creds (temporary)
    flagPGAdminUser        string
    flagPGAdminPassword    string
    flagPGAdminPasswordTmp string
    // OpenSearch base
    flagOSScheme   string
    flagOSHost     string
    flagOSPort     int
    flagOSInsecure bool
    // OpenSearch app creds
    flagOSAppUsername string
    flagOSAppPassword string
    // OpenSearch admin creds (temporary)
    flagOSAdminUsername    string
    flagOSAdminPassword    string
    flagOSAdminPasswordTmp string
)

var configureCmd = &cobra.Command{
    Use:   "configure",
    Short: "Create or overwrite the global config.yaml",
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
        if cmd.Flags().Changed("pg-admin-password-temp") {
            cfg.Postgres.Admin.PasswordTemp = flagPGAdminPasswordTmp
        }

        // OpenSearch base
        if cmd.Flags().Changed("os-scheme") {
            cfg.OpenSearch.Scheme = flagOSScheme
        }
        if cmd.Flags().Changed("os-host") {
            cfg.OpenSearch.Host = flagOSHost
        }
        if cmd.Flags().Changed("os-port") {
            cfg.OpenSearch.Port = flagOSPort
        }
        if cmd.Flags().Changed("os-insecure") {
            cfg.OpenSearch.InsecureSkipVerify = flagOSInsecure
        }
        // OpenSearch roles
        if cmd.Flags().Changed("os-app-username") {
            cfg.OpenSearch.App.Username = flagOSAppUsername
        }
        if cmd.Flags().Changed("os-app-password") {
            cfg.OpenSearch.App.Password = flagOSAppPassword
        }
        if cmd.Flags().Changed("os-admin-username") {
            cfg.OpenSearch.Admin.Username = flagOSAdminUsername
        }
        if cmd.Flags().Changed("os-admin-password") {
            cfg.OpenSearch.Admin.Password = flagOSAdminPassword
        }
        if cmd.Flags().Changed("os-admin-password-temp") {
            cfg.OpenSearch.Admin.PasswordTemp = flagOSAdminPasswordTmp
        }

        b, err := yaml.Marshal(cfg)
        if err != nil {
            return err
        }
        if flagDryRun {
            // Print merged config to stdout without writing
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
    DBCmd.AddCommand(configureCmd)

    configureCmd.Flags().BoolVar(&flagOverwrite, "overwrite", false, "Overwrite existing config.yaml if present")
    configureCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print merged config to stdout without writing")

    configureCmd.Flags().IntVar(&flagServerPort, "server-port", cfgpkg.DefaultServerPort, "Server port")

    configureCmd.Flags().StringVar(&flagPGHost, "pg-host", "127.0.0.1", "Postgres host")
    configureCmd.Flags().IntVar(&flagPGPort, "pg-port", 5432, "Postgres port")
    configureCmd.Flags().StringVar(&flagPGDBName, "pg-dbname", "rbc", "Postgres database name")
    configureCmd.Flags().StringVar(&flagPGSSLMode, "pg-sslmode", "disable", "Postgres SSL mode")

    configureCmd.Flags().StringVar(&flagPGAppUser, "pg-app-user", "rbc_app", "Postgres app user (runtime)")
    configureCmd.Flags().StringVar(&flagPGAppPassword, "pg-app-password", "", "Postgres app password")
    configureCmd.Flags().StringVar(&flagPGAdminUser, "pg-admin-user", "rbc_admin", "Postgres admin user (migrations)")
    configureCmd.Flags().StringVar(&flagPGAdminPassword, "pg-admin-password", "", "Postgres admin password (avoid; prefer --pg-admin-password-temp)")
    configureCmd.Flags().StringVar(&flagPGAdminPasswordTmp, "pg-admin-password-temp", "", "Postgres admin temporary password (preferred; remove after use)")

    configureCmd.Flags().StringVar(&flagOSScheme, "os-scheme", "http", "OpenSearch scheme (http/https)")
    configureCmd.Flags().StringVar(&flagOSHost, "os-host", "127.0.0.1", "OpenSearch host")
    configureCmd.Flags().IntVar(&flagOSPort, "os-port", cfgpkg.DefaultOpenSearchPort, "OpenSearch port")
    configureCmd.Flags().BoolVar(&flagOSInsecure, "os-insecure", false, "OpenSearch: skip TLS verification (dev)")

    configureCmd.Flags().StringVar(&flagOSAppUsername, "os-app-username", "rbc_app", "OpenSearch app username (runtime)")
    configureCmd.Flags().StringVar(&flagOSAppPassword, "os-app-password", "", "OpenSearch app password")
    configureCmd.Flags().StringVar(&flagOSAdminUsername, "os-admin-username", "admin", "OpenSearch admin username (operator)")
    configureCmd.Flags().StringVar(&flagOSAdminPassword, "os-admin-password", "", "OpenSearch admin password (avoid; prefer --os-admin-password-temp)")
    configureCmd.Flags().StringVar(&flagOSAdminPasswordTmp, "os-admin-password-temp", "", "OpenSearch admin temporary password (preferred; remove after use)")
}
