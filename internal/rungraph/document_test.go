package rungraph_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/rungraph"
)

func validDocument() map[string]any {
	return map[string]any{
		"apiVersion": rungraph.DocumentAPIVersion,
		"id":         "home",
		"title":      "My graph",
		"nodes": []any{
			map[string]any{
				"id":   "account-1",
				"kind": "record",
				"ref": map[string]any{
					"objectApiName": "Account",
					"recordId":      "00000000-0000-4000-8000-000000000111",
				},
				"layout":         map[string]any{"x": 10, "y": 20},
				"cardProjection": []any{"Name", "Industry"},
			},
			map[string]any{"id": "note-1", "kind": "insight", "text": "Follow up"},
		},
		"edges": []any{
			map[string]any{"id": "edge-1", "from": "note-1", "to": "account-1", "kind": "explains"},
		},
		"dataBindings": []any{
			map[string]any{
				"id":            "open-accounts",
				"objectApiName": "Account",
				"fields":        []any{"Name"},
				"filters": []any{
					map[string]any{"field": "Type", "op": "eq", "value": "Customer"},
				},
				"limit": 20,
			},
		},
	}
}

func marshal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPrepareDocumentStripsBakedPayloadsRecursively(t *testing.T) {
	doc := validDocument()
	doc["rows"] = []any{map[string]any{"Name": "must not persist"}}
	nodes := doc["nodes"].([]any)
	record := nodes[0].(map[string]any)
	record["fields"] = map[string]any{"Name": "must not persist"}
	record["cards"] = []any{map[string]any{"title": "must not persist"}}
	record["hydrated"] = map[string]any{"rows": []any{"nested"}}
	record["layout"].(map[string]any)["snapshot"] = map[string]any{"Name": "must not persist"}
	doc["dataBindings"].([]any)[0].(map[string]any)["filters"].([]any)[0].(map[string]any)["data"] =
		map[string]any{"Name": "must not persist"}

	sanitized, err := rungraph.PrepareDocument(marshal(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(sanitized, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["rows"]; ok {
		t.Fatal("top-level rows survived sanitize")
	}
	gotRecord := got["nodes"].([]any)[0].(map[string]any)
	for _, key := range []string{"fields", "cards", "hydrated"} {
		if _, ok := gotRecord[key]; ok {
			t.Fatalf("record node %s survived sanitize", key)
		}
	}
	if _, ok := gotRecord["layout"].(map[string]any)["snapshot"]; ok {
		t.Fatal("nested snapshot survived sanitize")
	}
	if gotRecord["ref"].(map[string]any)["recordId"] == "" {
		t.Fatal("record ref recordId was stripped")
	}
	binding := got["dataBindings"].([]any)[0].(map[string]any)
	if _, ok := binding["fields"]; !ok {
		t.Fatal("binding fields definition was stripped")
	}
	filter := binding["filters"].([]any)[0].(map[string]any)
	if filter["value"] != "Customer" {
		t.Fatal("binding filter value was stripped")
	}
	if _, ok := filter["data"]; ok {
		t.Fatal("nested binding data payload survived sanitize")
	}
}

func TestValidateDocumentRejectsInvalidKindAndUnknownKeys(t *testing.T) {
	doc := validDocument()
	doc["nodes"].([]any)[0].(map[string]any)["kind"] = "iframe"
	if err := rungraph.ValidateDocument(marshal(t, doc)); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("expected invalid kind, got %v", err)
	}

	doc = validDocument()
	doc["nodes"].([]any)[0].(map[string]any)["secretRecordMap"] = map[string]any{"Name": "no"}
	if err := rungraph.ValidateDocument(marshal(t, doc)); err == nil || !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("expected closed-schema rejection, got %v", err)
	}

	doc = validDocument()
	doc["nodes"].([]any)[0].(map[string]any)["ref"].(map[string]any)["recordId"] = "not-a-uuid"
	if err := rungraph.ValidateDocument(marshal(t, doc)); err == nil || !strings.Contains(err.Error(), "must be a UUID") {
		t.Fatalf("expected invalid record ref rejection, got %v", err)
	}
}

func TestValidateDocumentAcceptsCollectionNodes(t *testing.T) {
	doc := validDocument()
	doc["nodes"] = append(doc["nodes"].([]any), map[string]any{
		"id":        "accounts",
		"kind":      "collection",
		"label":     "Accounts",
		"ref":       map[string]any{"objectApiName": "Account"},
		"bindingId": "open-accounts",
		"searchQ":   "Acme",
	})
	if err := rungraph.ValidateDocument(marshal(t, doc)); err != nil {
		t.Fatalf("collection node should validate: %v", err)
	}

	baked := validDocument()
	baked["nodes"] = append(baked["nodes"].([]any), map[string]any{
		"id":   "accounts",
		"kind": "collection",
		"ref":  map[string]any{"objectApiName": "Account"},
		"rows": []any{map[string]any{"Name": "must not persist"}},
	})
	sanitized, err := rungraph.PrepareDocument(marshal(t, baked))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(sanitized, &got); err != nil {
		t.Fatal(err)
	}
	collection := got["nodes"].([]any)[len(got["nodes"].([]any))-1].(map[string]any)
	if _, ok := collection["rows"]; ok {
		t.Fatal("collection rows survived sanitize")
	}

	mismatch := validDocument()
	mismatch["nodes"] = append(mismatch["nodes"].([]any), map[string]any{
		"id":        "contacts",
		"kind":      "collection",
		"ref":       map[string]any{"objectApiName": "Contact"},
		"bindingId": "open-accounts",
	})
	if err := rungraph.ValidateDocument(marshal(t, mismatch)); err == nil || !strings.Contains(err.Error(), "does not match collection") {
		t.Fatalf("expected binding object mismatch, got %v", err)
	}

	missing := validDocument()
	missing["nodes"] = append(missing["nodes"].([]any), map[string]any{
		"id":        "accounts",
		"kind":      "collection",
		"ref":       map[string]any{"objectApiName": "Account"},
		"bindingId": "missing-binding",
	})
	if err := rungraph.ValidateDocument(marshal(t, missing)); err == nil || !strings.Contains(err.Error(), "does not reference a dataBinding") {
		t.Fatalf("expected missing binding rejection, got %v", err)
	}

	withRecordID := validDocument()
	withRecordID["nodes"] = append(withRecordID["nodes"].([]any), map[string]any{
		"id":   "accounts",
		"kind": "collection",
		"ref": map[string]any{
			"objectApiName": "Account",
			"recordId":      "00000000-0000-4000-8000-000000000111",
		},
	})
	if err := rungraph.ValidateDocument(marshal(t, withRecordID)); err == nil || !strings.Contains(err.Error(), "recordId is not allowed") {
		t.Fatalf("expected collection ref to reject recordId, got %v", err)
	}
}

func TestEnforceCapsRejectsCollectionSearchQLength(t *testing.T) {
	doc := validDocument()
	doc["nodes"] = append(doc["nodes"].([]any), map[string]any{
		"id":      "accounts",
		"kind":    "collection",
		"ref":     map[string]any{"objectApiName": "Account"},
		"searchQ": strings.Repeat("a", rungraph.MaxSearchQueryBytes+1),
	})
	raw := marshal(t, doc)
	if err := rungraph.ValidateDocument(raw); err != nil {
		t.Fatalf("document should validate before caps: %v", err)
	}
	if err := rungraph.EnforceCaps(raw); err == nil || !strings.Contains(err.Error(), "searchQ exceeds") {
		t.Fatalf("expected searchQ cap rejection, got %v", err)
	}
}

func TestEnforceCapsRejectsNodesAndAnnotationLength(t *testing.T) {
	doc := validDocument()
	nodes := make([]any, 0, rungraph.MaxNodes+1)
	for i := 0; i <= rungraph.MaxNodes; i++ {
		nodes = append(nodes, map[string]any{"id": "cluster-" + strings.Repeat("x", i%3+1) + string(rune(i+1000)), "kind": "cluster", "label": "Cluster"})
	}
	doc["nodes"] = nodes
	doc["edges"] = []any{}
	raw := marshal(t, doc)
	if err := rungraph.ValidateDocument(raw); err != nil {
		t.Fatalf("document should validate before caps: %v", err)
	}
	if err := rungraph.EnforceCaps(raw); err == nil || !strings.Contains(err.Error(), "nodes exceeds") {
		t.Fatalf("expected node cap rejection, got %v", err)
	}

	doc = validDocument()
	doc["nodes"].([]any)[1].(map[string]any)["text"] = strings.Repeat("a", rungraph.MaxAnnotationBytes+1)
	raw = marshal(t, doc)
	if err := rungraph.EnforceCaps(raw); err == nil || !strings.Contains(err.Error(), "text exceeds") {
		t.Fatalf("expected annotation cap rejection, got %v", err)
	}
}

func TestMergePatchReplacesArraysAndPreservesDocument(t *testing.T) {
	doc := marshal(t, validDocument())
	patch := marshal(t, map[string]any{
		"title": "Updated graph",
		"nodes": []any{map[string]any{"id": "cluster-1", "kind": "cluster", "label": "Today"}},
		"edges": []any{},
	})
	merged, err := rungraph.MergePatch(doc, patch)
	if err != nil {
		t.Fatal(err)
	}
	if err := rungraph.ValidateDocument(merged); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(merged, &got)
	if got["title"] != "Updated graph" || len(got["nodes"].([]any)) != 1 {
		t.Fatalf("unexpected merge result: %s", merged)
	}
}
