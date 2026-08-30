package metadata_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/metadata"
)

func TestApplySearchableRules(t *testing.T) {
	f := metadata.FieldDefinition{FieldType: metadata.FieldTypePhone, Searchable: true}
	if err := metadata.ApplySearchableRules(&f); err != nil {
		t.Fatal(err)
	}
	if !f.Filterable {
		t.Fatal("searchable should force filterable")
	}
	if f.Indexed {
		t.Fatal("searchable must not auto-set indexed")
	}

	bad := metadata.FieldDefinition{FieldType: metadata.FieldTypePicklist, Searchable: true}
	if err := metadata.ApplySearchableRules(&bad); err == nil || !strings.Contains(err.Error(), "searchable") {
		t.Fatalf("expected searchable type error, got %v", err)
	}
}

func TestIsSearchableEligible(t *testing.T) {
	if !metadata.IsSearchableEligible(metadata.FieldTypeEmail) {
		t.Fatal("email should be searchable-eligible")
	}
	if metadata.IsSearchableEligible(metadata.FieldTypeLookup) {
		t.Fatal("lookup should not be searchable-eligible")
	}
	if metadata.IsSearchableEligible(metadata.FieldTypeTextarea) {
		t.Fatal("textarea should not be searchable-eligible")
	}
}
