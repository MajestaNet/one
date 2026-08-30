package dataengine_test

import (
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestNormalizeRejectsAutonumberClientValue(t *testing.T) {
	fields := []metadata.FieldDefinition{{
		APIName: "CaseNumber", FieldType: metadata.FieldTypeAutonumber, Label: "Case Number",
	}}
	_, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{"CaseNumber": "A-1"}, "create")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestAssertAddressAndGeolocation(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "BillingAddress", FieldType: metadata.FieldTypeAddress, Label: "Billing"},
		{APIName: "Loc", FieldType: metadata.FieldTypeGeolocation, Label: "Loc"},
	}
	_, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{
		"BillingAddress": map[string]any{"street": "1 Main", "city": "Austin", "extra": "no"},
	}, "create")
	if err == nil {
		t.Fatal("expected reject extra address key")
	}
	_, err = dataengine.NormalizeAndValidateFields(fields, map[string]any{
		"Loc": map[string]any{"latitude": 30.0, "longitude": -97.0},
	}, "create")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRichTextSanitizesScript(t *testing.T) {
	n := 32000
	fields := []metadata.FieldDefinition{{
		APIName: "Body", FieldType: metadata.FieldTypeRichText, Label: "Body", Length: &n,
	}}
	out, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{
		"Body": `<p>hi</p><script>alert(1)</script>`,
	}, "create")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := out["Body"].(string)
	if strings.Contains(strings.ToLower(s), "<script") {
		t.Fatalf("script not sanitized: %s", s)
	}
}

func TestIntegerFieldRejectsFractional(t *testing.T) {
	fields := []metadata.FieldDefinition{{
		APIName: "Qty", FieldType: metadata.FieldTypeInteger, Label: "Qty",
	}}
	_, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{"Qty": 1.5}, "create")
	if err == nil {
		t.Fatal("expected reject non-integer")
	}
	out, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{"Qty": 2}, "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["Qty"] != float64(2) {
		t.Fatalf("%v", out["Qty"])
	}
}
