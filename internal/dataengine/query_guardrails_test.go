package dataengine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseQueryRequestRejectsCursorWithCustomSort(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"object": "Account",
		"sort":   []map[string]any{{"field": "Name", "direction": "asc"}},
		"cursor": "abc",
	})
	_, err := ParseQueryRequest(raw)
	if err == nil || !strings.Contains(err.Error(), "cursor pagination") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseQueryRequestEnforcesMaxChildRelationships(t *testing.T) {
	rels := make([]map[string]any, Limits.MaxChildRelationships+1)
	for i := range rels {
		rels[i] = map[string]any{"type": "child", "field": "AccountId", "object": "Contact"}
	}
	raw, _ := json.Marshal(map[string]any{"object": "Account", "relationships": rels})
	_, err := ParseQueryRequest(raw)
	if err == nil || !strings.Contains(err.Error(), "child relationships") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseQueryRequestEnforcesRelationshipRowBudget(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"object": "Account",
		"limit":  Limits.StandardMaxRows,
		"relationships": []map[string]any{{
			"type": "child", "field": "AccountId", "object": "Contact", "limit": 11,
		}},
	})
	_, err := ParseQueryRequest(raw)
	if err == nil || !strings.Contains(err.Error(), "row budget") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseQueryRequestRejectsInvalidCursor(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"object": "Account", "cursor": "not-a-cursor"})
	_, err := ParseQueryRequest(raw)
	if err == nil || !strings.Contains(err.Error(), "cursor is invalid") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseQueryRequestRejectsEmptyInList(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"object":  "Account",
		"filters": []map[string]any{{"field": "Name", "op": "in", "value": []any{}}},
	})
	_, err := ParseQueryRequest(raw)
	if err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("err=%v", err)
	}
}

func TestProjectJSONBExprPrunes(t *testing.T) {
	got, err := projectJSONBExpr("r", []string{"Name", "Id", "Website"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "jsonb_build_object") || !strings.Contains(got, "'Name'") {
		t.Fatalf("got=%s", got)
	}
	if strings.Contains(got, "'Id'") {
		t.Fatalf("system Id should be skipped from jsonb prune: %s", got)
	}
}

func TestAssertFlexibleRejectsUnindexedLike(t *testing.T) {
	err := assertFlexibleQueryGuardrails("flexible", &QueryRequest{
		Filters: []QueryFilter{{Field: "Name", Op: OpLike, Value: "%x%"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
