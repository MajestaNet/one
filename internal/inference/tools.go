package inference

import (
	"encoding/json"
	"fmt"
	"strings"
)

type toolCallDelta struct {
	Index    int              `json:"index"`
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type toolCallAssembler struct {
	byIndex map[int]*ToolCall
	order   []int
}

func newToolCallAssembler() *toolCallAssembler {
	return &toolCallAssembler{byIndex: map[int]*ToolCall{}}
}

func (a *toolCallAssembler) Add(deltas []toolCallDelta) {
	if a == nil {
		return
	}
	for _, d := range deltas {
		tc, ok := a.byIndex[d.Index]
		if !ok {
			tc = &ToolCall{Type: "function"}
			a.byIndex[d.Index] = tc
			a.order = append(a.order, d.Index)
		}
		if d.ID != "" {
			tc.ID = d.ID
		}
		if d.Type != "" {
			tc.Type = d.Type
		}
		if d.Function.Name != "" {
			tc.Function.Name = d.Function.Name
		}
		if d.Function.Arguments != "" {
			tc.Function.Arguments += d.Function.Arguments
		}
	}
}

func (a *toolCallAssembler) Result() []ToolCall {
	if a == nil || len(a.order) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(a.order))
	for _, idx := range a.order {
		tc := a.byIndex[idx]
		if tc == nil || strings.TrimSpace(tc.Function.Name) == "" {
			continue
		}
		if tc.Type == "" {
			tc.Type = "function"
		}
		out = append(out, *tc)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NewToolCall builds a function tool call with JSON arguments.
func NewToolCall(id, name string, args map[string]any) ToolCall {
	if args == nil {
		args = map[string]any{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		raw = []byte("{}")
	}
	return ToolCall{
		ID:   id,
		Type: "function",
		Function: ToolCallFunction{
			Name:      name,
			Arguments: string(raw),
		},
	}
}

// ArgsMap decodes function.arguments JSON into a map.
func (tc ToolCall) ArgsMap() map[string]any {
	raw := strings.TrimSpace(tc.Function.Arguments)
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// ToolCallsFromEffects extracts MCP-named toolCalls from a oneEffects object.
// graphCalls / proposal / boardHandoff are ignored (Control IDE Apply only).
func ToolCallsFromEffects(effects map[string]any) []ToolCall {
	if effects == nil {
		return nil
	}
	raw := effects["toolCalls"]
	if raw == nil {
		raw = effects["toolBridgeCalls"]
	}
	list := asAnySlice(raw)
	if len(list) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(list))
	for i, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringifyAny(row["tool"]))
		if name == "" {
			name = strings.TrimSpace(stringifyAny(row["name"]))
		}
		if name == "" || strings.HasPrefix(name, "graph.") {
			continue
		}
		input, _ := row["input"].(map[string]any)
		if input == nil {
			input, _ = row["arguments"].(map[string]any)
		}
		id := fmt.Sprintf("fence-%d", i+1)
		out = append(out, NewToolCall(id, name, input))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// CollectStreamToolCalls returns the last assembled tool_calls from a stream.
func CollectStreamToolCalls(chunks []StreamChunk) []ToolCall {
	var latest []ToolCall
	for _, c := range chunks {
		if len(c.ToolCalls) > 0 {
			latest = c.ToolCalls
		}
	}
	return latest
}
