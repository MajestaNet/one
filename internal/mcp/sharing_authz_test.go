package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/mcp"
)

type allowAllObjectStore struct{}

func (allowAllObjectStore) ListByPermissionSets(context.Context, []string) ([]authz.ObjectPermission, error) {
	return []authz.ObjectPermission{{
		ObjectAPIName: "Account",
		CanCreate:     true,
		CanRead:       true,
		CanUpdate:     true,
		CanDelete:     true,
	}}, nil
}

type denyObjectStore struct{}

func (denyObjectStore) ListByPermissionSets(context.Context, []string) ([]authz.ObjectPermission, error) {
	return nil, nil
}

func TestQueryRequiresObjectRead(t *testing.T) {
	actor := &authz.Actor{
		ID:               "u1",
		Scopes:           []authz.Scope{authz.ScopeClient},
		PermissionSetIDs: []string{"ps1"},
	}
	deps := mcp.Deps{
		ObjectAz: &authz.ObjectAuthz{Store: denyObjectStore{}},
		Data:     &dataengine.Service{},
	}
	_, err := mcp.CallTool(context.Background(), deps, actor, "query", map[string]any{
		"object": "Account",
	})
	if err == nil || !errors.Is(err, mcp.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestQueryRequiresObjectArgument(t *testing.T) {
	actor := &authz.Actor{ID: "u1", Scopes: []authz.Scope{authz.ScopeClient}}
	deps := mcp.Deps{
		ObjectAz: &authz.ObjectAuthz{Store: allowAllObjectStore{}},
		Data:     &dataengine.Service{},
	}
	_, err := mcp.CallTool(context.Background(), deps, actor, "query", map[string]any{})
	if err == nil {
		t.Fatal("expected object required error")
	}
}

func TestGetRecordUnavailableWithoutData(t *testing.T) {
	actor := &authz.Actor{ID: "u1", Scopes: []authz.Scope{authz.ScopeClient}, PermissionSetIDs: []string{"ps1"}}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{
		ObjectAz: &authz.ObjectAuthz{Store: allowAllObjectStore{}},
	}, actor, "get_record", map[string]any{"object": "Account", "id": "x"})
	if err == nil {
		t.Fatal("expected unavailable without Data")
	}
}

func TestUpdateRecordRequiresObjectUpdate(t *testing.T) {
	actor := &authz.Actor{
		ID:               "u1",
		Scopes:           []authz.Scope{authz.ScopeClient},
		PermissionSetIDs: []string{"ps1"},
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{
		ObjectAz: &authz.ObjectAuthz{Store: denyObjectStore{}},
		Data:     &dataengine.Service{},
	}, actor, "update_record", map[string]any{
		"object": "Account", "id": "00000000-0000-4000-8000-000000000099",
		"data": map[string]any{"Name": "x"},
	})
	if err == nil || !errors.Is(err, mcp.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestListToolsIncludesQueryAndUpdate(t *testing.T) {
	tools := mcp.ListTools()
	raw, _ := json.Marshal(tools)
	s := string(raw)
	for _, name := range []string{"query", "get_record", "update_record", "search"} {
		found := false
		for _, tool := range tools {
			if tool.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing tool %s in %s", name, s)
		}
	}
}
