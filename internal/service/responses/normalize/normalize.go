package normalize

import (
	"encoding/json"
	"strings"
)

// ToolDefinition represents an input tool definition from the JSON request.
// Shape mirrors OpenAI's tool/function schema.
type ToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name       string         `json:"name"`
		Parameters map[string]any `json:"parameters"`
	} `json:"function"`
}

// NormalizedTool is the internal canonical representation of a tool definition.
type NormalizedTool struct {
	Name       string
	Parameters map[string]any
}

// NormalizedToolCall is the internal canonical representation of a tool call returned by a provider.
type NormalizedToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolCallBlock is a minimal content block for tool calls.
type ToolCallBlock struct {
	Type     string             `json:"type"`
	ToolCall NormalizedToolCall `json:"tool_call"`
}

// NormalizeTools converts request tool definitions into normalized tools.
func NormalizeTools(tools []ToolDefinition) []NormalizedTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]NormalizedTool, 0, len(tools))
	for _, t := range tools {
		if strings.ToLower(strings.TrimSpace(t.Type)) != "function" {
			continue
		}
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		out = append(out, NormalizedTool{
			Name:       name,
			Parameters: t.Function.Parameters,
		})
	}
	return out
}

// NormalizeToolCalls attempts to extract tool calls from a provider-agnostic result.
// It supports common shapes seen across OpenAI (tool_calls/function_call), Gemini (functionCall), and Ollama.
// The input is expected to be JSON-like: map[string]any, []any, or nested combinations thereof.
func NormalizeToolCalls(result any) []NormalizedToolCall {
	var calls []NormalizedToolCall
	walkForToolCalls(result, &calls)
	return calls
}

// BuildToolCallBlocks converts normalized tool calls into minimal content blocks.
func BuildToolCallBlocks(calls []NormalizedToolCall) []ToolCallBlock {
	if len(calls) == 0 {
		return nil
	}
	blocks := make([]ToolCallBlock, 0, len(calls))
	for _, c := range calls {
		blocks = append(blocks, ToolCallBlock{Type: "tool_call", ToolCall: c})
	}
	return blocks
}

// walkForToolCalls recursively searches for known tool call patterns.
func walkForToolCalls(v any, out *[]NormalizedToolCall) {
	switch x := v.(type) {
	case map[string]any:
		// OpenAI: message.tool_calls: [{ type:"function", function:{ name, arguments } }]
		if tcRaw, ok := x["tool_calls"]; ok {
			if arr, ok := tcRaw.([]any); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]any); ok {
						// m["function"] may hold {name, arguments}
						if fn, ok := m["function"].(map[string]any); ok {
							if c, ok := parseFunctionCall(fn); ok {
								*out = append(*out, c)
							}
							continue
						}
						// Some providers may inline name/arguments
						if c, ok := parseFunctionCall(m); ok {
							*out = append(*out, c)
						}
					}
				}
			}
		}
		// OpenAI/Ollama: function_call: { name, arguments }
		if fc, ok := x["function_call"]; ok {
			if fn, ok := fc.(map[string]any); ok {
				if c, ok := parseFunctionCall(fn); ok {
					*out = append(*out, c)
				}
			}
		}
		// Gemini: functionCall: { name, args }
		if fc, ok := x["functionCall"]; ok {
			if fn, ok := fc.(map[string]any); ok {
				if c, ok := parseGeminiFunctionCall(fn); ok {
					*out = append(*out, c)
				}
			}
		}
		// Recurse into values to catch nested placements (e.g., choices, candidates, parts).
		for _, v := range x {
			walkForToolCalls(v, out)
		}
	case []any:
		for _, it := range x {
			walkForToolCalls(it, out)
		}
	}
}

func parseFunctionCall(m map[string]any) (NormalizedToolCall, bool) {
	name, _ := m["name"].(string)
	if strings.TrimSpace(name) == "" {
		return NormalizedToolCall{}, false
	}
	// arguments may be an object or a JSON string
	var args map[string]any
	switch a := m["arguments"].(type) {
	case string:
		var tmp map[string]any
		if err := json.Unmarshal([]byte(a), &tmp); err == nil {
			args = tmp
		}
	case map[string]any:
		args = a
	}
	return NormalizedToolCall{Name: name, Arguments: args}, true
}

func parseGeminiFunctionCall(m map[string]any) (NormalizedToolCall, bool) {
	name, _ := m["name"].(string)
	if strings.TrimSpace(name) == "" {
		return NormalizedToolCall{}, false
	}
	var args map[string]any
	if a, ok := m["args"].(map[string]any); ok {
		args = a
	}
	return NormalizedToolCall{Name: name, Arguments: args}, true
}
