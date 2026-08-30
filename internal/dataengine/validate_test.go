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
