package seed_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestRunCoachTemplateExplainsCuratorDoerAndPublisherRoles(t *testing.T) {
	m, ok := packages.Get("agents_starter")
	if !ok {
		t.Fatal("agents_starter module missing")
	}
	for _, template := range m.AgentSpecTemplates {
		if template.APIName != "RunCoach" {
			continue
		}
		for _, want := range []string{"Curator", "Doer", "human Apply", "graph.publishSubgraph", "org ToolSpec", "ignores graphCalls"} {
			if !strings.Contains(template.Instructions, want) {
				t.Fatalf("RunCoach instructions missing %q: %s", want, template.Instructions)
			}
		}
		return
	}
	t.Fatal("RunCoach template missing")
}

func TestStarterInstructionsUseInstallNotIDEHost(t *testing.T) {
	m, ok := packages.Get("agents_starter")
	if !ok {
		t.Fatal("agents_starter module missing")
	}
	seen := map[string]bool{}
	for _, template := range m.AgentSpecTemplates {
		seen[template.APIName] = true
		if template.JobClass == "" {
			t.Fatalf("%s missing jobClass", template.APIName)
		}
		switch template.APIName {
		case "ShipGuide":
			if !strings.Contains(template.Instructions, "one org validate") {
				t.Fatalf("ShipGuide must document CLI ship: %s", template.Instructions)
			}
			if strings.Contains(template.Instructions, "Ship panels") {
				t.Fatalf("ShipGuide must not send builders to IDE Ship panels: %s", template.Instructions)
			}
		case "AccountGuide":
			if strings.Contains(template.Instructions, "Control IDE Account") {
				t.Fatalf("AccountGuide must not bind to Control IDE chrome: %s", template.Instructions)
			}
		}
	}
	for _, name := range []string{"AdminSetup", "MetadataBuilder", "RunCoach", "ShipGuide", "AccountGuide"} {
		if !seen[name] {
			t.Fatalf("missing starter %s", name)
		}
	}
}

func TestAgentsStarterAlwaysOnCloneIdempotent(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	meta := metadata.NewService(pool)
	_, _ = pool.Exec(ctx, `DELETE FROM agent_playbooks WHERE api_name IN ('AdminSetup','MetadataBuilder')`)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_playbooks WHERE api_name IN ('AdminSetup','MetadataBuilder')`)
	})

	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID: "00000000-0000-4000-8000-000000000001", AutoSeed: true, FeatureFlags: []string{"agents"},
	}); err != nil {
		t.Fatal(err)
	}

	m, ok := packages.Get("agents_starter")
	if !ok || m.Optional {
		t.Fatalf("agents_starter must be always-on: %+v ok=%v", m, ok)
	}
	if len(m.AgentSpecTemplates) < 2 {
		t.Fatalf("templates missing: %+v", m)
	}

	ver, enabled, err := meta.GetPackageInstall(ctx, "agents_starter")
	if err != nil || !enabled || ver != seed.AgentsStarterPackageVersion {
		t.Fatalf("agents_starter install ver=%q enabled=%v err=%v", ver, enabled, err)
	}

	var ownership, instructions string
	err = pool.QueryRow(ctx, `SELECT ownership, instructions FROM agent_playbooks WHERE api_name='AdminSetup'`).
		Scan(&ownership, &instructions)
	if err != nil {
		t.Fatal(err)
	}
	if ownership != "custom" {
		t.Fatalf("ownership=%s", ownership)
	}
	if instructions == "" {
		t.Fatal("expected instructions")
	}
	_, _ = pool.Exec(ctx, `UPDATE agent_playbooks SET instructions='customer-edited' WHERE api_name='AdminSetup'`)

	// Re-install must not overwrite customer edits.
	if err := seed.InstallAgentsStarter(ctx, meta); err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `SELECT instructions FROM agent_playbooks WHERE api_name='AdminSetup'`).Scan(&instructions)
	if err != nil {
		t.Fatal(err)
	}
	if instructions != "customer-edited" {
		t.Fatalf("clone overwrote customer edits: %q", instructions)
	}

	if _, err := seed.EnablePackage(ctx, meta, "agents_starter"); err == nil {
		t.Fatal("agents_starter must not be optionally enableable")
	}
}
