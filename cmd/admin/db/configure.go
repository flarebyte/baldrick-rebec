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
    flagOverwrite     bool
    // Server
    flagServerPort    int
    // Postgres
    flagPGHost        string
    flagPGPort        int
    flagPGUser        string
    flagPGPassword    string
    flagPGDBName      string
    flagPGSSLMode     string
    // OpenSearch
    flagOSScheme      string
    flagOSHost        string
    flagOSPort        int
    flagOSUsername    string
    flagOSPassword    string
    flagOSInsecure    bool
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
                Host: flagPGHost, Port: flagPGPort, User: flagPGUser,
                Password: flagPGPassword, DBName: flagPGDBName, SSLMode: flagPGSSLMode,
            },
            OpenSearch: cfgpkg.OpenSearchConfig{
                Scheme: flagOSScheme, Host: flagOSHost, Port: flagOSPort,
                Username: flagOSUsername, Password: flagOSPassword,
                InsecureSkipVerify: flagOSInsecure,
            },
        }

        // Fill defaults when zero
        if cfg.Server.Port == 0 { cfg.Server.Port = cfgpkg.DefaultServerPort }
        if cfg.Postgres.Host == "" { cfg.Postgres.Host = "127.0.0.1" }
        if cfg.Postgres.Port == 0 { cfg.Postgres.Port = 5432 }
        if cfg.Postgres.User == "" { cfg.Postgres.User = "rbc" }
        if cfg.Postgres.DBName == "" { cfg.Postgres.DBName = "rbc" }
        if cfg.Postgres.SSLMode == "" { cfg.Postgres.SSLMode = "disable" }
        if cfg.OpenSearch.Scheme == "" { cfg.OpenSearch.Scheme = "http" }
        if cfg.OpenSearch.Host == "" { cfg.OpenSearch.Host = "127.0.0.1" }
        if cfg.OpenSearch.Port == 0 { cfg.OpenSearch.Port = cfgpkg.DefaultOpenSearchPort }

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
    configureCmd.Flags().StringVar(&flagPGUser, "pg-user", "rbc", "Postgres user")
    configureCmd.Flags().StringVar(&flagPGPassword, "pg-password", "", "Postgres password")
    configureCmd.Flags().StringVar(&flagPGDBName, "pg-dbname", "rbc", "Postgres database name")
    configureCmd.Flags().StringVar(&flagPGSSLMode, "pg-sslmode", "disable", "Postgres SSL mode")

    configureCmd.Flags().StringVar(&flagOSScheme, "os-scheme", "http", "OpenSearch scheme (http/https)")
    configureCmd.Flags().StringVar(&flagOSHost, "os-host", "127.0.0.1", "OpenSearch host")
    configureCmd.Flags().IntVar(&flagOSPort, "os-port", cfgpkg.DefaultOpenSearchPort, "OpenSearch port")
    configureCmd.Flags().StringVar(&flagOSUsername, "os-username", "", "OpenSearch username")
    configureCmd.Flags().StringVar(&flagOSPassword, "os-password", "", "OpenSearch password")
    configureCmd.Flags().BoolVar(&flagOSInsecure, "os-insecure", false, "OpenSearch: skip TLS verification (dev)")
}

