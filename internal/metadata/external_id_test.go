package metadata_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/metadata"
)

func TestApplyExternalIDRules(t *testing.T) {
	f := metadata.FieldDefinition{FieldType: metadata.FieldTypeText, ExternalID: true}
	if err := metadata.ApplyExternalIDRules(&f); err != nil {
		t.Fatal(err)
	}
	if !f.UniqueField || !f.Indexed || !f.Filterable {
		t.Fatalf("expected unique+indexed+filterable, got unique=%v indexed=%v filterable=%v", f.UniqueField, f.Indexed, f.Filterable)
	}

	bad := metadata.FieldDefinition{FieldType: metadata.FieldTypeBoolean, ExternalID: true}
	if err := metadata.ApplyExternalIDRules(&bad); err == nil || !strings.Contains(err.Error(), "externalId") {
		t.Fatalf("expected externalId type error, got %v", err)
	}
}

func TestIsExternalIDEligible(t *testing.T) {
	if !metadata.IsExternalIDEligible(metadata.FieldTypeEmail) {
		t.Fatal("email should be eligible")
	}
	if metadata.IsExternalIDEligible(metadata.FieldTypeLookup) {
		t.Fatal("lookup should not be eligible")
	}
}
