package httpapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
)

func TestQueryIncludes(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/client/v1/principals?include=data", nil)
	if !queryIncludes(req, "data") {
		t.Fatal("expected include=data")
	}
	req, _ = http.NewRequest(http.MethodGet, "/client/v1/principals?include=fields,data", nil)
	if !queryIncludes(req, "data") {
		t.Fatal("expected comma list to include data")
	}
	req, _ = http.NewRequest(http.MethodGet, "/client/v1/principals", nil)
	if queryIncludes(req, "data") {
		t.Fatal("bare list must not include data")
	}
}

func TestPrincipalChangedFieldAPINames(t *testing.T) {
	emp := "E-9"
	before := &db.User{
		Email: "a@example.com", DisplayName: "A",
		Data: map[string]any{"CostCenter__c": "CC-1"},
	}
	after := &db.User{
		Email: "a@example.com", DisplayName: "A",
		EmployeeNumber: &emp,
		Data:           map[string]any{"CostCenter__c": "CC-2"},
	}
	got := principalChangedFieldAPINames(before, after, false)
	want := map[string]struct{}{"CostCenter__c": {}, "EmployeeNumber": {}}
	if len(got) != len(want) {
		t.Fatalf("fields=%v", got)
	}
	for _, n := range got {
		if _, ok := want[n]; !ok {
			t.Fatalf("unexpected %s in %v", n, got)
		}
	}
}

func TestQueryIncludesEmptyURL(t *testing.T) {
	req := &http.Request{URL: &url.URL{}}
	if queryIncludes(req, "data") {
		t.Fatal("empty query")
	}
}
