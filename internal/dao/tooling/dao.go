package tooling

import (
	"context"
	"errors"
)

// ProviderType enumerates supported LLM providers.
type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderGemini ProviderType = "gemini"
	ProviderOllama ProviderType = "ollama"
)

// ToolConfig models tool-level configuration used to build LLM clients.
// Matches prior specifications used by the factory and service layers.
type ToolConfig struct {
	Name            string
	Provider        ProviderType // "openai", "gemini", "ollama"
	Model           string
	BaseURL         string
	APIKeySecret    string
	Temperature     *float32
	MaxOutputTokens *int
	TopP            *float32
	Settings        map[string]any
}

// SecretMetadata represents a resolved secret.
type SecretMetadata struct {
	Value string
}

// Errors returned by DAO implementations.
var (
	ErrToolNotFound   = errors.New("tool not found")
	ErrSecretNotFound = errors.New("secret not found")
)

// ToolDAO provides read access to tool configurations.
type ToolDAO interface {
	GetToolByName(ctx context.Context, name string) (*ToolConfig, error)
}

// VaultDAO provides access to secret metadata by name or key reference.
type VaultDAO interface {
	GetSecretMetadata(ctx context.Context, name string) (*SecretMetadata, error)
}
