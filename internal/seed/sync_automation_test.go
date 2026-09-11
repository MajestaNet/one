package seed_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/packages"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestSyncAutomationManagedPreservesActive(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := context.Background()

	orig, ok := packages.Get("notes")
	if !ok {
		t.Fatal("notes module missing")
	}
	const autoName = "Notes_FixtureNoopOnCreate"
	srcV1 := `export default async function run(ctx) { return { ok: true, v: 1 }; }`
	srcV2 := `export default async function run(ctx) { return { ok: true, v: 2 }; }`
	descV1 := "Fixture v1 process wrap."
	descV2 := "Fixture v2 refreshed description."

	withAutos := orig
	withAutos.Automations = []packages.AutomationDef{{
		APIName:       autoName,
		Label:         "Fixture Noop",
		Description:   descV1,
		ObjectAPIName: "Note",
		TriggerEvent:  "create",
		Runtime:       "code",
		Execution:     "async",
		EntryFile:     "seed/automations/Notes_FixtureNoopOnCreate.ts",
		Source:        srcV1,
	}}
	packages.Register(withAutos)
	t.Cleanup(func() {
		packages.Register(orig)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM automation_permissions WHERE automation_api_name=$1`, autoName)
		_, _ = d.Pool.Exec(context.Background(), `DELETE FROM metadata_automations WHERE api_name=$1`, autoName)
		_, _ = seed.DisablePackage(context.Background(), d.Meta, "notes")
	})

	if _, err := seed.EnablePackage(ctx, d.Meta, "notes"); err != nil {
		t.Fatalf("enable: %v", err)
	}

	var active bool
	var desc, source, ownership string
	err := d.Pool.QueryRow(ctx, `
SELECT active, description, COALESCE(source,''), ownership
FROM metadata_automations WHERE api_name=$1`, autoName).Scan(&active, &desc, &source, &ownership)
	if err != nil {
		t.Fatal(err)
	}
	if !active || ownership != "managed" || desc != descV1 || source != srcV1 {
		t.Fatalf("insert: active=%v ownership=%s desc=%q source=%q", active, ownership, desc, source)
	}

	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE api_name=$1`, autoName); err != nil {
		t.Fatal(err)
	}

	withAutos.Version = orig.Version
	withAutos.Automations = []packages.AutomationDef{{
		APIName:       autoName,
		Label:         "Fixture Noop v2",
		Description:   descV2,
		ObjectAPIName: "Note",
		TriggerEvent:  "create",
		Runtime:       "code",
		Execution:     "async",
		EntryFile:     "seed/automations/Notes_FixtureNoopOnCreate.ts",
		Source:        srcV2,
	}}
	packages.Register(withAutos)
	if _, err := seed.EnablePackage(ctx, d.Meta, "notes"); err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	var label string
	err = d.Pool.QueryRow(ctx, `
SELECT active, description, COALESCE(source,''), label
FROM metadata_automations WHERE api_name=$1`, autoName).Scan(&active, &desc, &source, &label)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected active=false to survive re-sync")
	}
	if desc != descV2 || source != srcV2 || label != "Fixture Noop v2" {
		t.Fatalf("refresh: desc=%q source=%q label=%q", desc, source, label)
	}

	packages.Register(orig)
	if _, err := seed.EnablePackage(ctx, d.Meta, "notes"); err != nil {
		t.Fatalf("enable without def: %v", err)
	}
	err = d.Pool.QueryRow(ctx, `
SELECT active FROM metadata_automations WHERE api_name=$1`, autoName).Scan(&active)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("registry-removed automation should be deactivated")
	}
}
