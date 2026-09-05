package dataengine_test

import (
	"encoding/json"
	"testing"

	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestNormalizeAndValidateFields(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "Name", Label: "Name", FieldType: "text", Required: true, Filterable: true, Sortable: true},
	}
	_, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{}, "create")
	if err == nil {
		t.Fatal("expected required error")
	}
	data, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{"Name": "Acme"}, "create")
	if err != nil || data["Name"] != "Acme" {
		t.Fatalf("got %v err=%v", data, err)
	}
}

func TestNormalizeOmitsBlankOptionalFields(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "LastName", Label: "Last Name", FieldType: "text", Required: true},
		{APIName: "Email", Label: "Email", FieldType: "email"},
		{APIName: "AccountId", Label: "Account", FieldType: "lookup"},
		{APIName: "Salutation", Label: "Salutation", FieldType: "picklist", PicklistValues: []string{"Mr.", "Ms."}},
	}
	data, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{
		"LastName":   "Shah",
		"Email":      "",
		"AccountId":  "",
		"Salutation": "   ",
	}, "create")
	if err != nil {
		t.Fatal(err)
	}
	if data["LastName"] != "Shah" {
		t.Fatalf("LastName=%v", data["LastName"])
	}
	if _, ok := data["Email"]; ok {
		t.Fatalf("blank Email should be omitted: %v", data)
	}
	if _, ok := data["AccountId"]; ok {
		t.Fatalf("blank AccountId should be omitted: %v", data)
	}
	if _, ok := data["Salutation"]; ok {
		t.Fatalf("blank Salutation should be omitted: %v", data)
	}
}

func TestNormalizeRejectsBlankRequired(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "LastName", Label: "Last Name", FieldType: "text", Required: true},
	}
	_, err := dataengine.NormalizeAndValidateFields(fields, map[string]any{"LastName": "  "}, "create")
	if err == nil {
		t.Fatal("expected required error for blank LastName")
	}
}

func TestEvaluateValidationRules(t *testing.T) {
	expr, _ := json.Marshal(map[string]any{"<": []any{map[string]any{"var": "Amount"}, 0}})
	err := dataengine.EvaluateValidationRules([]metadata.ValidationRuleDefinition{
		{APIName: "positive", Active: true, ErrorMessage: "Amount must be positive", Expression: expr},
	}, map[string]any{"Amount": -1.0})
	if err == nil || err.Error() != "Amount must be positive" {
		t.Fatalf("err=%v", err)
	}
}

func TestParseQueryRequestAllowsRelationships(t *testing.T) {
	raw := []byte(`{"object":"Account","relationships":[{"type":"parent","field":"ParentId","object":"Account"}]}`)
	req, err := dataengine.ParseQueryRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Relationships) != 1 || req.Relationships[0].Type != "parent" {
		t.Fatalf("rel=%+v", req.Relationships)
	}
}
