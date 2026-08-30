package inference

import "testing"

func TestToolCallAssemblerConcatenatesArgumentDeltas(t *testing.T) {
	a := newToolCallAssembler()
	a.Add([]toolCallDelta{{
		Index: 0, ID: "call_1", Type: "function",
		Function: ToolCallFunction{Name: "query", Arguments: `{"ob`},
	}})
	a.Add([]toolCallDelta{{
		Index:    0,
		Function: ToolCallFunction{Arguments: `ject":"Account"}`},
	}})
	got := a.Result()
	if len(got) != 1 || got[0].ID != "call_1" || got[0].Function.Name != "query" {
		t.Fatalf("%+v", got)
	}
	args := got[0].ArgsMap()
	if args["object"] != "Account" {
		t.Fatalf("args=%v", args)
	}
}

func TestToolCallsFromEffectsIgnoresGraph(t *testing.T) {
	effects := map[string]any{
		"graphCalls": []any{map[string]any{"tool": "graph.pin", "input": map[string]any{"ref": "x"}}},
		"toolCalls": []any{
			map[string]any{"tool": "query", "input": map[string]any{"object": "Account"}},
			map[string]any{"tool": "graph.link", "input": map[string]any{}},
		},
	}
	got := ToolCallsFromEffects(effects)
	if len(got) != 1 || got[0].Function.Name != "query" {
		t.Fatalf("%+v", got)
	}
}

func TestResponseToolCalls(t *testing.T) {
	resp := &ChatResponse{}
	resp.Choices = []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}{{
		Message: Message{
			Role: "assistant",
			ToolCalls: []ToolCall{
				NewToolCall("c1", "get_record", map[string]any{"object": "Account", "id": "a1"}),
			},
		},
		FinishReason: "tool_calls",
	}}
	got := ResponseToolCalls(resp)
	if len(got) != 1 || got[0].Function.Name != "get_record" {
		t.Fatalf("%+v", got)
	}
	if TextContent(resp) != "" {
		t.Fatalf("content=%q", TextContent(resp))
	}
}

func TestNewToolCallRoundTrip(t *testing.T) {
	tc := NewToolCall("id1", "create_record", map[string]any{"object": "Account", "data": map[string]any{"Name": "Acme"}})
	args := tc.ArgsMap()
	if args["object"] != "Account" {
		t.Fatalf("%v", args)
	}
	data, _ := args["data"].(map[string]any)
	if data["Name"] != "Acme" {
		t.Fatalf("%v", data)
	}
}
