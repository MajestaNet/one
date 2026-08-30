package inference

import (
	"encoding/json"
	"strings"
)

// Known structured keys agents may emit for Control IDE run/tool bridges.
var agentEffectKeys = []string{
	"graphCalls", "graphBridgeCalls", "toolCalls", "toolBridgeCalls",
	"proposal", "proposalId", "proposedMutations",
	"boardHandoff", "handoff", "toolHandoff",
}

// BuildAgentMessages builds system + prior conversation turns + the current user goal.
// When input includes contextExcerpts / activeTool, they are serialized into the user turn
// so selection-scoped Run coaching is available to the model (BP-056).
// When input.conversation is a list of {role, content|body} turns, those are sent
// as prior user/assistant messages so the model sees the thread, not only the latest goal.
func BuildAgentMessages(instructions, goal string, input map[string]any) []Message {
	sys := instructions
	if strings.TrimSpace(sys) == "" {
		sys = "You are a helpful assistant operating inside a Majesta One customer install. Answer clearly and concisely."
	}
	if !strings.Contains(sys, "```oneEffects") {
		sys = strings.TrimRight(sys, "\n") + `

When you need the Control IDE to act on tools or the personal Run graph, end your reply with a fenced JSON block tagged oneEffects, for example:
` + "```oneEffects\n" + `{
  "summary": "Short human-readable outcome",
  "graphCalls": [{"tool": "graph.pin", "input": {"ref": {"objectApiName": "Account", "recordId": "…"}}}],
  "proposal": {"proposalId": "…", "mutations": [{"op": "update", "object": "Account", "id": "…", "data": {"Name": "…"}}], "rationale": "…"},
  "boardHandoff": {"objectApiName": "Account", "recordIds": ["…"]}
}
` + "```" + `
Omit keys you do not need. Never put record field maps or mutation ops into graph.* inputs. Prose outside the fence is fine.`
	}

	user := goal
	if user == "" {
		user = "Help the user."
	}
	var history []Message
	if input != nil {
		if extra, ok := input["message"].(string); ok && extra != "" {
			user = extra
		}
		if extra, ok := input["userMessage"].(string); ok && extra != "" {
			user = extra
		}
		var extras []string
		if excerpts := formatContextExcerpts(input["contextExcerpts"]); excerpts != "" {
			extras = append(extras, excerpts)
		}
		if tool := formatActiveTool(input["activeTool"]); tool != "" {
			extras = append(extras, tool)
		}
		if len(extras) > 0 {
			user = user + "\n\n" + strings.Join(extras, "\n\n")
		}
		history = conversationFromInput(input)
	}

	msgs := []Message{{Role: "system", Content: sys}}
	if n := len(history); n > 0 {
		last := history[n-1]
		if last.Role == "user" && last.Content == user {
			history = history[:n-1]
		}
		msgs = append(msgs, history...)
	}
	msgs = append(msgs, Message{Role: "user", Content: user})
	return msgs
}

const maxConversationMessages = 32
const maxTurnChars = 4000

func conversationFromInput(input map[string]any) []Message {
	if input == nil {
		return nil
	}
	raw, ok := input["conversation"]
	if !ok {
		return nil
	}
	list := asAnySlice(raw)
	if len(list) == 0 {
		return nil
	}
	out := make([]Message, 0, len(list))
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := normalizeConversationRole(stringifyAny(row["role"]))
		content := strings.TrimSpace(stringifyAny(row["content"]))
		if content == "" {
			content = strings.TrimSpace(stringifyAny(row["body"]))
		}
		if role == "" || content == "" {
			continue
		}
		if len(content) > maxTurnChars {
			content = content[:maxTurnChars]
		}
		out = append(out, Message{Role: role, Content: content})
	}
	if len(out) > maxConversationMessages {
		out = out[len(out)-maxConversationMessages:]
	}
	return out
}

func normalizeConversationRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user", "human":
		return "user"
	case "assistant", "agent":
		return "assistant"
	default:
		return ""
	}
}

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func asAnySlice(raw any) []any {
	switch t := raw.(type) {
	case []any:
		return t
	case []map[string]any:
		out := make([]any, len(t))
		for i, row := range t {
			out[i] = row
		}
		return out
	default:
		return nil
	}
}

