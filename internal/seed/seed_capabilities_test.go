package seed

import (
	"testing"

	"github.com/MajestaNet/ide/internal/authz"
)

func TestDeployPermissionSetsIncludeSettingsCapabilities(t *testing.T) {
	defs := capabilityPermissionSetDefs()
	for _, apiName := range []string{"Deploy", "DeployPromote"} {
		def := findCapabilityPermissionSetDef(t, defs, apiName)
		for _, want := range []string{
			authz.CapDeployPromote,
			authz.CapIDEShip,
			authz.CapIDEShipDeploy,
			authz.CapIDEShipEnv,
			authz.CapIDESettings,
			authz.CapIDESettingsAccount,
			authz.CapIDESettingsHosting,
		} {
			if !containsCapability(def.Caps, want) {
				t.Fatalf("%s caps missing %s in %#v", apiName, want, def.Caps)
			}
		}
	}
}

func TestNonDeployPermissionSetsDoNotIncludeHosting(t *testing.T) {
	defs := capabilityPermissionSetDefs()
	for _, apiName := range []string{"ManageUsers", "ManageIntegrations", "Operate"} {
		def := findCapabilityPermissionSetDef(t, defs, apiName)
		if containsCapability(def.Caps, authz.CapIDESettingsHosting) {
			t.Fatalf("%s caps should not include %s: %#v", apiName, authz.CapIDESettingsHosting, def.Caps)
		}
	}
}

func findCapabilityPermissionSetDef(t *testing.T, defs []capabilityPermissionSetDef, apiName string) capabilityPermissionSetDef {
	t.Helper()
	for _, def := range defs {
		if def.APIName == apiName {
			return def
		}
	}
	t.Fatalf("permission set def %s not found", apiName)
	return capabilityPermissionSetDef{}
}

func containsCapability(caps []string, want string) bool {
	for _, cap := range caps {
		if cap == want {
			return true
		}
	}
	return false
}
