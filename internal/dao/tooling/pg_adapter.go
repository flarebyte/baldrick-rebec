package tooling

import (
    "context"
    "fmt"

    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/jackc/pgx/v5/pgxpool"
)

// PGToolDAOAdapter implements ToolDAO backed by Postgres tools table.
type PGToolDAOAdapter struct {
    DB *pgxpool.Pool
}

// NewPGToolDAOAdapter creates a new adapter using the given pool.
func NewPGToolDAOAdapter(db *pgxpool.Pool) *PGToolDAOAdapter {
    return &PGToolDAOAdapter{DB: db}
}

// GetToolByName loads a tool by name and maps settings into ToolConfig.
func (a *PGToolDAOAdapter) GetToolByName(ctx context.Context, name string) (*ToolConfig, error) {
    if a == nil || a.DB == nil {
        return nil, fmt.Errorf("pgtooldao: not initialized")
    }
    t, err := pgdao.GetToolByName(ctx, a.DB, name)
    if err != nil {
        return nil, err
    }
    cfg := &ToolConfig{
        Name:     t.Name,
        Settings: t.Settings,
    }
    // Map known keys from settings (stringly-typed JSON)
    if s := t.Settings; s != nil {
        if v, ok := s["provider"].(string); ok {
            cfg.Provider = ProviderType(v)
        }
        if v, ok := s["model"].(string); ok {
            cfg.Model = v
        }
        if v, ok := s["base_url"].(string); ok {
            cfg.BaseURL = v
        }
        if v, ok := s["api_key_secret"].(string); ok {
            cfg.APIKeySecret = v
        }
        if v, ok := s["temperature"].(float64); ok {
            vv := float32(v)
            cfg.Temperature = &vv
        }
        if v, ok := s["max_output_tokens"].(float64); ok {
            vi := int(v)
            cfg.MaxOutputTokens = &vi
        }
        if v, ok := s["top_p"].(float64); ok {
            vv := float32(v)
            cfg.TopP = &vv
        }
    }
    return cfg, nil
}

