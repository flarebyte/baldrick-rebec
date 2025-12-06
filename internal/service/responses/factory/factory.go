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
        // Validate API key
        if secret == nil || strings.TrimSpace(secret.Value) == "" {
            return nil, fmt.Errorf("llmfactory: openai missing API key secret")
        }
        opts := []openai.Option{
            openai.WithModel(cfg.Model),
            openai.WithAPIKey(secret.Value),
        }
        if strings.TrimSpace(cfg.BaseURL) != "" {
            opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
        }
        llm, err := openai.New(opts...)
        if err != nil {
            return nil, fmt.Errorf("llmfactory: openai init: %w", err)
        }
        return llm, nil

    case ProviderGemini:
        if secret == nil || strings.TrimSpace(secret.Value) == "" {
            return nil, fmt.Errorf("llmfactory: gemini missing API key secret")
        }
        gopts := []googleai.Option{
            googleai.WithAPIKey(secret.Value),
            googleai.WithModel(cfg.Model),
        }
        // Optional project and location
        if cfg.Settings != nil {
            if v, ok := cfg.Settings["project"].(string); ok && strings.TrimSpace(v) != "" {
                // Note: option name may vary by version; adhere to requirement.
                gopts = append(gopts, googleai.WithProject(v))
            }
            if v, ok := cfg.Settings["location"].(string); ok && strings.TrimSpace(v) != "" {
                gopts = append(gopts, googleai.WithLocation(v))
            }
        }
        llm, err := googleai.New(gopts...)
        if err != nil {
            return nil, fmt.Errorf("llmfactory: gemini init: %w", err)
        }
        return llm, nil

    case ProviderOllama:
        oopts := []ollama.Option{
            ollama.WithModel(cfg.Model),
        }
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

