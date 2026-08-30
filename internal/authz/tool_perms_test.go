package authz_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type memToolStore struct {
	perms map[string][]authz.ToolPermission // psID -> rows
	all   map[string]bool
}

func (m *memToolStore) ListByPermissionSets(_ context.Context, ids []string) ([]authz.ToolPermission, error) {
	var out []authz.ToolPermission
	for _, id := range ids {
		out = append(out, m.perms[id]...)
	}
	return out, nil
}

func (m *memToolStore) AnyAllTools(_ context.Context, ids []string) (bool, error) {
	for _, id := range ids {
		if m.all[id] {
			return true, nil
		}
	}
	return false, nil
}

func TestActorCanOpenTool(t *testing.T) {
	store := &memToolStore{
		perms: map[string][]authz.ToolPermission{
			"ps1": {{PermissionSetID: "ps1", ToolAPIName: "SalesHome", CanOpen: true, CanInteract: true}},
			"ps2": {{PermissionSetID: "ps2", ToolAPIName: "SalesHome", CanOpen: false}},
			"ps3": {{PermissionSetID: "ps3", ToolAPIName: "SalesHome", CanPublish: true}},
		},
		all: map[string]bool{},
	}
	az := &authz.ToolAuthz{Store: store}

	ok, err := az.ActorCanOpenTool(context.Background(), &authz.Actor{
		ID: "u1", PermissionSetIDs: []string{"ps1"},
	}, "SalesHome")
	if err != nil || !ok {
		t.Fatalf("expected grant via ps1, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanOpenTool(context.Background(), &authz.Actor{
		ID: "u2", PermissionSetIDs: []string{"ps2"},
	}, "SalesHome")
	if err != nil || ok {
		t.Fatalf("expected deny via ps2 stub, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanOpenTool(context.Background(), &authz.Actor{
		ID: "u3", PermissionSetIDs: []string{"ps2"},
	}, "OtherTool")
	if err != nil || ok {
		t.Fatalf("expected deny for unknown tool, ok=%v err=%v", ok, err)
	}

	store.all["ps-all"] = true
	ok, err = az.ActorCanOpenTool(context.Background(), &authz.Actor{
		ID: "u4", PermissionSetIDs: []string{"ps-all"},
	}, "Anything")
	if err != nil || !ok {
		t.Fatalf("expected allTools grant, ok=%v err=%v", ok, err)
	}

	ok, err = az.ActorCanOpenTool(context.Background(), &authz.Actor{
		ID: "admin", IsAdmin: true,
	}, "Anything")
	if err != nil || !ok {
		t.Fatalf("expected IsAdmin grant, ok=%v err=%v", ok, err)
	}

	if err := az.AssertCanOpenTool(context.Background(), &authz.Actor{
		ID: "u2", PermissionSetIDs: []string{"ps2"},
	}, "SalesHome"); err == nil {
		t.Fatal("expected AssertCanOpenTool to fail")
	}

	access, err := az.ActorToolAccess(context.Background(), &authz.Actor{
		ID: "union", PermissionSetIDs: []string{"ps1", "ps3"},
	}, "SalesHome")
	if err != nil || !access.CanOpen || !access.CanInteract || !access.CanPublish || access.CanModify {
		t.Fatalf("unexpected permission OR-union: %+v err=%v", access, err)
	}
}