func formatContextExcerpts(raw any) string {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		// JSON round-trip from typed slices may arrive as []map[string]any via map[string]any only as []any;
		// also accept []map from typed Go callers.
		if typed, ok := raw.([]map[string]any); ok && len(typed) > 0 {
			list = make([]any, len(typed))
			for i, row := range typed {
				list[i] = row
			}
		} else {
			return ""
		}
	}
	var b strings.Builder
	b.WriteString("Context excerpts (refs / labels only — do not invent field payloads):")
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label, _ := row["label"].(string)
		text, _ := row["text"].(string)
		if strings.TrimSpace(label) == "" && strings.TrimSpace(text) == "" {
			continue
		}
		if strings.TrimSpace(label) == "" {
			label = "excerpt"
		}
		b.WriteString("\n- ")
		b.WriteString(label)
		if strings.TrimSpace(text) != "" {
			b.WriteString(": ")
			b.WriteString(text)
		}
	}
	out := b.String()
	if !strings.Contains(out, "\n- ") {
		return ""
	}
	return out
}

func formatActiveTool(raw any) string {
	row, ok := raw.(map[string]any)
	if !ok || len(row) == 0 {
		return ""
	}
	b, err := json.Marshal(row)
	if err != nil {
		return ""
	}
	return "Active Tool context (JSON):\n" + string(b)
}

// ParseStructuredAgentOutput extracts a oneEffects object from model text.
// Returns human-facing summary text (fence stripped when present) and any structured effects map.
func ParseStructuredAgentOutput(text string) (summary string, effects map[string]any) {
	summary = text
	if strings.TrimSpace(text) == "" {
		return summary, nil
	}
	if obj, rest, ok := extractFencedJSON(text, "oneEffects"); ok {
		summary = strings.TrimSpace(rest)
		if summary == "" {
			if s, _ := obj["summary"].(string); strings.TrimSpace(s) != "" {
				summary = s
			}
		}
		return summary, filterEffectKeys(obj)
	}
	if obj, rest, ok := extractFencedJSON(text, "json"); ok && hasAnyEffectKey(obj) {
		summary = strings.TrimSpace(rest)
		if summary == "" {
			if s, _ := obj["summary"].(string); strings.TrimSpace(s) != "" {
				summary = s
			}
		}
		return summary, filterEffectKeys(obj)
	}
	if obj, ok := extractTrailingJSONObject(text); ok && hasAnyEffectKey(obj) {
		// Keep full text as summary when trailing JSON is effects-only.
		return summary, filterEffectKeys(obj)
	}
	return summary, nil
}

// EnrichAgentOutput merges model text into a run output map, promoting structured
// graph/proposal/tool keys so IDE bridges can execute without fabricating fixtures.
func EnrichAgentOutput(base map[string]any, text string) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	summary, effects := ParseStructuredAgentOutput(text)
	out["summary"] = summary
	for k, v := range effects {
		// Do not clobber keys the caller already set explicitly.
		if _, exists := out[k]; exists && k != "summary" {
			continue
		}
		out[k] = v
	}
	return out
}

func filterEffectKeys(obj map[string]any) map[string]any {
	if obj == nil {
		return nil
	}
	out := map[string]any{}
	for _, k := range agentEffectKeys {
		if v, ok := obj[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasAnyEffectKey(obj map[string]any) bool {
	for _, k := range agentEffectKeys {
		if _, ok := obj[k]; ok {
			return true
		}
	}
	return false
}

func extractFencedJSON(text, tag string) (map[string]any, string, bool) {
	lower := strings.ToLower(text)
	needle := "```" + strings.ToLower(tag)
	idx := strings.Index(lower, needle)
	if idx < 0 {
		return nil, text, false
	}
	start := idx + len(needle)
	// Skip optional whitespace/newline after fence tag.
	for start < len(text) && (text[start] == ' ' || text[start] == '\t' || text[start] == '\r') {
		start++
	}
	if start < len(text) && text[start] == '\n' {
		start++
	}
	endRel := strings.Index(lower[start:], "```")
	if endRel < 0 {
		return nil, text, false
	}
	end := start + endRel
	raw := strings.TrimSpace(text[start:end])
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return nil, text, false
	}
	rest := strings.TrimSpace(text[:idx] + text[end+3:])
	return obj, rest, true
}

func extractTrailingJSONObject(text string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(text)
	start := strings.LastIndex(trimmed, "{")
	if start < 0 {
		return nil, false
	}
	raw := trimmed[start:]
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}
