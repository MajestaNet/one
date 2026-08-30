package customerrepo_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/customerrepo"
	"github.com/MajestaNet/ide/internal/deploy"
)

func TestPackArchiveRejectsExpandedSizeBombBeforeExtraction(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	if err := tw.WriteHeader(&tar.Header{Name: "large.yaml", Mode: 0o600, Size: 129 << 20, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	// Deliberately do not write the declared body: admission must reject from the
	// authenticated tar header before allocating or extracting the payload.
	_, _, err := customerrepo.PackArchive(bytes.NewReader(raw.Bytes()), int64(raw.Len()), "application/x-tar", customerrepo.PackOptions{})
	if err == nil || !strings.Contains(err.Error(), "archive expands beyond") {
		t.Fatalf("err=%v want expanded archive limit", err)
	}
}

func TestPackArchiveRejectsTooManyFiles(t *testing.T) {
	var raw bytes.Buffer
	zw := zip.NewWriter(&raw)
	for i := 0; i < 10_001; i++ {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("f%d.txt", i), Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte{'x'}); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, err := customerrepo.PackArchive(bytes.NewReader(raw.Bytes()), int64(raw.Len()), "application/zip", customerrepo.PackOptions{})
	if err == nil || !strings.Contains(err.Error(), "archive exceeds") {
		t.Fatalf("err=%v want file count limit", err)
	}
}

func TestPackUnpackRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: customer.default
productVersionRange: ">=0.1.0 <0.2.0"
repoFormat: one/v1
`)
	mustWrite(t, filepath.Join(root, "metadata", "objects", "Invoice__c.yaml"), `
apiName: Invoice__c
label: Invoice
pluralLabel: Invoices
storageMode: flexible
ownership: custom
features: {}
`)
	mustWrite(t, filepath.Join(root, "metadata", "fields", "Invoice__c", "Amount__c.yaml"), `
objectApiName: Invoice__c
apiName: Amount__c
label: Amount
fieldType: number
required: true
ownership: custom
`)
	mustWrite(t, filepath.Join(root, "metadata", "permission-sets", "Builder.yaml"), `
apiName: Builder
label: Builder
ownership: custom
systemPermissions: []
objectPermissions:
  - objectApiName: Invoice__c
    canCreate: true
    canRead: true
    canUpdate: true
    canDelete: false
    viewAll: false
    modifyAll: false
`)
	mustWrite(t, filepath.Join(root, "metadata", "data-roles", "Sales.yaml"), `
apiName: Sales
label: Sales
`)
	mustWrite(t, filepath.Join(root, "metadata", "object-sharing", "Invoice__c.yaml"), `
objectApiName: Invoice__c
defaultAccess: private
sharingRulesEnabled: true
`)
	mustWrite(t, filepath.Join(root, "metadata", "sharing-rules", "Invoice__c", "Sales_Read.yaml"), `
objectApiName: Invoice__c
apiName: Sales_Read
label: Sales Read
active: true
accessLevel: read
sharedToDataRoleApiName: Sales
criteria:
  "==":
    - var: Status__c
    - Open
sortOrder: 1
`)
	mustWrite(t, filepath.Join(root, ".one", "baseline", "manifest.yaml"), `
productVersion: 0.1.0
generatedAt: "2026-01-01T00:00:00Z"
`)
	mustWrite(t, filepath.Join(root, ".one", "baseline", "objects", "Account.yaml"), `
apiName: Account
label: Account
pluralLabel: Accounts
storageMode: flexible
ownership: managed
packageName: core
features: {}
`)
	mustWrite(t, filepath.Join(root, ".one", "baseline", "fields", "Account", "Name.yaml"), `
objectApiName: Account
apiName: Name
label: Account Name
fieldType: text
required: true
ownership: managed
packageName: core
`)
	mustWrite(t, filepath.Join(root, "tests", "Smoke.yaml"), `
apiName: Smoke
label: Smoke
active: true
ownership: custom
steps:
  - type: objectExists
    objectApiName: Invoice__c
`)

	art, man, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if man.CustomerID != "acme" {
		t.Fatalf("customerId=%s", man.CustomerID)
	}
	if len(art.Objects) != 1 || art.Objects[0].APIName != "Invoice__c" {
		t.Fatalf("objects=%+v", art.Objects)
	}
	if len(art.Fields) != 1 || art.Fields[0].APIName != "Amount__c" {
		t.Fatalf("fields=%+v", art.Fields)
	}
	if len(art.PermissionSets) != 1 {
		t.Fatalf("permissionSets=%+v", art.PermissionSets)
	}
	if len(art.DataRoles) != 1 || art.DataRoles[0].APIName != "Sales" {
		t.Fatalf("dataRoles=%+v", art.DataRoles)
	}
	if len(art.ObjectSharingSettings) != 1 {
		t.Fatalf("objectSharing=%+v", art.ObjectSharingSettings)
	}
	if len(art.SharingRules) != 1 || art.SharingRules[0].APIName != "Sales_Read" {
		t.Fatalf("sharingRules=%+v", art.SharingRules)
	}
	if art.Baseline != nil {
		t.Fatal("pack must ignore .one/baseline")
	}
	if len(art.Tests) != 1 {
		t.Fatalf("tests=%+v", art.Tests)
	}

	out := t.TempDir()
	if err := customerrepo.UnpackToDir(out, art, *man); err != nil {
		t.Fatalf("unpack: %v", err)
	}
	art2, _, err := customerrepo.PackFromDir(out, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("repack: %v", err)
	}
	if len(art2.Objects) != 1 || len(art2.Fields) != 1 || len(art2.Tests) != 1 {
		t.Fatalf("repack counts objects=%d fields=%d tests=%d", len(art2.Objects), len(art2.Fields), len(art2.Tests))
	}
	if len(art2.DataRoles) != 1 || len(art2.ObjectSharingSettings) != 1 || len(art2.SharingRules) != 1 {
		t.Fatalf("repack sharing roles=%d owd=%d rules=%d", len(art2.DataRoles), len(art2.ObjectSharingSettings), len(art2.SharingRules))
	}
}

func TestUnpackWritesBaseline(t *testing.T) {
	pkg := "core"
	art := &deploy.BundleArtifact{
		ManifestVersion:    1,
		Ownership:          "custom",
		DefaultPackageName: deploy.DefaultCustomerPackage,
		Objects: []deploy.SnapshotObject{{
			APIName: "Invoice__c", Label: "Invoice", PluralLabel: "Invoices",
			StorageMode: "flexible", Ownership: "custom", Features: map[string]bool{},
		}},
		Baseline: &deploy.ManagedBaseline{
			ProductVersion:  "0.1.0",
			GeneratedAt:     "2026-01-01T00:00:00Z",
			SourceInstallID: "prod-1",
			Objects: []deploy.SnapshotObject{{
				APIName: "Account", Label: "Account", PluralLabel: "Accounts",
				StorageMode: "flexible", Ownership: "managed", PackageName: &pkg, Features: map[string]bool{},
			}},
			Fields: []deploy.SnapshotField{{
				ObjectAPIName: "Account", APIName: "Name", Label: "Name",
				FieldType: "text", Required: true, Ownership: "managed", PackageName: &pkg,
			}},
		},
	}
	out := t.TempDir()
	man := customerrepo.Manifest{CustomerID: "acme", PackageName: deploy.DefaultCustomerPackage, RepoFormat: customerrepo.RepoFormat}
	if err := customerrepo.UnpackToDir(out, art, man); err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, ".one", "baseline", "manifest.yaml")); err != nil {
		t.Fatalf("baseline manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, ".one", "baseline", "objects", "Account.yaml")); err != nil {
		t.Fatalf("baseline object: %v", err)
	}
	packed, _, err := customerrepo.PackFromDir(out, customerrepo.PackOptions{})
	if err != nil {
		t.Fatalf("pack after baseline unpack: %v", err)
	}
	if packed.Baseline != nil {
		t.Fatal("repack must not include baseline")
	}
	if len(packed.Objects) != 1 || packed.Objects[0].Ownership != "custom" {
		t.Fatalf("unexpected objects after pack: %+v", packed.Objects)
	}
}

func TestPackRejectsManagedPackage(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.yaml"), `
customerId: acme
packageName: core
repoFormat: one/v1
`)
	_, _, err := customerrepo.PackFromDir(root, customerrepo.PackOptions{})
	if err == nil {
		t.Fatal("expected error for managed packageName")
	}
}

func TestParseBundleDefaultsNewKinds(t *testing.T) {
	raw := map[string]any{
		"manifestVersion": 1,
		"ownership":       "custom",
		"permissionSets": []any{
			map[string]any{"apiName": "X", "label": "X"},
		},
		"webhooks": []any{
			map[string]any{"apiName": "Hook", "url": "https://example.com/h"},
		},
		"tests": []any{
			map[string]any{"apiName": "T", "label": "T"},
		},
	}
	art, err := deploy.ParseBundleArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	if art.PermissionSets[0].Ownership != "custom" {
		t.Fatalf("ps ownership=%s", art.PermissionSets[0].Ownership)
	}
	if len(art.Webhooks[0].EventTypes) == 0 {
		t.Fatal("expected default event types")
	}
	if art.Tests[0].Steps == nil {
		t.Fatal("expected steps slice")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
