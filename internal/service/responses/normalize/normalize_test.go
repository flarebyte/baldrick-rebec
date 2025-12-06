package normalize

import (
    "encoding/json"
    "reflect"
    "testing"
)

func TestNormalizeTools(t *testing.T) {
    in := []ToolDefinition{
        {Type: "function", Function: struct {
            Name       string                 `json:"name"`
            Parameters map[string]any        `json:"parameters"`
        }{Name: "sum", Parameters: map[string]any{"type": "object"}}},
        {Type: "ignored"},
        {Type: "function", Function: struct {
            Name       string                 `json:"name"`
            Parameters map[string]any        `json:"parameters"`
        }{Name: "", Parameters: map[string]any{"type": "object"}}},
    }
    out := NormalizeTools(in)
    if len(out) != 1 {
        t.Fatalf("expected 1 tool, got %d", len(out))
    }
    if out[0].Name != "sum" {
        t.Fatalf("expected tool name 'sum', got %q", out[0].Name)
    }
}

func TestNormalizeToolCalls_OpenAI(t *testing.T) {
    // Simulate OpenAI message structure with tool_calls
    raw := map[string]any{
        "choices": []any{
            map[string]any{
                "message": map[string]any{
                    "tool_calls": []any{
                        map[string]any{
                            "type": "function",
                            "function": map[string]any{
                                "name":      "getWeather",
                                "arguments": "{\n  \"city\": \"Paris\"\n}",
                            },
                        },
                    },
                },
            },
        },
    }
    calls := NormalizeToolCalls(raw)
    if len(calls) != 1 {
        t.Fatalf("expected 1 call, got %d", len(calls))
    }
    if calls[0].Name != "getWeather" {
        t.Fatalf("unexpected call name: %q", calls[0].Name)
    }
    if calls[0].Arguments["city"] != "Paris" {
        t.Fatalf("unexpected args: %#v", calls[0].Arguments)
    }
}

func TestNormalizeToolCalls_Gemini(t *testing.T) {
    // Simulate Gemini parts with functionCall
    raw := map[string]any{
        "candidates": []any{
            map[string]any{
                "content": map[string]any{
                    "parts": []any{
                        map[string]any{
                            "functionCall": map[string]any{
                                "name": "search",
                                "args": map[string]any{"q": "golang"},
                            },
                        },
                    },
                },
            },
        },
    }
    calls := NormalizeToolCalls(raw)
    if len(calls) != 1 {
        t.Fatalf("expected 1 call, got %d", len(calls))
    }
    if calls[0].Name != "search" || calls[0].Arguments["q"] != "golang" {
        t.Fatalf("unexpected call: %#v", calls[0])
    }
}

func TestNormalizeToolCalls_Ollama(t *testing.T) {
    // Simulate function_call present directly
    raw := map[string]any{
        "message": map[string]any{
            "function_call": map[string]any{
                "name":      "translate",
                "arguments": map[string]any{"text": "hi", "to": "fr"},
            },
        },
    }
    calls := NormalizeToolCalls(raw)
    if len(calls) != 1 {
        t.Fatalf("expected 1 call, got %d", len(calls))
    }
    wantArgs := map[string]any{"text": "hi", "to": "fr"}
    if calls[0].Name != "translate" || !reflect.DeepEqual(calls[0].Arguments, wantArgs) {
        t.Fatalf("unexpected call: %#v", calls[0])
    }
}

func TestBuildToolCallBlocks(t *testing.T) {
    calls := []NormalizedToolCall{{Name: "foo", Arguments: map[string]any{"a": 1}}}
    blocks := BuildToolCallBlocks(calls)
    if len(blocks) != 1 || blocks[0].Type != "tool_call" || blocks[0].ToolCall.Name != "foo" {
        t.Fatalf("unexpected blocks: %#v", blocks)
    }
    // Ensure JSON marshals to the expected envelope
    b, err := json.Marshal(blocks[0])
    if err != nil {
        t.Fatal(err)
    }
    var m map[string]any
    if err := json.Unmarshal(b, &m); err != nil {
        t.Fatal(err)
    }
    if m["type"] != "tool_call" {
        t.Fatalf("bad type: %v", m["type"])
    }
}

