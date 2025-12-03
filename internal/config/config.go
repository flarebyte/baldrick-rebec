package config

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "github.com/flarebyte/baldrick-rebec/internal/paths"
    "gopkg.in/yaml.v3"
)

const (
    DefaultServerPort    = 53051
)

type ServerConfig struct {
    Port int `yaml:"port"`
}

type Config struct {
    Server     ServerConfig     `yaml:"server"`
    Postgres   PostgresConfig   `yaml:"postgres"`
    Graph      GraphConfig      `yaml:"graph"`
    Vault      VaultConfig      `yaml:"vault"`
}

func defaults() Config {
    return Config{
        Server:     ServerConfig{Port: DefaultServerPort},
        Postgres:   PostgresConfig{Host: "127.0.0.1", Port: 5432, DBName: "rbc", SSLMode: "disable",
            Admin: PGRole{User: "rbc_admin"}, App: PGRole{User: "rbc_app"}, Backup: PGRole{}},
        Graph:      GraphConfig{AllowFallback: false},
        Vault:      VaultConfig{Backend: "keychain"},
    }
}

// Path returns the expected path to the config.yaml file.
func Path() string {
    return filepath.Join(paths.Home(), "config.yaml")
}

// Load reads configuration from config.yaml if it exists.
// Missing file is not an error; defaults are returned.
func Load() (Config, error) {
    cfg := defaults()
    p := Path()
    b, err := os.ReadFile(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return cfg, nil
        }
        return cfg, fmt.Errorf("read config: %w", err)
    }
    var fileCfg Config
    if err := yaml.Unmarshal(b, &fileCfg); err != nil {
        return cfg, fmt.Errorf("parse config: %w", err)
    }
    // Merge: override defaults with provided values if non-zero
    if fileCfg.Server.Port != 0 {
        cfg.Server.Port = fileCfg.Server.Port
    }
    // Postgres overrides
    if fileCfg.Postgres.Host != "" {
        cfg.Postgres.Host = fileCfg.Postgres.Host
    }
    if fileCfg.Postgres.Port != 0 {
        cfg.Postgres.Port = fileCfg.Postgres.Port
    }
    if fileCfg.Postgres.DBName != "" {
        cfg.Postgres.DBName = fileCfg.Postgres.DBName
    }
    if fileCfg.Postgres.SSLMode != "" {
        cfg.Postgres.SSLMode = fileCfg.Postgres.SSLMode
    }
    if fileCfg.Postgres.Admin.User != "" {
        cfg.Postgres.Admin.User = fileCfg.Postgres.Admin.User
    }
    if fileCfg.Postgres.Admin.Password != "" {
        cfg.Postgres.Admin.Password = fileCfg.Postgres.Admin.Password
    }
    if fileCfg.Postgres.App.User != "" {
        cfg.Postgres.App.User = fileCfg.Postgres.App.User
    }
    if fileCfg.Postgres.App.Password != "" {
        cfg.Postgres.App.Password = fileCfg.Postgres.App.Password
    }
    if fileCfg.Postgres.Backup.User != "" {
        cfg.Postgres.Backup.User = fileCfg.Postgres.Backup.User
    }
    if fileCfg.Postgres.Backup.Password != "" {
        cfg.Postgres.Backup.Password = fileCfg.Postgres.Backup.Password
    }
    // Graph overrides
    // Booleans default to false; direct assignment is fine.
    cfg.Graph.AllowFallback = fileCfg.Graph.AllowFallback
    // Vault overrides
    if fileCfg.Vault.Backend != "" {
        cfg.Vault.Backend = fileCfg.Vault.Backend
    }
    return cfg, nil
}

type PostgresConfig struct {
    Host    string `yaml:"host"`
    Port    int    `yaml:"port"`
    DBName  string `yaml:"dbname"`
    SSLMode string `yaml:"sslmode"` // disable, require, verify-ca, verify-full
    Admin   PGRole `yaml:"admin"`
    App     PGRole `yaml:"app"`
    Backup  PGRole `yaml:"backup"`
}

type PGRole struct {
    User         string `yaml:"user"`
    Password     string `yaml:"password,omitempty"`
}

type GraphConfig struct {
    // AllowFallback controls whether CLI falls back to SQL mirror when graph (AGE) operations fail.
    // Default: false (no fallback) so issues are visible.
    AllowFallback bool `yaml:"allow_fallback"`
}

// VaultConfig controls secret storage backend used by the CLI/runtime.
// Default backend is "keychain" on macOS.
type VaultConfig struct {
    Backend string `yaml:"backend"`
}
