package agentharness_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/agentharness"
)

func TestApplyPrependsPreambleAndUnionsTools(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection: "build",
		HarnessID:      "harness.build.metadata",
		HarnessVersion: agentharness.CatalogVersion,
		Instructions:   "Help with custom objects.",
		AllowedTools:   []string{"sobjects.write", "unknown.tool"},
	})
	if !strings.Contains(applied.SystemInstructions, "Help with custom objects.") {
		t.Fatalf("customer instructions missing: %s", applied.SystemInstructions)
	}
	if !strings.Contains(applied.SystemInstructions, "Build") {
		t.Fatalf("preamble missing: %s", applied.SystemInstructions)
	}
	if applied.HarnessID != "harness.operate.query" || applied.PrimarySection != "build" {
		t.Fatalf("binding=%+v", applied)
	}
	wantTools := []string{"sobjects.read", "query", "sobjects.write"}
	if len(applied.AllowedTools) != len(wantTools) {
		t.Fatalf("tools=%v", applied.AllowedTools)
	}
	for i, w := range wantTools {
		if applied.AllowedTools[i] != w {
			t.Fatalf("tools=%v", applied.AllowedTools)
		}
	}
	if !applied.RequireApproval {
		t.Fatal("expect approval floor")
	}
	if applied.VersionMismatch {
		t.Fatal("unexpected version mismatch")
	}
}

func TestApplyKeepsSearchTool(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection: "operate",
		AllowedTools:   []string{"search"},
	})
	found := false
	for _, tool := range applied.AllowedTools {
		if tool == "search" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("search dropped from tools=%v", applied.AllowedTools)
	}
}

func TestApplyVersionMismatchWarns(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection: "operate",
		HarnessVersion: "0",
		Instructions:   "x",
	})
	if !applied.VersionMismatch {
		t.Fatal("expected version mismatch")
	}
	if len(applied.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
	// Floor still from current catalog.
	if applied.HarnessVersion != agentharness.CatalogVersion {
		t.Fatalf("harnessVersion=%s", applied.HarnessVersion)
	}
}

func TestApplyDefaultsMissingSection(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{Instructions: "hi"})
	if applied.PrimarySection != "operate" || applied.HarnessID != "harness.run.tools" {
		t.Fatalf("got %+v", applied)
	}
	if len(applied.Warnings) == 0 {
		t.Fatal("expected missing-section warning")
	}
}

func TestComposeSystemInstructions(t *testing.T) {
	got := agentharness.ComposeSystemInstructions("PRE", "BODY")
	if got != "PRE\n\nBODY" {
		t.Fatalf("%q", got)
	}
}

func TestApplyJobClassFloorCannotBeDropped(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		JobClass:     "query",
		AllowedTools: []string{"sobjects.write"},
	})
	if applied.JobClass != "query" || applied.HarnessID != "harness.query.read" {
		t.Fatalf("binding=%+v", applied)
	}
	if applied.PrimarySection != "operate" {
		t.Fatalf("filled primarySection=%s", applied.PrimarySection)
	}
	want := []string{"sobjects.read", "query", "search", "sobjects.write"}
	if len(applied.AllowedTools) != len(want) {
		t.Fatalf("tools=%v want %v", applied.AllowedTools, want)
	}
	for i, w := range want {
		if applied.AllowedTools[i] != w {
			t.Fatalf("tools=%v want %v", applied.AllowedTools, want)
		}
	}
	if !applied.RequireApproval {
		t.Fatal("expect approval floor")
	}
}

func TestApplyUsesSectionCatalogWhenJobClassUnset(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		PrimarySection: "build",
		AllowedTools:   []string{"sobjects.write"},
	})
	if applied.HarnessID != "harness.operate.query" || applied.PrimarySection != "build" {
		t.Fatalf("section catalog path: %+v", applied)
	}
	if applied.JobClass != "" {
		t.Fatalf("unset jobClass must keep section catalog (no invented jobClass): %+v", applied)
	}
}

func TestApplySettingsKeepsSettingsFloor(t *testing.T) {
	applied := agentharness.Apply(agentharness.Spec{
		JobClass:       "govern",
		PrimarySection: "settings",
		AllowedTools:   []string{"sobjects.write"},
	})
	if applied.HarnessID != "harness.settings.install" || applied.PrimarySection != "settings" {
		t.Fatalf("settings floor: %+v", applied)
	}
	if applied.JobClass != "govern" {
		t.Fatalf("jobClass=%s", applied.JobClass)
	}
}
