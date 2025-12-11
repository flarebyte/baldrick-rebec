package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tmc/langchaingo/llms"
)

// ResponsesService defines the service behavior for creating responses using an injected LLM.
type ResponsesService interface {
	CreateResponse(
		ctx context.Context,
		cfg *ToolConfig,
		req *ResponseRequest,
		llm llms.LLM,
	) (*Response, error)
}

// Service is a concrete implementation of ResponsesService.
type Service struct{}

// New creates a new Service instance.
func New() *Service { return &Service{} }

// ToolConfig provides tool-specific configuration for LLM calls.
// Provider-agnostic; providers are handled inside LLMFactory.
type ToolConfig struct {
	Name               string
	Provider           string
	Model              string
	APIKeySecret       string
	Settings           map[string]any
	DefaultTemperature *float32
	DefaultMaxTokens   *int
	DefaultTopP        *float32
}

// ResponseRequest mirrors the HTTP-layer request accepted by the endpoint.
type ResponseRequest struct {
	Model           string           `json:"model"`
	Input           any              `json:"input"`
	Temperature     *float32         `json:"temperature,omitempty"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"`
	Tools           []ToolDefinition `json:"tools,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
}

// ToolDefinition is a minimal tool/function schema.
type ToolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// NormalizedToolCall captures a provider-agnostic tool call.
type NormalizedToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ContentBlock is a canonical output block.
type ContentBlock struct {
	Type     string              `json:"type"` // "output_text" or "tool_call"
	Text     string              `json:"text,omitempty"`
	ToolCall *NormalizedToolCall `json:"tool_call,omitempty"`
}

// Usage contains token accounting.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Response is the canonical response object.
type Response struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Created int64          `json:"created"`
	Output  []ContentBlock `json:"output"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// CreateResponse implements the ResponsesService interface.
func (s *Service) CreateResponse(
	ctx context.Context,
	cfg *ToolConfig,
	req *ResponseRequest,
	llm llms.LLM,
) (*Response, error) {
	if cfg == nil {
		return nil, fmt.Errorf("responses: missing tool config")
	}
	if req == nil {
		return nil, fmt.Errorf("responses: missing request")
	}

	// a) Determine final model
	finalModel := firstNonEmpty(req.Model, cfg.Model)

	// b) Convert input into a single prompt string (best-effort)
	prompt, err := normalizeInputToPrompt(req.Input)
	if err != nil {
		return nil, fmt.Errorf("responses: normalize input: %w", err)
	}

	// c) Merge parameters: request overrides config defaults
	var opts []llms.CallOption
	if t := firstPtr(req.Temperature, cfg.DefaultTemperature); t != nil {
		opts = append(opts, llms.WithTemperature(float64(*t)))
	}
	if mt := firstPtr(req.MaxOutputTokens, cfg.DefaultMaxTokens); mt != nil {
		opts = append(opts, llms.WithMaxTokens(*mt))
	}
	if tp := cfg.DefaultTopP; req.Temperature == nil { // top_p is independent; include if set
		if tp != nil {
			opts = append(opts, llms.WithTopP(float64(*tp)))
		}
	} else if cfg.DefaultTopP != nil {
		// include top_p default even when temperature present, if the client did not specify an explicit top_p
		opts = append(opts, llms.WithTopP(float64(*cfg.DefaultTopP)))
	}

	// d) Convert tools to function definitions (best-effort)
	fns := toLLMFunctions(req.Tools)
	if len(fns) > 0 {
		opts = append(opts, llms.WithFunctions(fns))
	}

	// Provider/model hints if supported by backend
	if finalModel != "" {
		opts = append(opts, llms.WithModel(finalModel))
	}

	// e) Non-streaming generate call.
	// We prefer the helper that returns a single string; providers may ignore unsupported options.
	text, err := llms.GenerateFromSinglePrompt(ctx, llm, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("responses: llm generate: %w", err)
	}

	// f) Normalize outputs into content blocks. We only have plain text from the helper above.
	blocks := []ContentBlock{
		{Type: "output_text", Text: text},
	}

	// g) Token usage: Unavailable via helper; default zeros.
	usage := &Usage{InputTokens: 0, OutputTokens: 0, TotalTokens: 0}

	// h) Build response
	id := ulid.Make().String()
	resp := &Response{
		ID:      id,
		Object:  "response",
		Model:   finalModel,
		Created: time.Now().Unix(),
		Output:  blocks,
		Usage:   usage,
	}
	return resp, nil
}

// normalizeInputToPrompt converts supported input shapes into a single prompt string.
func normalizeInputToPrompt(in any) (string, error) {
	switch v := in.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []map[string]any:
		var b strings.Builder
		for _, m := range v {
			// Prefer OpenAI-style {type:"text", text:"..."}
			if t, _ := m["type"].(string); t == "text" {
				if txt, _ := m["text"].(string); txt != "" {
					b.WriteString(txt)
					if !strings.HasSuffix(txt, "\n") {
						b.WriteString("\n")
					}
					continue
				}
			}
			// Fallbacks: content or raw JSON
			if txt, _ := m["content"].(string); txt != "" {
				b.WriteString(txt)
				if !strings.HasSuffix(txt, "\n") {
					b.WriteString("\n")
				}
				continue
			}
			// As a last resort, JSON-encode the block
			if enc, err := json.Marshal(m); err == nil {
				b.Write(enc)
				b.WriteString("\n")
			}
		}
		return strings.TrimSpace(b.String()), nil
	case []any:
		// Attempt to coerce []any into []map[string]any or concatenate strings
		var b strings.Builder
		for _, item := range v {
			switch t := item.(type) {
			case string:
				b.WriteString(t)
				if !strings.HasSuffix(t, "\n") {
					b.WriteString("\n")
				}
			case map[string]any:
				s, _ := normalizeInputToPrompt([]map[string]any{t})
				if s != "" {
					b.WriteString(s)
					if !strings.HasSuffix(s, "\n") {
						b.WriteString("\n")
					}
				}
			default:
				if enc, err := json.Marshal(t); err == nil {
					b.Write(enc)
					b.WriteString("\n")
				}
			}
		}
		return strings.TrimSpace(b.String()), nil
	default:
		// Best-effort JSON encoding
		enc, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("encode input: %w", err)
		}
		return string(enc), nil
	}
}

// toLLMFunctions converts ToolDefinition into langchaingo function definitions.
func toLLMFunctions(tools []ToolDefinition) []llms.FunctionDefinition {
	if len(tools) == 0 {
		return nil
	}
	out := make([]llms.FunctionDefinition, 0, len(tools))
	for _, t := range tools {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		var params any
		if t.Parameters != nil {
			params = t.Parameters
		}
		out = append(out, llms.FunctionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return out
}

// firstNonEmpty returns a if non-empty, else b.
func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// firstPtr returns the first non-nil pointer.
func firstPtr[T any](a, b *T) *T {
	if a != nil {
		return a
	}
	return b
}
