package deploy_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestValidateLookupTargetMissingWithoutPackage(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	pkg := deploy.DefaultCustomerPackage
	obj := "VisitLookupGate__c"

	lookupArt := func(target string) *deploy.BundleArtifact {
		ref := target
		return &deploy.BundleArtifact{
			ManifestVersion:    1,
			Ownership:          "custom",
			DefaultPackageName: pkg,
			Objects: []deploy.SnapshotObject{{
				APIName:     obj,
				Label:       "Visit Lookup Gate",
				PluralLabel: "Visit Lookup Gates",
				StorageMode: "flexible",
				Ownership:   "custom",
				PackageName: &pkg,
				Features:    map[string]bool{},
			}},
			Fields: []deploy.SnapshotField{{
				ObjectAPIName: obj,
				APIName:       "OpportunityId",
				Label:         "Opportunity",
				FieldType:     "lookup",
				ReferenceTo:   &ref,
				Ownership:     "custom",
				PackageName:   &pkg,
			}},
		}
	}

	missingTarget := "NoSuchManagedObject__c"
	if _, err := d.Meta.GetObject(ctx, "Opportunity"); err != nil {
		missingTarget = "Opportunity"
	}

	report, err := deploy.ValidateBundleArtifact(ctx, d.Meta, lookupArt(missingTarget), "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("validate must fail when lookup target is not on the install")
	}
	found := false
	for _, iss := range report.Issues {
		if iss.Code == "LOOKUP_TARGET_MISSING" && strings.Contains(iss.Message, missingTarget) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected LOOKUP_TARGET_MISSING naming %s, issues=%+v", missingTarget, report.Issues)
	}

	if _, err := seed.EnablePackage(ctx, d.Meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "sales"); err != nil {
		t.Fatalf("enable sales: %v", err)
	}
	okReport, err := deploy.ValidateBundleArtifact(ctx, d.Meta, lookupArt("Opportunity"), "0.1.0", "*")
	if err != nil {
		t.Fatal(err)
	}
	if !okReport.OK {
		t.Fatalf("lookup to Opportunity must pass after sales is enabled, issues=%+v", okReport.Issues)
	}
}
