package canvas

import (
	"encoding/json"
	"testing"
)

func TestValidateDocument(t *testing.T) {
	doc := map[string]any{
		"apiVersion": DocumentAPIVersion,
		"id":         "c1",
		"title":      "Pipeline",
		"layout":     map[string]any{"mode": "sections"},
		"nodes": []map[string]any{
			{"id": "n1", "kind": "stat", "props": map[string]any{"value": 1}},
		},
	}
	raw, _ := json.Marshal(doc)
	if err := ValidateDocument(raw); err != nil {
		t.Fatalf("valid doc: %v", err)
	}

	bad := map[string]any{
		"apiVersion": DocumentAPIVersion,
		"id":         "c1",
		"title":      "Bad",
		"layout":     map[string]any{"mode": "sections"},
		"nodes": []map[string]any{
			{"id": "n1", "kind": "rawHtml", "props": map[string]any{}},
		},
	}
	badRaw, _ := json.Marshal(bad)
	if err := ValidateDocument(badRaw); err == nil {
		t.Fatal("expected unknown kind rejection")
	}
}
