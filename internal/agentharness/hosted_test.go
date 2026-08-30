package agentharness_test

import (
	"slices"
	"testing"

	"github.com/MajestaNet/ide/internal/agentharness"
)

func TestExpandToHostedMCPReadFloor(t *testing.T) {
	got := agentharness.ExpandToHostedMCP([]string{"sobjects.read", "query", "unknown.tool", "create_record"})
	want := []string{"describe_global", "describe_object", "get_record", "query"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if slices.Contains(got, "create_record") {
		t.Fatal("MCP write names must not sneak in without sobjects.write")
	}
	if slices.Contains(got, "get_object_metadata") || slices.Contains(got, "org_deploy") {
		t.Fatal("hosted v1 catalog must exclude Metadata/Deploy")
	}
}

func TestExpandToHostedMCPWritesAndInvoke(t *testing.T) {
	got := agentharness.ExpandToHostedMCP([]string{"sobjects.write", "skills.invoke", "actions.invoke", "search"})
	want := []string{"create_record", "update_record", "invoke_skill", "invoke_action", "search"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExpandToHostedMCPDedupeAndQueryToken(t *testing.T) {
	got := agentharness.ExpandToHostedMCP([]string{"query", "query", "sobjects.read"})
	n := 0
	for _, name := range got {
		if name == "query" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("query should appear once: %v", got)
	}
	if !slices.Contains(got, "get_record") {
		t.Fatalf("sobjects.read should expand get_record: %v", got)
	}
}

func TestHostedToolClass(t *testing.T) {
	if agentharness.HostedToolClass("query") != agentharness.ToolClassRead {
		t.Fatal("query is read")
	}
	if agentharness.HostedToolClass("create_record") != agentharness.ToolClassWrite {
		t.Fatal("create_record is write")
	}
	if agentharness.HostedToolClass("org_deploy") != "" {
		t.Fatal("deploy is out of hosted v1")
	}
	if agentharness.HostedToolClass("graph.pin") != "" {
		t.Fatal("graph.* is out of hosted v1")
	}
}

func TestApplyThenExpandJobClassQuery(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{JobClass: "query"})
	got := agentharness.ExpandToHostedMCP(applied.AllowedTools)
	for _, name := range []string{"describe_global", "get_record", "query", "search"} {
		if !slices.Contains(got, name) {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
	if slices.Contains(got, "create_record") {
		t.Fatalf("query floor must not admit writes: %v", got)
	}
}

func TestHostedToolAdmitted(t *testing.T) {
	admitted := []string{"query", "get_record"}
	if !agentharness.HostedToolAdmitted("query", admitted) {
		t.Fatal("expected admitted")
	}
	if agentharness.HostedToolAdmitted("create_record", admitted) {
		t.Fatal("write must not be admitted")
	}
}
