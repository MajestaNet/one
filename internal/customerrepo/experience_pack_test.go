package customerrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MajestaNet/ide/internal/deploy"
)

func TestParseExperienceYAMLAndRoundTrip(t *testing.T) {
	raw := []byte(`apiName: AcmePortal
label: Acme CRM Portal
description: End-user list views
homeUrl: https://portal.acme.example
connectedAppApiName: acme.portal
allowedOrigins:
  - https://portal.acme.example
active: true
ownership: custom
`)
	ex, err := parseExperienceYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ex.APIName != "AcmePortal" || ex.HomeURL == "" || ex.ConnectedAppAPIName != "acme.portal" {
		t.Fatalf("unexpected %+v", ex)
	}
	dir := t.TempDir()
	art := &deploy.BundleArtifact{
		Experiences: []deploy.SnapshotExperience{ex},
	}
	if err := UnpackToDir(dir, art, Manifest{PackageName: "customer.default"}); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "metadata", "experiences", ex.APIName+".yaml")
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	again, err := parseExperienceYAML(body)
	if err != nil {
		t.Fatal(err)
	}
	if again.APIName != ex.APIName || again.HomeURL != ex.HomeURL {
		t.Fatalf("round trip mismatch: %+v vs %+v", again, ex)
	}
}
