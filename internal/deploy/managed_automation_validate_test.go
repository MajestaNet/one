package deploy_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestValidateRejectsManagedAutomation(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	src := `export default async function run(ctx) { return { ok: true }; }`
	pkg := "lead_marketing"
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: deploy.DefaultCustomerPackage,
		Automations: []deploy.SnapshotAutomation{{
			APIName:       "Lead_ConvertOnConvertedStatus",
			Label:         "Convert Lead on Converted status",
			ObjectAPIName: "Lead",
			TriggerEvent:  "update",
			Active:        true,
			Runtime:       "code",
			Execution:     "sync",
			Source:        &src,
			Ownership:     "managed",
			PackageName:   &pkg,
		}},
	}
	report, err := deploy.ValidateBundleArtifact(ctx, d.Meta, art, "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("validate must reject managed automations in a customer pack")
	}
	found := false
	for _, iss := range report.Issues {
		if iss.Code == "MANAGED_IN_BUNDLE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MANAGED_IN_BUNDLE, issues=%+v", report.Issues)
	}
}

func TestApplySkipsExistingManagedAutomation(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	const autoName = "Lead_ConvertOnConvertedStatus"
	var wantSrc, wantOwn string
	if err := d.Pool.QueryRow(ctx, `
SELECT COALESCE(source,''), COALESCE(ownership,'') FROM metadata_automations WHERE api_name=$1`, autoName).
		Scan(&wantSrc, &wantOwn); err != nil {
		t.Fatal(err)
	}
	if wantOwn != "managed" || wantSrc == "" {
		t.Fatalf("precondition ownership=%s srcLen=%d", wantOwn, len(wantSrc))
	}

	hijack := "export default async function run(ctx) { return { hijacked: true }; }"
	cust := deploy.DefaultCustomerPackage
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: cust,
		Automations: []deploy.SnapshotAutomation{{
			APIName:       autoName,
			Label:         "hijack",
			ObjectAPIName: "Lead",
			TriggerEvent:  "update",
			Active:        true,
			Runtime:       "code",
			Execution:     "sync",
			Source:        &hijack,
			Ownership:     "custom",
			PackageName:   &cust,
		}},
	}
	report, err := deploy.ApplyBundleArtifact(ctx, d.Pool, d.Meta, art, false)
	if err != nil {
		t.Fatal(err)
	}
	skipped := false
	for _, a := range report.Actions {
		if a.Kind == "automation" && a.APIName == autoName && a.Action == "skipped" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected skip, actions=%+v", report.Actions)
	}
	var gotSrc, gotOwn string
	if err := d.Pool.QueryRow(ctx, `
SELECT COALESCE(source,''), COALESCE(ownership,'') FROM metadata_automations WHERE api_name=$1`, autoName).
		Scan(&gotSrc, &gotOwn); err != nil {
		t.Fatal(err)
	}
	if gotOwn != "managed" || gotSrc != wantSrc {
		t.Fatalf("managed row mutated ownership=%s srcChanged=%v", gotOwn, gotSrc != wantSrc)
	}
}

func TestExportSnapshotOmitsManagedAutomations(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatal(err)
	}
	snap, err := d.Meta.ExportCustomerSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := snap["automations"].([]any)
	if raw == nil {
		if ms, ok := snap["automations"].([]map[string]any); ok {
			for _, a := range ms {
				if a["apiName"] == "Lead_ConvertOnConvertedStatus" {
					t.Fatal("snapshot must omit managed automations")
				}
			}
			return
		}
	}
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if m["apiName"] == "Lead_ConvertOnConvertedStatus" {
			t.Fatal("snapshot must omit managed automations")
		}
		if m["ownership"] == "managed" {
			t.Fatalf("snapshot included managed automation %#v", m)
		}
	}
}
