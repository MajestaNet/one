package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

type memSysStore struct {
	byPS map[string][]string
}

func (m *memSysStore) ListSystemPermissions(_ context.Context, ids []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		for _, c := range m.byPS[id] {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out, nil
}

func TestAssertCapability(t *testing.T) {
	store := &memSysStore{byPS: map[string][]string{
		"ps1": {authz.CapMetadataBuild},
	}}
	az := &authz.SystemAuthz{Store: store}

	builder := &authz.Actor{
		ID:               "u1",
		PrincipalType:    "user",
		PermissionSetIDs: []string{"ps1"},
		Scopes:           []authz.Scope{authz.ScopeMetadata, authz.ScopeClient},
	}
	if err := az.AssertCapability(context.Background(), builder, authz.CapMetadataBuild); err != nil {
		t.Fatal(err)
	}
	// Legacy required name still satisfied by canonical hold.
	if err := az.AssertCapability(context.Background(), builder, authz.CapMetadataCustomize); err != nil {
		t.Fatal(err)
	}
	if err := az.AssertCapability(context.Background(), builder, authz.CapGovernNetwork); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	legacyPS := &memSysStore{byPS: map[string][]string{
		"ps-legacy": {authz.CapMetadataCustomize},
	}}
	azLegacy := &authz.SystemAuthz{Store: legacyPS}
	legacyActor := &authz.Actor{ID: "u2", PermissionSetIDs: []string{"ps-legacy"}}
	if err := azLegacy.AssertCapability(context.Background(), legacyActor, authz.CapMetadataBuild); err != nil {
		t.Fatal(err)
	}

	admin := &authz.Actor{ID: "admin", IsAdmin: true}
	if err := az.AssertCapability(context.Background(), admin, authz.CapDeployPromote); err != nil {
		t.Fatal(err)
	}

	noPS := &authz.Actor{ID: "x", Scopes: []authz.Scope{authz.ScopeMetadata}}
	if err := az.AssertCapability(context.Background(), noPS, authz.CapMetadataBuild); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestValidateSystemPermissions(t *testing.T) {
	if err := authz.ValidateSystemPermissions([]string{authz.CapMetadataBuild, authz.CapGovernAgents}); err != nil {
		t.Fatal(err)
	}
	if err := authz.ValidateSystemPermissions([]string{authz.CapMetadataCustomize}); err != nil {
		t.Fatal(err) // legacy still valid
	}
	if err := authz.ValidateSystemPermissions([]string{
		authz.CapIDEOperate,
		authz.CapIDEOperateQuery,
		authz.CapIDESettings,
		authz.CapIDESettingsAccount,
		authz.CapIDESettingsHosting,
		authz.CapIDESettingsInference,
		authz.CapDebugRead,
	}); err != nil {
		t.Fatal(err)
	}
	if err := authz.ValidateSystemPermissions([]string{"nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestIDECapabilitiesInAdminCatalog(t *testing.T) {
	all := authz.AllSystemCapabilities()
	have := map[string]bool{}
	for _, c := range all {
		have[c] = true
	}
	for _, want := range []string{
		authz.CapIDEOperate, authz.CapIDERun, authz.CapIDEBuild, authz.CapIDEShip, authz.CapIDEGovern,
		authz.CapIDESettings,
		authz.CapIDEOperateQuery, authz.CapIDERunTools, authz.CapIDESettingsAccount,
		authz.CapIDESettingsHosting, authz.CapIDEGovernEnv, authz.CapDebugRead, authz.CapDebugTrace,
	} {
		if !have[want] {
			t.Fatalf("Admin catalog missing %s", want)
		}
	}
}

func TestAllIDECapabilitiesIncludesSettings(t *testing.T) {
	all := authz.AllIDECapabilities()
	for _, want := range []string{
		authz.CapIDESettings,
		authz.CapIDESettingsAccount,
		authz.CapIDESettingsHosting,
		authz.CapIDESettingsInference,
		authz.CapIDESettingsEnv,
	} {
		if !containsString(all, want) {
			t.Fatalf("AllIDECapabilities missing %s in %#v", want, all)
		}
	}
}

func TestSettingsIDECapabilities(t *testing.T) {
	got := authz.SettingsIDECapabilities()
	want := []string{
		authz.CapIDESettings,
		authz.CapIDESettingsAccount,
		authz.CapIDESettingsHosting,
		authz.CapIDESettingsInference,
		authz.CapIDESettingsEnv,
	}
	if len(got) != len(want) {
		t.Fatalf("SettingsIDECapabilities length=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SettingsIDECapabilities[%d]=%s want %s (all %#v)", i, got[i], want[i], got)
		}
	}
}

func TestEffectiveCapabilitiesMultiPSIncludesIDE(t *testing.T) {
	store := &memSysStore{byPS: map[string][]string{
		"a": {authz.CapIDEOperate, authz.CapIDEOperateQuery},
		"b": {authz.CapIDEOperateMonitor, authz.CapIdentityUsers},
	}}
	az := &authz.SystemAuthz{Store: store}
	actor := &authz.Actor{ID: "u", PermissionSetIDs: []string{"a", "b"}}
	caps, err := az.EffectiveCapabilities(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{authz.CapIDEOperate, authz.CapIDEOperateQuery, authz.CapIDEOperateMonitor, authz.CapIdentityUsers} {
		if !authz.CapabilitySatisfied(caps, want) {
			t.Fatalf("expected multi-PS OR to include %s; got %#v", want, caps)
		}
	}
}

func TestNormalizeAndMultiCap(t *testing.T) {
	got := authz.NormalizeCapabilitySet([]string{
		authz.CapIdentityManage,
		authz.CapMetadataCustomize,
		authz.CapIdentityUsers,
	})
	have := map[string]bool{}
	for _, c := range got {
		have[c] = true
	}
	for _, want := range []string{authz.CapIdentityUsers, authz.CapIdentityIntegrations, authz.CapMetadataBuild} {
		if !have[want] {
			t.Fatalf("missing %s in %#v", want, got)
		}
	}
	if !authz.CapabilitySatisfied([]string{authz.CapIdentityManage}, authz.CapIdentityUsers) {
		t.Fatal("legacy identity.manage should satisfy identity.users")
	}
	if authz.CapabilitySatisfied([]string{authz.CapIdentityUsers}, authz.CapIdentityManage) {
		t.Fatal("identity.users alone should not satisfy legacy identity.manage")
	}
}

func TestEffectiveCapabilities(t *testing.T) {
	store := &memSysStore{byPS: map[string][]string{
		"a": {authz.CapIdentityUsers},
		"b": {authz.CapAuthzManage, authz.CapGovernAgents},
	}}
	az := &authz.SystemAuthz{Store: store}
	actor := &authz.Actor{ID: "u", PermissionSetIDs: []string{"a", "b"}}
	caps, err := az.EffectiveCapabilities(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !authz.CapabilitySatisfied(caps, authz.CapIdentityUsers) || !authz.CapabilitySatisfied(caps, authz.CapAuthzManage) {
		t.Fatalf("unexpected caps %#v", caps)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
