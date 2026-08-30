package authz_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type memAutoStore struct {
	perms map[string][]authz.AutomationPermission // psID -> rows
	all   map[string]bool
}

func (m *memAutoStore) ListByPermissionSets(_ context.Context, ids []string) ([]authz.AutomationPermission, error) {
	var out []authz.AutomationPermission
	for _, id := range ids {
		out = append(out, m.perms[id]...)
	}
	return out, nil
}

func (m *memAutoStore) AnyAllAutomations(_ context.Context, ids []string) (bool, error) {
	for _, id := range ids {
		if m.all[id] {
			return true, nil
		}
	}
	return false, nil
}

func TestActorCanRunAutomation(t *testing.T) {
	store := &memAutoStore{
		perms: map[string][]authz.AutomationPermission{
			"ps1": {{PermissionSetID: "ps1", AutomationAPIName: "CreateOpp", CanRun: true}},
			"ps2": {{PermissionSetID: "ps2", AutomationAPIName: "CreateOpp", CanRun: false}},
		},
		all: map[string]bool{},
	}
	az := &authz.AutomationAuthz{Store: store}

	ok, err := az.ActorCanRunAutomation(context.Background(), &authz.Actor{
		ID: "u1", PermissionSetIDs: []string{"ps1"},
	}, "CreateOpp")
	if err != nil || !ok {
		t.Fatalf("expected grant via ps1, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanRunAutomation(context.Background(), &authz.Actor{
		ID: "u2", PermissionSetIDs: []string{"ps2"},
	}, "CreateOpp")
	if err != nil || ok {
		t.Fatalf("expected deny via ps2 stub, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanRunAutomation(context.Background(), &authz.Actor{
		ID: "u3", PermissionSetIDs: []string{"ps2"},
	}, "OtherAuto")
	if err != nil || ok {
		t.Fatalf("expected deny for unknown automation, ok=%v err=%v", ok, err)
	}

	store.all["ps-all"] = true
	ok, err = az.ActorCanRunAutomation(context.Background(), &authz.Actor{
		ID: "u4", PermissionSetIDs: []string{"ps-all"},
	}, "Anything")
	if err != nil || !ok {
		t.Fatalf("expected allAutomations grant, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanRunAutomation(context.Background(), &authz.Actor{
		ID: "admin", IsAdmin: true,
	}, "Anything")
	if err != nil || !ok {
		t.Fatalf("expected IsAdmin grant, ok=%v err=%v", ok, err)
	}

	if err := az.AssertCanRunAutomation(context.Background(), &authz.Actor{
		ID: "u2", PermissionSetIDs: []string{"ps2"},
	}, "CreateOpp"); err == nil {
		t.Fatal("expected AssertCanRunAutomation to fail")
	}
}
