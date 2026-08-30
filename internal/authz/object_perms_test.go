package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type memStore struct {
	perms []authz.ObjectPermission
}

func (m *memStore) ListByPermissionSets(_ context.Context, ids []string) ([]authz.ObjectPermission, error) {
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

func TestAssertObjectAccess(t *testing.T) {
	store := &memStore{perms: []authz.ObjectPermission{{
		PermissionSetID: "ps1",
		ObjectAPIName:   "Account",
		CanRead:         true,
	}}}
	az := &authz.ObjectAuthz{Store: store}
	actor := &authz.Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}, Scopes: []authz.Scope{authz.ScopeClient}}

	if err := az.AssertObjectAccess(context.Background(), actor, "Account", authz.ActionRead); err != nil {
		t.Fatal(err)
	}
	if err := az.AssertObjectAccess(context.Background(), actor, "Account", authz.ActionCreate); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	admin := &authz.Actor{ID: "a", IsAdmin: true, PermissionSetIDs: nil, Scopes: authz.AllScopes}
	if err := az.AssertObjectAccess(context.Background(), admin, "Account", authz.ActionDelete); err != nil {
		t.Fatal(err)
	}
}

func TestCanViewRecordAndViewAll(t *testing.T) {
	store := &memStore{perms: []authz.ObjectPermission{{
		PermissionSetID: "ps1",
		ObjectAPIName:   "Account",
		ViewAll:         true,
	}}}
	az := &authz.ObjectAuthz{Store: store}
	actor := &authz.Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	viewAll, err := az.GetViewAllObjects(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	// view_all on Account
	if !authz.CanViewRecord(actor, "other", "creator", "Account", viewAll) {
		t.Fatal("expected view all")
	}
	// no view_all on Contact; not owner; not creator
	if authz.CanViewRecord(actor, "other", "creator", "Contact", viewAll) {
		t.Fatal("should not view Contact")
	}
	// owner match
	if !authz.CanViewRecord(actor, "u1", "creator", "Contact", viewAll) {
		t.Fatal("owner should view own")
	}
	// created-by match with null owner
	if !authz.CanViewRecord(actor, "", "u1", "Contact", viewAll) {
		t.Fatal("creator should view own")
	}
	// null owner, other creator → deny
	if authz.CanViewRecord(actor, "", "other", "Contact", viewAll) {
		t.Fatal("should deny when neither owner nor creator")
	}
}

func TestCanModifyRecordAndOwnerID(t *testing.T) {
	store := &memStore{perms: []authz.ObjectPermission{
		{PermissionSetID: "ps1", ObjectAPIName: "Account", CanUpdate: true},
		{PermissionSetID: "ps1", ObjectAPIName: "Contact", ModifyAll: true},
	}}
	az := &authz.ObjectAuthz{Store: store}
	actor := &authz.Actor{ID: "u1", PermissionSetIDs: []string{"ps1"}}
	modifyAll, err := az.GetModifyAllObjects(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	// can_update alone does not grant modify of others' records
	if authz.CanModifyRecord(actor, "other", "creator", "Account", modifyAll) {
		t.Fatal("should not modify foreign Account without modifyAll")
	}
	// owner can modify
	if !authz.CanModifyRecord(actor, "u1", "creator", "Account", modifyAll) {
		t.Fatal("owner should modify")
	}
	// creator can modify when OwnerId empty
	if !authz.CanModifyRecord(actor, "", "u1", "Account", modifyAll) {
		t.Fatal("creator should modify unowned")
	}
	// creator cannot modify when owned by someone else
	if authz.CanModifyRecord(actor, "other", "u1", "Account", modifyAll) {
		t.Fatal("creator should not modify after ownership transfer")
	}
	// modifyAll on Contact
	if !authz.CanModifyRecord(actor, "other", "creator", "Contact", modifyAll) {
		t.Fatal("modifyAll should allow")
	}
	other := "other-user"
	if err := authz.AssertOwnerIDWritable(actor, "Account", &other, modifyAll); err == nil {
		t.Fatal("expected OwnerId reassignment forbidden without modifyAll")
	}
	self := "u1"
	if err := authz.AssertOwnerIDWritable(actor, "Account", &self, modifyAll); err != nil {
		t.Fatal(err)
	}
	if err := authz.AssertOwnerIDWritable(actor, "Contact", &other, modifyAll); err != nil {
		t.Fatal(err)
	}
}
