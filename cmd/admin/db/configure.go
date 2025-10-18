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
        if !flagOverwrite {
            if _, err := os.Stat(path); err == nil {
                return fmt.Errorf("config already exists at %s (use --overwrite to replace)", path)
            }
        }

        cfg := cfgpkg.Config{
            Server: cfgpkg.ServerConfig{Port: flagServerPort},
            Postgres: cfgpkg.PostgresConfig{
                Host: flagPGHost, Port: flagPGPort, DBName: flagPGDBName, SSLMode: flagPGSSLMode,
                App:   cfgpkg.PGRole{User: flagPGAppUser, Password: flagPGAppPassword},
                Admin: cfgpkg.PGRole{User: flagPGAdminUser, Password: flagPGAdminPassword, PasswordTemp: flagPGAdminPasswordTmp},
            },
            OpenSearch: cfgpkg.OpenSearchConfig{
                Scheme: flagOSScheme, Host: flagOSHost, Port: flagOSPort, InsecureSkipVerify: flagOSInsecure,
                App:   cfgpkg.OSRole{Username: flagOSAppUsername, Password: flagOSAppPassword},
                Admin: cfgpkg.OSRole{Username: flagOSAdminUsername, Password: flagOSAdminPassword, PasswordTemp: flagOSAdminPasswordTmp},
            },
        }

        // Fill defaults when zero
        if cfg.Server.Port == 0 { cfg.Server.Port = cfgpkg.DefaultServerPort }
        if cfg.Postgres.Host == "" { cfg.Postgres.Host = "127.0.0.1" }
        if cfg.Postgres.Port == 0 { cfg.Postgres.Port = 5432 }
        if cfg.Postgres.DBName == "" { cfg.Postgres.DBName = "rbc" }
        if cfg.Postgres.SSLMode == "" { cfg.Postgres.SSLMode = "disable" }
        if cfg.Postgres.App.User == "" { cfg.Postgres.App.User = "rbc_app" }
        if cfg.Postgres.Admin.User == "" { cfg.Postgres.Admin.User = "rbc_admin" }
        if cfg.OpenSearch.Scheme == "" { cfg.OpenSearch.Scheme = "http" }
        if cfg.OpenSearch.Host == "" { cfg.OpenSearch.Host = "127.0.0.1" }
        if cfg.OpenSearch.Port == 0 { cfg.OpenSearch.Port = cfgpkg.DefaultOpenSearchPort }
        if cfg.OpenSearch.App.Username == "" { cfg.OpenSearch.App.Username = "rbc_app" }
        if cfg.OpenSearch.Admin.Username == "" { cfg.OpenSearch.Admin.Username = "admin" }

        b, err := yaml.Marshal(cfg)
        if err != nil {
            return err
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
