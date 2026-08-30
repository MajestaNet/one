package metadata_test

import (
	"testing"

	"github.com/MajestaNet/ide/internal/metadata"
)

func TestListFieldTypesIncludesCoreAndEnhancements(t *testing.T) {
	types := metadata.ListFieldTypes()
	want := []string{
		metadata.FieldTypeText, metadata.FieldTypeTextarea, metadata.FieldTypeInteger,
		metadata.FieldTypePercent, metadata.FieldTypeJSON, metadata.FieldTypeAutonumber,
		metadata.FieldTypeRichText, metadata.FieldTypeAddress, metadata.FieldTypeGeolocation,
	}
	have := map[string]bool{}
	for _, info := range types {
		have[info.APIName] = true
		if info.APIName == "multipicklist" {
			t.Fatal("multipicklist must not be in catalog")
		}
		if info.APIName == "polymorphic_lookup" {
			t.Fatal("polymorphic_lookup must not be in catalog")
		}
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("missing type %s", w)
		}
	}
}

func TestValidateFieldTypeCreateRejectsAliasAndUnknown(t *testing.T) {
	err := metadata.ValidateFieldTypeCreate(&metadata.FieldDefinition{
		ObjectAPIName: "Account", APIName: "X__c", Label: "X", FieldType: "string",
	})
	if err == nil {
		t.Fatal("expected reject string alias")
	}
	err = metadata.ValidateFieldTypeCreate(&metadata.FieldDefinition{
		ObjectAPIName: "Account", APIName: "X__c", Label: "X", FieldType: "nope",
	})
	if err == nil {
		t.Fatal("expected reject unknown")
	}
	err = metadata.ValidateFieldTypeCreate(&metadata.FieldDefinition{
		ObjectAPIName: "Account", APIName: "ParentId", Label: "Parent", FieldType: "polymorphic_lookup",
	})
	if err == nil {
		t.Fatal("expected reject retired polymorphic_lookup")
	}
	ref := "Account"
	err = metadata.ValidateFieldTypeCreate(&metadata.FieldDefinition{
		ObjectAPIName: "Contact", APIName: "AccountId", Label: "Account", FieldType: "lookup", ReferenceTo: &ref,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyFieldTypeDefaultsTextarea(t *testing.T) {
	f := &metadata.FieldDefinition{FieldType: metadata.FieldTypeTextarea}
	metadata.ApplyFieldTypeDefaults(f, false, false, false)
	if f.Filterable || f.Sortable {
		t.Fatalf("textarea should default filterable/sortable false: %+v", f)
	}
	if f.Length == nil || *f.Length != 32000 {
		t.Fatalf("length=%v", f.Length)
	}
}
