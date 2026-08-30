package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/metadata"
)

type memObjectStore struct {
	perms []authz.ObjectPermission
}

func (m *memObjectStore) ListByPermissionSets(_ context.Context, ids []string) ([]authz.ObjectPermission, error) {
	set := map[string]struct{}{}
	for _, id := range ids {
		set[id] = struct{}{}
	}
	var out []authz.ObjectPermission
	for _, p := range m.perms {
		if _, ok := set[p.PermissionSetID]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func TestFilterDescribeGlobalByObjectRead(t *testing.T) {
	objectAz := &authz.ObjectAuthz{Store: &memObjectStore{perms: []authz.ObjectPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", CanRead: true},
		{PermissionSetID: "ps1", ObjectAPIName: "Secret__c", CanRead: false},
	}}}
	actor := &authz.Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	desc := &metadata.GlobalDescribe{
		SObjects: []metadata.GlobalSObjectRef{
			{Name: "Account", Label: "Account"},
			{Name: "Secret__c", Label: "Secret"},
		},
	}
	out, err := filterDescribeGlobal(context.Background(), objectAz, actor, desc)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.SObjects) != 1 || out.SObjects[0].Name != "Account" {
		t.Fatalf("expected only Account, got %#v", out.SObjects)
	}
}

func TestFilterDescribeObjectFLS(t *testing.T) {
	objectAz := &authz.ObjectAuthz{Store: &memObjectStore{perms: []authz.ObjectPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", CanRead: true},
	}}}
	fieldAz := &authz.FieldAuthz{Store: &memFieldStore{perms: []authz.FieldPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", FieldAPIName: "Name", CanRead: true, CanEdit: true},
		{PermissionSetID: "ps1", ObjectAPIName: "Account", FieldAPIName: "Salary__c", CanRead: false, CanEdit: false},
	}}}
	actor := &authz.Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	desc := &metadata.DescribeObject{
		ObjectDefinition: metadata.ObjectDefinition{APIName: "Account"},
		Fields: []metadata.FieldDefinition{
			{APIName: "Name"},
			{APIName: "Salary__c"},
			{APIName: "Id"},
		},
	}
	out, err := filterDescribeObject(context.Background(), objectAz, fieldAz, actor, desc)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range out.Fields {
		names[f.APIName] = true
	}
	if !names["Name"] || !names["Id"] || names["Salary__c"] {
		t.Fatalf("unexpected fields %#v", names)
	}

	noRead := &authz.Actor{ID: "u2", PermissionSetIDs: []string{"ps1"}}
	objectAzDeny := &authz.ObjectAuthz{Store: &memObjectStore{perms: []authz.ObjectPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", CanRead: false},
	}}}
	_, err = filterDescribeObject(context.Background(), objectAzDeny, fieldAz, noRead, desc)
	if !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

// memFieldStore lives in describe_authz_test for httpapi package tests.
type memFieldStore struct {
	perms []authz.FieldPermission
}

func (m *memFieldStore) ListByPermissionSets(_ context.Context, ids []string) ([]authz.FieldPermission, error) {
	set := map[string]struct{}{}
	for _, id := range ids {
		set[id] = struct{}{}
	}
	var out []authz.FieldPermission
	for _, p := range m.perms {
		if _, ok := set[p.PermissionSetID]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
