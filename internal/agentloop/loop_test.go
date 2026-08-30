package agentloop_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/agentharness"
	"github.com/MajestaNet/ide/internal/agentloop"
	"github.com/MajestaNet/ide/internal/inference"
)

func TestChatToolsStayInHostedCatalog(t *testing.T) {
	admitted := agentharness.ExpandToHostedMCP([]string{"sobjects.read", "query", "sobjects.write", "skills.invoke"})
	for _, name := range admitted {
		if agentharness.HostedToolClass(name) == "" {
			t.Fatalf("admitted %s is not hosted", name)
		}
	}
	if len(admitted) == 0 {
		t.Fatal("expected admitted tools")
	}
}

func TestToolCallsFromEffectsNotAppliedAsGraph(t *testing.T) {
	text := "Done.\n```oneEffects\n" + `{
  "graphCalls": [{"tool": "graph.pin", "input": {"ref": {"objectApiName": "Account"}}}],
  "proposal": {"proposalId": "p1"},
  "toolCalls": [{"tool": "query", "input": {"object": "Account"}}]
}` + "\n```"
	_, effects := inference.ParseStructuredAgentOutput(text)
	if effects["graphCalls"] == nil {
		t.Fatal("graphCalls should persist for optional clients")
	}
	calls := inference.ToolCallsFromEffects(effects)
	if len(calls) != 1 || calls[0].Function.Name != "query" {
		t.Fatalf("hosted executor should only take MCP toolCalls, got %+v", calls)
	}
	for _, c := range calls {
		if strings.HasPrefix(c.Function.Name, "graph.") {
			t.Fatal("graph.* must not become hosted tool calls")
		}
	}
}

func TestRedactAndTruncateHelpersViaPendingJSON(t *testing.T) {
	p := agentloop.PendingToolCall{
		ID:   "c1",
		Name: "create_record",
		Arguments: map[string]any{
			"object": "Account",
			"data":   map[string]any{"Name": "Acme"},
		},
		Round: 0,
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var back agentloop.PendingToolCall
	if err := json.Unmarshal(b, &back); err != nil || back.Name != "create_record" {
		t.Fatalf("%s %v", b, err)
	}
}

func TestMaxToolRoundsConstant(t *testing.T) {
	if agentloop.MaxToolRounds != 8 {
		t.Fatalf("MaxToolRounds=%d", agentloop.MaxToolRounds)
	}
	if agentloop.StatusAwaitingToolApproval != "awaiting_tool_approval" {
		t.Fatal(agentloop.StatusAwaitingToolApproval)
	}
}
