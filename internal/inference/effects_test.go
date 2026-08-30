package inference

import "testing"

func TestBuildAgentMessagesIncludesConversationHistory(t *testing.T) {
	msgs := BuildAgentMessages("Be brief.", "What about Acme?", map[string]any{
		"conversation": []any{
			map[string]any{"role": "human", "content": "Find Acme"},
			map[string]any{"role": "assistant", "content": "Acme is an Account."},
		},
	})
	if len(msgs) != 4 {
		t.Fatalf("want system + 2 history + current user, got %d %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "Find Acme" {
		t.Fatalf("history user: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Content != "Acme is an Account." {
		t.Fatalf("history assistant: %+v", msgs[2])
	}
	if msgs[3].Role != "user" || msgs[3].Content != "What about Acme?" {
		t.Fatalf("current user: %+v", msgs[3])
	}
}

func TestBuildAgentMessagesDropsDuplicateTrailingUser(t *testing.T) {
	msgs := BuildAgentMessages("Be brief.", "Find Acme", map[string]any{
		"conversation": []any{
			map[string]any{"role": "user", "content": "Find Acme"},
		},
	})
	if len(msgs) != 2 {
		t.Fatalf("duplicate current user should be dropped: %+v", msgs)
	}
	if msgs[1].Content != "Find Acme" {
		t.Fatalf("user: %+v", msgs[1])
	}
}

func TestBuildAgentMessagesIncludesContextExcerpts(t *testing.T) {
	msgs := BuildAgentMessages("Be brief.", "Pin selection", map[string]any{
		"userMessage": "Pin these",
		"contextExcerpts": []any{
			map[string]any{"label": "Graph selection", "text": "Account/a-1"},
		},
		"activeTool": map[string]any{"toolId": "tool-1", "title": "Pipeline"},
	})
	if len(msgs) != 2 {
		t.Fatalf("msgs: %+v", msgs)
	}
	if !containsAll(msgs[0].Content, "oneEffects", "graphCalls") {
		t.Fatalf("system should instruct oneEffects envelope: %q", msgs[0].Content)
	}
	if !containsAll(msgs[1].Content, "Pin these", "Graph selection", "Account/a-1", "Active Tool context") {
		t.Fatalf("user missing excerpts/tool: %q", msgs[1].Content)
	}
}

func TestParseStructuredAgentOutputOneFence(t *testing.T) {
	text := "Pinned the account.\n\n```oneEffects\n" + `{
  "graphCalls": [{"tool": "graph.pin", "input": {"ref": {"objectApiName": "Account", "recordId": "a1"}}}],
  "proposal": {"proposalId": "p1", "mutations": [{"op": "update", "object": "Account", "id": "a1", "data": {"Name": "Acme"}}]}
}` + "\n```\n"
	summary, effects := ParseStructuredAgentOutput(text)
	if summary != "Pinned the account." {
		t.Fatalf("summary=%q", summary)
	}
	if effects == nil {
		t.Fatal("expected effects")
	}
	calls, ok := effects["graphCalls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("graphCalls=%v", effects["graphCalls"])
	}
	if _, ok := effects["proposal"]; !ok {
		t.Fatalf("proposal missing: %+v", effects)
	}
}

func TestEnrichAgentOutputPromotesKeys(t *testing.T) {
	out := EnrichAgentOutput(map[string]any{
		"goal":         "pin",
		"toolsPlanned": []string{"query"},
	}, "Done.\n```json\n"+`{"graphCalls":[{"tool":"graph.link","input":{"from":"a","to":"b","kind":"next"}}]}`+"\n```")
	if out["summary"] != "Done." {
		t.Fatalf("summary=%v", out["summary"])
	}
	if out["goal"] != "pin" {
		t.Fatalf("goal clobbered")
	}
	calls, ok := out["graphCalls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("graphCalls=%v", out["graphCalls"])
	}
}

func TestEnrichAgentOutputDoesNotClobberExisting(t *testing.T) {
	out := EnrichAgentOutput(map[string]any{
		"summary":    "preset",
		"graphCalls": []any{map[string]any{"tool": "graph.get"}},
	}, "```oneEffects\n"+`{"graphCalls":[{"tool":"graph.pin"}]}`+"\n```")
	calls := out["graphCalls"].([]any)
	row := calls[0].(map[string]any)
	if row["tool"] != "graph.get" {
		t.Fatalf("should keep preset graphCalls, got %+v", row)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
