package deploy

import "testing"

func TestCompareArtifactsAddChangeRemove(t *testing.T) {
	pkg := DefaultCustomerPackage
	local := &BundleArtifact{
		Objects: []SnapshotObject{
			{APIName: "Account__c", Label: "Account Local", Ownership: "custom", PackageName: &pkg},
			{APIName: "OnlyLocal__c", Label: "Only Local", Ownership: "custom", PackageName: &pkg},
		},
		Fields: []SnapshotField{
			{ObjectAPIName: "Account__c", APIName: "Name", Label: "Name", Ownership: "custom"},
		},
	}
	install := &BundleArtifact{
		Objects: []SnapshotObject{
			{APIName: "Account__c", Label: "Account Install", Ownership: "custom", PackageName: &pkg},
			{APIName: "OnlyInstall__c", Label: "Only Install", Ownership: "custom", PackageName: &pkg},
		},
		Fields: []SnapshotField{
			{ObjectAPIName: "Account__c", APIName: "Name", Label: "Name", Ownership: "custom"},
		},
	}
	diff := CompareArtifacts(local, install)
	if diff.Counts.Add != 1 || diff.Counts.Change != 1 || diff.Counts.Remove != 1 {
		t.Fatalf("counts=%+v entries=%+v", diff.Counts, diff.Entries)
	}
	kinds := map[string]string{}
	for _, e := range diff.Entries {
		kinds[e.Path] = e.Kind
	}
	if kinds["objects.OnlyLocal__c"] != DiffAdd {
		t.Fatalf("expected add OnlyLocal, got %v", kinds)
	}
	if kinds["objects.Account__c"] != DiffChange {
		t.Fatalf("expected change Account, got %v", kinds)
	}
	if kinds["objects.OnlyInstall__c"] != DiffRemove {
		t.Fatalf("expected remove OnlyInstall, got %v", kinds)
	}
}

func TestCompareArtifactsIgnoresVolatileID(t *testing.T) {
	id1 := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	id2 := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	local := &BundleArtifact{
		Objects: []SnapshotObject{{APIName: "X__c", Label: "X", Ownership: "custom", ID: &id1}},
	}
	install := &BundleArtifact{
		Objects: []SnapshotObject{{APIName: "X__c", Label: "X", Ownership: "custom", ID: &id2}},
	}
	diff := CompareArtifacts(local, install)
	if diff.Counts.Change != 0 || len(diff.Entries) != 0 {
		t.Fatalf("expected no diff for id-only change, got %+v", diff)
	}
}

func TestCompareBaselineInformational(t *testing.T) {
	local := &BundleArtifact{
		Baseline: &ManagedBaseline{
			Objects: []SnapshotObject{{APIName: "Account", Label: "Account v2"}},
		},
	}
	install := &BundleArtifact{
		Baseline: &ManagedBaseline{
			Objects: []SnapshotObject{{APIName: "Account", Label: "Account v1"}},
		},
	}
	diff := CompareArtifacts(local, install)
	if diff.Counts.Baseline != 1 {
		t.Fatalf("expected baseline drift, got %+v", diff)
	}
	if diff.Entries[0].Kind != DiffBaseline {
		t.Fatalf("kind=%s", diff.Entries[0].Kind)
	}
}
