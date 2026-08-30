package customerrepo_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/customerrepo"
)

func TestPackCodeAutomationRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: customer.default
repoFormat: one/v1
`)
	mustWrite(t, filepath.Join(root, "metadata", "automations", "CreateOpp_On_Account.yaml"), `
apiName: CreateOpp_On_Account
label: Create Opp On Account
objectApiName: Account
triggerEvent: create
active: true
runtime: code
execution: async
entryFile: src/automations/create_opp_on_account.ts
ownership: custom
actions: []
`)
	mustWrite(t, filepath.Join(root, "src", "automations", "create_opp_on_account.ts"), `
import type { AutomationContext } from "one:automation";

export default async function run(ctx: AutomationContext) {
  return { ok: true };
}
`)
	mustWrite(t, filepath.Join(root, "tests", "automations", "create_opp_on_account_test.ts"), `
export default async function run(ctx) {
  return { ok: true };
}
`)

	art, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if len(art.Automations) != 1 {
		t.Fatalf("automations=%d", len(art.Automations))
	}
	a := art.Automations[0]
	if a.Runtime != "code" || a.Execution != "async" {
		t.Fatalf("runtime/execution=%s/%s", a.Runtime, a.Execution)
	}
	if a.EntryFile == nil || *a.EntryFile != "src/automations/create_opp_on_account.ts" {
		t.Fatalf("entryFile=%v", a.EntryFile)
	}
	if a.Source == nil || !strings.Contains(*a.Source, "return { ok: true }") {
		t.Fatalf("expected embedded source, got %v", a.Source)
	}
	if art.Sources["src/automations/create_opp_on_account.ts"] == "" {
		t.Fatal("missing sources map entry")
	}
	if art.Sources["tests/automations/create_opp_on_account_test.ts"] == "" {
		t.Fatal("missing test source")
	}

	out := t.TempDir()
	if err := customerrepo.UnpackToDir(out, art, customerrepo.Manifest{
		CustomerID: "acme", PackageName: "customer.default", RepoFormat: customerrepo.RepoFormat,
	}); err != nil {
		t.Fatalf("unpack: %v", err)
	}
	art2, _, err := customerrepo.PackFromDir(out, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("repack: %v", err)
	}
	if len(art2.Automations) != 1 || art2.Sources["src/automations/create_opp_on_account.ts"] == "" {
		t.Fatalf("repack lost automation sources: autos=%d sources=%v", len(art2.Automations), art2.Sources)
	}
}

func TestPackRejectsNpmImport(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: customer.default
repoFormat: one/v1
`)
	mustWrite(t, filepath.Join(root, "metadata", "automations", "BadAuto.yaml"), `
apiName: BadAuto
label: Bad
objectApiName: Account
triggerEvent: create
active: true
runtime: code
execution: async
entryFile: src/automations/bad.ts
ownership: custom
actions: []
`)
	mustWrite(t, filepath.Join(root, "src", "automations", "bad.ts"), `
import _ from "npm:lodash";
export default async function run(ctx) { return { ok: true }; }
`)
	_, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err == nil {
		t.Fatal("expected npm import to fail pack")
	}
	if !strings.Contains(err.Error(), "npm:lodash") && !strings.Contains(err.Error(), "forbidden import") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackRejectsSyncOutboundAction(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: customer.default
repoFormat: one/v1
`)
	mustWrite(t, filepath.Join(root, "metadata", "automations", "SyncHTTP.yaml"), `
apiName: SyncHTTP
label: Sync HTTP
objectApiName: Account
triggerEvent: create
active: true
runtime: actions
execution: sync
ownership: custom
actions:
  - type: http
    url: https://example.com
`)
	_, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("expected sync outbound rejection, got %v", err)
	}
}
