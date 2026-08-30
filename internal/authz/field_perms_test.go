package authz

import (
	"context"
	"errors"
	"testing"
)

type memFieldStore struct {
	perms []FieldPermission
}

func (m *memFieldStore) ListByPermissionSets(_ context.Context, ids []string) ([]FieldPermission, error) {
	set := map[string]struct{}{}
	for _, id := range ids {
		set[id] = struct{}{}
	}
	var out []FieldPermission
	for _, p := range m.perms {
		if _, ok := set[p.PermissionSetID]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestFieldAuthzDenyIfAbsent(t *testing.T) {
	az := &FieldAuthz{Store: &memFieldStore{}}
	actor := &Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	data := map[string]any{"Name": "Acme", "Secret__c": "x", "Id": "1"}
	out, err := az.StripUnreadableFields(context.Background(), actor, "Account", data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["Secret__c"]; ok {
		t.Fatalf("expected deny-if-absent strip, got %#v", out)
	}
	if _, ok := out["Name"]; ok {
		t.Fatalf("Name without grant should be stripped, got %#v", out)
	}
	if out["Id"] != "1" {
		t.Fatalf("system Id should remain, got %#v", out)
	}
	if err := az.AssertEditableFields(context.Background(), actor, "Account", map[string]any{"Name": "B"}); err == nil {
		t.Fatal("expected edit forbidden when no grant")
	}
}

func TestFieldAuthzStripAndEdit(t *testing.T) {
	az := &FieldAuthz{Store: &memFieldStore{perms: []FieldPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", FieldAPIName: "Secret__c", CanRead: false, CanEdit: false},
		{PermissionSetID: "ps1", ObjectAPIName: "Account", FieldAPIName: "Name", CanRead: true, CanEdit: true},
	}}}
	actor := &Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	data := map[string]any{"Id": "1", "Name": "Acme", "Secret__c": "x"}
	out, err := az.StripUnreadableFields(context.Background(), actor, "Account", data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["Secret__c"]; ok {
		t.Fatal("Secret__c should be stripped")
	}
	if out["Name"] != "Acme" || out["Id"] != "1" {
		t.Fatalf("unexpected %#v", out)
	}
	if err := az.AssertEditableFields(context.Background(), actor, "Account", map[string]any{"Secret__c": "y"}); err == nil {
		t.Fatal("expected edit forbidden")
	}
	if err := az.AssertEditableFields(context.Background(), actor, "Account", map[string]any{"Name": "B"}); err != nil {
		t.Fatal(err)
	}
}

func TestFieldAuthzMultiPSAdditive(t *testing.T) {
	az := &FieldAuthz{Store: &memFieldStore{perms: []FieldPermission{
		{PermissionSetID: "psA", ObjectAPIName: "Account", FieldAPIName: "Salary__c", CanRead: true, CanEdit: false},
		{PermissionSetID: "psA", ObjectAPIName: "Account", FieldAPIName: "Name", CanRead: true, CanEdit: true},
		{PermissionSetID: "psB", ObjectAPIName: "Account", FieldAPIName: "Salary__c", CanRead: false, CanEdit: false},
		{PermissionSetID: "psB", ObjectAPIName: "Account", FieldAPIName: "Name", CanRead: false, CanEdit: false},
		{PermissionSetID: "psB", ObjectAPIName: "Account", FieldAPIName: "Phone", CanRead: true, CanEdit: true},
	}}}
	actor := &Actor{ID: "u1", PermissionSetIDs: []string{"psA", "psB"}}
	data := map[string]any{"Name": "Acme", "Salary__c": 100, "Phone": "1", "Other__c": "x"}
	out, err := az.StripUnreadableFields(context.Background(), actor, "Account", data)
	if err != nil {
		t.Fatal(err)
	}
	if out["Salary__c"] != 100 {
		t.Fatalf("PS-A read should OR over PS-B deny stub: %#v", out)
	}
	if out["Phone"] != "1" || out["Name"] != "Acme" {
		t.Fatalf("expected additive grants, got %#v", out)
	}
	if _, ok := out["Other__c"]; ok {
		t.Fatal("ungranted field must be denied")
	}
	if err := az.AssertEditableFields(context.Background(), actor, "Account", map[string]any{"Salary__c": 2}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Salary edit should stay forbidden, got %v", err)
	}
	if err := az.AssertEditableFields(context.Background(), actor, "Account", map[string]any{"Phone": "2"}); err != nil {
		t.Fatal(err)
	}
}

func TestFieldAuthzAdminBypass(t *testing.T) {
	az := &FieldAuthz{Store: &memFieldStore{perms: []FieldPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", FieldAPIName: "Secret__c", CanRead: false, CanEdit: false},
	}}}
	actor := &Actor{ID: "u1", IsAdmin: true, PermissionSetIDs: []string{"ps1"}}
	data := map[string]any{"Secret__c": "x"}
	out, err := az.StripUnreadableFields(context.Background(), actor, "Account", data)
	if err != nil || out["Secret__c"] != "x" {
		t.Fatalf("admin should bypass FLS: %#v %v", out, err)
	}
}
