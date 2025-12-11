package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ResponseRequest represents the subset of the OpenAI Responses API request we care about.
// It is used to decode the incoming JSON body.
type ResponseRequest struct {
	Model           string           `json:"model"`
	Input           any              `json:"input"`
	Temperature     *float32         `json:"temperature,omitempty"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"`
	Tools           []ToolDefinition `json:"tools,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
}

// ToolDefinition represents a tool/function definition referenced by the request.
// The shape intentionally mirrors common tool/function schemas but remains minimal.
type ToolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	// Extra captures any vendor-specific fields without failing decoding.
	Extra map[string]any `json:"-"`
}

// Usage encapsulates token accounting in the OpenAI Responses format.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// CreateResponseResult is the expected response payload shape.
type CreateResponseResult struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Created int64  `json:"created"`
	Output  []any  `json:"output"`
	Usage   *Usage `json:"usage,omitempty"`
}

// ToolConfig is a minimal configuration model for a tool used to construct the LLM.
// Implementations can embed or map this to their persistence layer.
type ToolConfig struct {
	Name         string
	RoleName     string
	Model        string
	APIKeySecret string
	Settings     map[string]any
	// Additional fields may be added as needed by the factory/service.
}

// SecretMetadata is an abstract representation of a secret resolved from a vault.
type SecretMetadata struct {
	Name     string
	Value    string
	Metadata map[string]string
}

// LLM is an opaque, implementation-defined large language model handle.
type LLM interface{}

// ToolDAO provides lookup for tool configuration by name.
type ToolDAO interface {
	GetToolByName(ctx context.Context, name string) (*ToolConfig, error)
}

// VaultDAO fetches secret metadata when a tool configuration references a secret by key.
type VaultDAO interface {
	GetSecretMetadata(ctx context.Context, key string) (*SecretMetadata, error)
}

// LLMFactory constructs an LLM handle given a tool configuration and optional secret.
type LLMFactory interface {
	NewLLM(ctx context.Context, tool *ToolConfig, secret *SecretMetadata) (LLM, error)
}

// ResponsesService forwards the request to the underlying LLM and returns a formatted result.
type ResponsesService interface {
	CreateResponse(ctx context.Context, tool *ToolConfig, req *ResponseRequest, llm LLM) (*CreateResponseResult, error)
}

// Handler bundles dependencies for the prompt responses endpoint.
type Handler struct {
	toolDAO          ToolDAO
	vaultDAO         VaultDAO
	llmFactory       LLMFactory
	responsesService ResponsesService
}

// New constructs a new Handler with its dependencies.
func New(toolDAO ToolDAO, vaultDAO VaultDAO, llmFactory LLMFactory, responsesService ResponsesService) *Handler {
	return &Handler{
		toolDAO:          toolDAO,
		vaultDAO:         vaultDAO,
		llmFactory:       llmFactory,
		responsesService: responsesService,
	}
}

// Router wires the handler into a chi router at /prompt/v1/{tool}/responses.
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/prompt/v1/{tool}/responses", h.postResponses)
	return r
}

// postResponses handles POST /prompt/v1/{tool}/responses with a single, non-streaming response.
func (h *Handler) postResponses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()
	toolName := chi.URLParam(r, "tool")
	if toolName == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "tool path parameter is required", "missing_tool_param")
		return
	}

	var req ResponseRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body: "+err.Error(), "invalid_json")
		return
	}

	toolCfg, err := h.toolDAO.GetToolByName(ctx, toolName)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "failed to fetch tool config", "tool_lookup_failed")
		return
	}
	if toolCfg == nil {
		// Per requirement, missing tool returns OpenAI-style invalid_request_error with tool_not_found code.
		// Use 404 to align with common OpenAI semantics for missing resources.
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "tool not found", "tool_not_found")
		return
	}

	var secret *SecretMetadata
	if toolCfg.APIKeySecret != "" {
		s, err := h.vaultDAO.GetSecretMetadata(ctx, toolCfg.APIKeySecret)
		if err != nil {
			writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "failed to resolve tool secret", "secret_resolve_failed")
			return
		}
		secret = s
	}

	llm, err := h.llmFactory.NewLLM(ctx, toolCfg, secret)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "failed to initialize LLM: "+safeErr(err), "llm_init_failed")
		return
	}

	resp, err := h.responsesService.CreateResponse(ctx, toolCfg, &req, llm)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "failed to create response: "+safeErr(err), "response_create_failed")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// writeJSON writes a value as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// openAIError matches the typical OpenAI error envelope.
type openAIError struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// writeOpenAIError formats and writes an OpenAI-style error envelope.
func writeOpenAIError(w http.ResponseWriter, status int, typ, message, code string) {
	var e openAIError
	e.Error.Type = typ
	e.Error.Message = message
	e.Error.Code = code
	writeJSON(w, status, e)
}

// safeErr returns a minimal error string without exposing nested details.
func safeErr(err error) string {
	if err == nil {
		return ""
	}
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) {
		if inner := ue.Unwrap(); inner != nil {
			return inner.Error()
		}
	}
	return err.Error()
}
