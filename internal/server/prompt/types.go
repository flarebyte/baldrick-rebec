package prompt

import (
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// Proto-compatible structs for JSON codec based gRPC.

type ToolFunction struct {
	Name       string         `json:"name,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type ToolDefinition struct {
	Type     string       `json:"type,omitempty"`
	Function ToolFunction `json:"function,omitempty"`
}

type ToolCall struct {
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type ContentBlock struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

type Usage struct {
	InputTokens  int32 `json:"input_tokens,omitempty"`
	OutputTokens int32 `json:"output_tokens,omitempty"`
	TotalTokens  int32 `json:"total_tokens,omitempty"`
}

type PromptRunRequest struct {
	ToolName        string           `json:"tool_name,omitempty"`
	Model           string           `json:"model,omitempty"`
	Input           any              `json:"input,omitempty"`
	Tools           []ToolDefinition `json:"tools,omitempty"`
	Temperature     float32          `json:"temperature,omitempty"`
	MaxOutputTokens int32            `json:"max_output_tokens,omitempty"`
}

type PromptRunResponse struct {
	Id      string         `json:"id,omitempty"`
	Object  string         `json:"object,omitempty"`
	Model   string         `json:"model,omitempty"`
	Created int64          `json:"created,omitempty"`
	Output  []ContentBlock `json:"output,omitempty"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// Helpers for structpb conversion when needed by future proto-based clients.
func toStructPB(m map[string]any) *structpb.Struct {
	if m == nil {
		return nil
	}
	s, _ := structpb.NewStruct(m)
	return s
}
