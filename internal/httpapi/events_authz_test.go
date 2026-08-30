package httpapi

import (
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

func TestEventVisibleToActor(t *testing.T) {
	actor := &authz.Actor{ID: "u1", Scopes: []authz.Scope{authz.ScopeClient}}
	obj := "Account"
	ownerOther := "other"
	ownerSelf := "u1"
	createdSelf := "u1"
	viewAll := map[string]struct{}{"Account": {}}

	if !eventVisibleToActor(actor, &obj, &ownerOther, &createdSelf, map[string]any{}, viewAll) {
		t.Fatal("viewAll should allow")
	}
	if eventVisibleToActor(actor, &obj, &ownerOther, &ownerOther, map[string]any{}, map[string]struct{}{}) {
		t.Fatal("foreign record should be hidden")
	}
	if !eventVisibleToActor(actor, &obj, &ownerSelf, &ownerOther, map[string]any{}, map[string]struct{}{}) {
		t.Fatal("owner should see")
	}
	if !eventVisibleToActor(actor, nil, nil, nil, map[string]any{"actorId": "u1"}, map[string]struct{}{}) {
		t.Fatal("actor-produced event should be visible")
	}
	admin := &authz.Actor{ID: "a", IsAdmin: true}
	if !eventVisibleToActor(admin, &obj, &ownerOther, &ownerOther, map[string]any{"data": map[string]any{"x": 1}}, nil) {
		t.Fatal("admin should see all")
	}
}

func TestRedactEventPayload(t *testing.T) {
	actor := &authz.Actor{ID: "u1"}
	p := redactEventPayload(actor, map[string]any{
		"action": "create", "data": map[string]any{"Name": "x"}, "patch": map[string]any{"Name": "y"}, "recordId": "r1",
	})
	m := p.(map[string]any)
	if _, ok := m["data"]; ok {
		t.Fatal("data should be redacted")
	}
	if _, ok := m["patch"]; ok {
		t.Fatal("patch should be redacted")
	}
	if m["recordId"] != "r1" {
		t.Fatal("metadata should remain")
	}
	admin := &authz.Actor{ID: "a", IsAdmin: true}
	full := redactEventPayload(admin, map[string]any{"data": 1})
	if full.(map[string]any)["data"] != 1 {
		t.Fatal("admin keeps payload")
	}
}
