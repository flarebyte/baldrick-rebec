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
    DefaultOpenSearchPort = 9200
)

type ServerConfig struct {
    Port int `yaml:"port"`
}

type OpenSearchConfig struct {
    Port int `yaml:"port"`
}

type Config struct {
    Server     ServerConfig     `yaml:"server"`
    OpenSearch OpenSearchConfig `yaml:"opensearch"`
}

func defaults() Config {
    return Config{
        Server:     ServerConfig{Port: DefaultServerPort},
        OpenSearch: OpenSearchConfig{Port: DefaultOpenSearchPort},
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
    if fileCfg.OpenSearch.Port != 0 {
        cfg.OpenSearch.Port = fileCfg.OpenSearch.Port
    }
    return cfg, nil
}

