package mcp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/mcp"
)

func TestGetObjectMetadataRequiresMetadataScope(t *testing.T) {
	actor := &authz.Actor{
		ID:     "a1",
		Scopes: []authz.Scope{authz.ScopeClient},
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "get_object_metadata", map[string]any{
		"apiName": "Account",
	})
	if !errors.Is(err, mcp.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateRecordRequiresClientScope(t *testing.T) {
	actor := &authz.Actor{
		ID:     "a1",
		Scopes: []authz.Scope{authz.ScopeMetadata},
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "create_record", map[string]any{
		"object": "Account",
		"data":   map[string]any{"Name": "x"},
	})
	if !errors.Is(err, mcp.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestListToolsIncludesDescribeAndCreate(t *testing.T) {
	tools := mcp.ListTools()
	seen := map[string]bool{}
	for _, tool := range tools {
		seen[tool.Name] = true
	}
	for _, name := range []string{
		"describe_object", "create_record", "get_object_metadata", "list_agent_specs", "search",
		"invoke_action", "invoke_skill", "list_objects_metadata", "upsert_object", "upsert_field",
		"org_validate", "org_deploy", "pack", "org_retrieve", "install_version",
	} {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestSearchRequiresClientScope(t *testing.T) {
	actor := &authz.Actor{
		ID:     "a1",
		Scopes: []authz.Scope{authz.ScopeMetadata},
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "search", map[string]any{
		"q": "acme",
	})
	if !errors.Is(err, mcp.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestOrgValidateRequiresDeployScope(t *testing.T) {
	actor := &authz.Actor{
		ID:     "a1",
		Scopes: []authz.Scope{authz.ScopeClient},
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "org_validate", map[string]any{
		"artifact": map[string]any{"manifestVersion": 1},
	})
	if !errors.Is(err, mcp.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestInvokeSkillDeniedWhenNotInAllowlist(t *testing.T) {
	actor := &authz.Actor{
		ID:            "a1",
		PrincipalType: "agent",
		Scopes:        []authz.Scope{authz.ScopeClient},
		IsAdmin:       true,
	}
	_, err := mcp.CallTool(context.Background(), mcp.Deps{}, actor, "invoke_skill", map[string]any{
		"apiName": "SomeSkill",
	})
	if !errors.Is(err, mcp.ErrForbidden) {
		t.Fatalf("expected ErrForbidden without playbookApiName, got %v", err)
	}
}

func TestInstallVersionRequiresActor(t *testing.T) {
	_, err := mcp.CallTool(context.Background(), mcp.Deps{Version: "0.1.0", ProductVersion: "0.1.0"}, nil, "install_version", nil)
	if !errors.Is(err, mcp.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	out, err := mcp.CallTool(context.Background(), mcp.Deps{Version: "0.1.0", ProductVersion: "dev"}, &authz.Actor{
		ID: "u1", Scopes: []authz.Scope{authz.ScopeClient},
	}, "install_version", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := out.(map[string]any)
	if m["runtime"] != "go" || m["version"] != "0.1.0" {
		t.Fatalf("version payload=%v", out)
	}
}
