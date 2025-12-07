package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// ProviderType enumerates supported LLM providers.
type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderGemini ProviderType = "gemini"
	ProviderOllama ProviderType = "ollama"
)

// ToolConfig holds minimal configuration for creating provider clients.
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

// SecretMetadata contains secret value resolved from vault.
type SecretMetadata struct {
	Value string
}

// LLMFactory constructs provider-specific LLM instances using langchaingo.
type LLMFactory interface {
	NewLLM(
		ctx context.Context,
		cfg *ToolConfig,
		secret *SecretMetadata,
	) (llms.LLM, error)
}

type factoryImpl struct{}

// New returns a default LLMFactory implementation.
func New() LLMFactory { return &factoryImpl{} }

// NewLLM creates a provider client based on cfg.Provider.
func (f *factoryImpl) NewLLM(ctx context.Context, cfg *ToolConfig, secret *SecretMetadata) (llms.LLM, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llmfactory: missing tool config")
	}
	provider := ProviderType(strings.ToLower(string(cfg.Provider)))
	if cfg.Model == "" {
		return nil, fmt.Errorf("llmfactory: missing model for provider %q", provider)
	}

	switch provider {
	case ProviderOpenAI:
		// API key typically supplied via environment; if provided, newer SDKs may support WithToken.
		opts := []openai.Option{}
		if strings.TrimSpace(cfg.BaseURL) != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
		}
		llm, err := openai.New(opts...)
		if err != nil {
			return nil, fmt.Errorf("llmfactory: openai init: %w", err)
		}
		return llm, nil

	case ProviderGemini:
		gopts := []googleai.Option{}
		if secret != nil && strings.TrimSpace(secret.Value) != "" {
			gopts = append(gopts, googleai.WithAPIKey(secret.Value))
		}
		llm, err := googleai.New(ctx, gopts...)
		if err != nil {
			return nil, fmt.Errorf("llmfactory: gemini init: %w", err)
		}
		return llm, nil

	case ProviderOllama:
		oopts := []ollama.Option{}
		if strings.TrimSpace(cfg.BaseURL) != "" {
			oopts = append(oopts, ollama.WithServerURL(cfg.BaseURL))
		}
		llm, err := ollama.New(oopts...)
		if err != nil {
			return nil, fmt.Errorf("llmfactory: ollama init: %w", err)
		}
		return llm, nil

	default:
		return nil, fmt.Errorf("llmfactory: unsupported provider %q", cfg.Provider)
	}
}
