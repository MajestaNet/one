package authz

import (
	"context"
	"fmt"
)

// System capability strings stored on permission_sets.system_permissions.
// Canonical product catalog (multi-cap per permission set; OR-union across assignments).
const (
	CapIdentityUsers        = "identity.users"
	CapIdentityIntegrations = "identity.integrations"
	CapAuthzManage          = "authz.manage"
	CapMetadataBuild        = "metadata.build"
	CapDeployPromote        = "deploy.promote"
	CapGovernNetwork        = "govern.network"
	CapGovernAgents         = "govern.agents"
	CapGovernAudit          = "govern.audit"
	CapDebugRead            = "debug.read"
	CapDebugTrace           = "debug.trace"

	// Control IDE chrome (section + tool). Require matching Role family scopes for API calls.
	CapIDEOperate  = "ide.operate"
	CapIDERun      = "ide.run"
	CapIDEBuild    = "ide.build"
	CapIDEShip     = "ide.ship"
	CapIDEGovern   = "ide.govern"
	CapIDESettings = "ide.settings"

	CapIDEOperateQuery    = "ide.operate.query"
	CapIDEOperateMonitor  = "ide.operate.monitor"
	CapIDEOperateExplorer = "ide.operate.explorer"
	CapIDEOperateCanvases = "ide.operate.canvases"

	CapIDERunTools = "ide.run.tools"

	CapIDEBuildObjects     = "ide.build.objects"
	CapIDEBuildPackages    = "ide.build.packages"
	CapIDEBuildAgentSpecs  = "ide.build.agentSpecs"
	CapIDEBuildCanvasSpecs = "ide.build.canvasSpecs"
	CapIDEBuildTools       = "ide.build.tools"
	CapIDEBuildRepo        = "ide.build.repo"

	CapIDEShipDeploy = "ide.ship.deploy"
	CapIDEShipEnv    = "ide.ship.env"

	CapIDESettingsAccount   = "ide.settings.account"
	CapIDESettingsHosting   = "ide.settings.hosting"
	CapIDESettingsInference = "ide.settings.inference"
	CapIDESettingsEnv       = "ide.settings.env"

	CapIDEGovernUsers        = "ide.govern.users"
	CapIDEGovernIntegrations = "ide.govern.integrations"
	CapIDEGovernExperiences  = "ide.govern.experiences"
	CapIDEGovernInstallAuth  = "ide.govern.installAuth"
	CapIDEGovernPermissions  = "ide.govern.permissions"
	CapIDEGovernEnv          = "ide.govern.env"

	// Legacy aliases — still accepted on write and imply canonical caps on check.
	CapMetadataCustomize   = "metadata.customize"
	CapMetadataPackages    = "metadata.packages"
	CapMetadataAssignAuthz = "metadata.assignAuthz"
	CapMetadataNetwork     = "metadata.network"
	CapIdentityManage      = "identity.manage"
	CapAgentsApprove       = "agents.approve"
)

// IDEModeCapabilities are the Control IDE section caps used by AllIDECapabilities
// and the Admin seed. Settings is included here so it is granted with the other
// sections even though it opens from the footer instead of the mode launcher.
var IDEModeCapabilities = []string{
	CapIDEOperate, CapIDERun, CapIDEBuild, CapIDEShip, CapIDEGovern, CapIDESettings,
}

// IDEOperateToolCapabilities are Operate rail tools.
var IDEOperateToolCapabilities = []string{
	CapIDEOperateQuery, CapIDEOperateMonitor, CapIDEOperateExplorer, CapIDEOperateCanvases,
}

// IDERunToolCapabilities are Run rail tools.
var IDERunToolCapabilities = []string{
	CapIDERunTools,
}

// IDEBuildToolCapabilities are Build rail tools.
var IDEBuildToolCapabilities = []string{
	CapIDEBuildObjects, CapIDEBuildPackages, CapIDEBuildAgentSpecs, CapIDEBuildCanvasSpecs, CapIDEBuildTools, CapIDEBuildRepo,
}

// IDEShipToolCapabilities are Ship rail tools.
var IDEShipToolCapabilities = []string{
	CapIDEShipDeploy, CapIDEShipEnv,
}

// IDESettingsToolCapabilities are Settings rail tools.
var IDESettingsToolCapabilities = []string{
	CapIDESettingsAccount, CapIDESettingsHosting, CapIDESettingsInference, CapIDESettingsEnv,
}

// IDEGovernToolCapabilities are Govern rail tools.
var IDEGovernToolCapabilities = []string{
	CapIDEGovernUsers, CapIDEGovernIntegrations, CapIDEGovernExperiences,
	CapIDEGovernInstallAuth, CapIDEGovernPermissions, CapIDEGovernEnv,
}

// AllIDECapabilities returns every ide.* section and tool capability.
func AllIDECapabilities() []string {
	out := make([]string, 0, len(IDEModeCapabilities)+len(IDEOperateToolCapabilities)+
		len(IDERunToolCapabilities)+len(IDEBuildToolCapabilities)+len(IDEShipToolCapabilities)+
		len(IDESettingsToolCapabilities)+len(IDEGovernToolCapabilities))
	out = append(out, IDEModeCapabilities...)
	out = append(out, IDEOperateToolCapabilities...)
	out = append(out, IDERunToolCapabilities...)
	out = append(out, IDEBuildToolCapabilities...)
	out = append(out, IDEShipToolCapabilities...)
	out = append(out, IDESettingsToolCapabilities...)
	out = append(out, IDEGovernToolCapabilities...)
	return out
}

// OperateIDECapabilities is ide.operate + Operate tools + Run (business users) for the Operate pack.
func OperateIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDEOperateToolCapabilities)+1+len(IDERunToolCapabilities))
	out = append(out, CapIDEOperate)
	out = append(out, IDEOperateToolCapabilities...)
	out = append(out, CapIDERun)
	out = append(out, IDERunToolCapabilities...)
	return out
}

// RunIDECapabilities is ide.run + Run tools.
func RunIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDERunToolCapabilities))
	out = append(out, CapIDERun)
	out = append(out, IDERunToolCapabilities...)
	return out
}

// BuildIDECapabilities is ide.build + Build tools.
func BuildIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDEBuildToolCapabilities))
	out = append(out, CapIDEBuild)
	out = append(out, IDEBuildToolCapabilities...)
	return out
}

// ShipIDECapabilities is ide.ship + Ship tools.
func ShipIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDEShipToolCapabilities))
	out = append(out, CapIDEShip)
	out = append(out, IDEShipToolCapabilities...)
	return out
}

// SettingsIDECapabilities is ide.settings + Settings tools.
func SettingsIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDESettingsToolCapabilities))
	out = append(out, CapIDESettings)
	out = append(out, IDESettingsToolCapabilities...)
	return out
}

// GovernIDECapabilities is ide.govern + Govern tools.
func GovernIDECapabilities() []string {
	out := make([]string, 0, 1+len(IDEGovernToolCapabilities))
	out = append(out, CapIDEGovern)
	out = append(out, IDEGovernToolCapabilities...)
	return out
}

// KnownCapabilities is the allowlist for system_permissions values (canonical + legacy).
var KnownCapabilities = []string{
	CapIdentityUsers,
	CapIdentityIntegrations,
	CapAuthzManage,
	CapMetadataBuild,
	CapDeployPromote,
	CapGovernNetwork,
	CapGovernAgents,
	CapGovernAudit,
	CapDebugRead,
	CapDebugTrace,
	// ide.*
	CapIDEOperate, CapIDERun, CapIDEBuild, CapIDEShip, CapIDEGovern, CapIDESettings,
	CapIDEOperateQuery, CapIDEOperateMonitor, CapIDEOperateExplorer, CapIDEOperateCanvases,
	CapIDERunTools,
	CapIDEBuildObjects, CapIDEBuildPackages, CapIDEBuildAgentSpecs, CapIDEBuildCanvasSpecs, CapIDEBuildTools, CapIDEBuildRepo,
	CapIDEShipDeploy, CapIDEShipEnv,
	CapIDESettingsAccount, CapIDESettingsHosting, CapIDESettingsInference, CapIDESettingsEnv,
	CapIDEGovernUsers, CapIDEGovernIntegrations, CapIDEGovernExperiences,
	CapIDEGovernInstallAuth, CapIDEGovernPermissions, CapIDEGovernEnv,
	// legacy
	CapMetadataCustomize,
	CapMetadataPackages,
	CapMetadataAssignAuthz,
	CapMetadataNetwork,
	CapIdentityManage,
	CapAgentsApprove,
}

// CanonicalCapabilities is the Admin / product catalog (no legacy aliases).
// Includes API caps, debug, and every Control IDE section/tool capability.
var CanonicalCapabilities = append([]string{
	CapIdentityUsers,
	CapIdentityIntegrations,
	CapAuthzManage,
	CapMetadataBuild,
	CapDeployPromote,
	CapGovernNetwork,
	CapGovernAgents,
	CapGovernAudit,
	CapDebugRead,
	CapDebugTrace,
}, AllIDECapabilities()...)

// AllSystemCapabilities returns the full canonical capability set (Admin PS seed).
func AllSystemCapabilities() []string {
	out := make([]string, len(CanonicalCapabilities))
	copy(out, CanonicalCapabilities)
	return out
}

// MergeCapabilityLists unions capability slices (deduped, order of first occurrence).
func MergeCapabilityLists(lists ...[]string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, list := range lists {
		for _, c := range list {
			if c == "" {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

// ValidateSystemPermissions returns an error if any capability is unknown.
func ValidateSystemPermissions(caps []string) error {
	known := map[string]struct{}{}
	for _, c := range KnownCapabilities {
		known[c] = struct{}{}
	}
	for _, c := range caps {
		if c == "" {
			continue
		}
		if _, ok := known[c]; !ok {
			return fmt.Errorf("unknown system permission %q", c)
		}
	}
	return nil
}

// NormalizeCapabilitySet expands legacy aliases into canonical caps and dedupes.
func NormalizeCapabilitySet(caps []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(c string) {
		if c == "" {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	for _, c := range caps {
		switch c {
		case CapIdentityManage:
			add(CapIdentityUsers)
			add(CapIdentityIntegrations)
		case CapMetadataCustomize, CapMetadataPackages:
			add(CapMetadataBuild)
		case CapMetadataAssignAuthz:
			add(CapAuthzManage)
		case CapMetadataNetwork:
			add(CapGovernNetwork)
		case CapAgentsApprove:
			add(CapGovernAgents)
		default:
			add(c)
		}
	}
	return out
}

// CapabilitySatisfied reports whether held (normalized) caps grant required.
func CapabilitySatisfied(held []string, required string) bool {
	if required == "" {
		return false
	}
	norm := NormalizeCapabilitySet(held)
	reqNorm := NormalizeCapabilitySet([]string{required})
	have := map[string]struct{}{}
	for _, c := range norm {
		have[c] = struct{}{}
	}
	// Legacy identity.manage as required: need both user + integration caps.
	if required == CapIdentityManage {
		_, u := have[CapIdentityUsers]
		_, i := have[CapIdentityIntegrations]
		return u && i
	}
	for _, r := range reqNorm {
		if _, ok := have[r]; !ok {
			return false
		}
	}
	return len(reqNorm) > 0
}

// SystemPermissionStore loads system_permissions for permission set ids.
type SystemPermissionStore interface {
	ListSystemPermissions(ctx context.Context, permissionSetIDs []string) ([]string, error)
}

// SystemAuthz evaluates system capabilities (Metadata / Deploy / govern / identity / IDE).
type SystemAuthz struct {
	Store SystemPermissionStore
}

// HasAdminPrivilege reports whether the actor has admin (DB flag or Role admin scope).
func HasAdminPrivilege(actor *Actor) bool {
	if actor == nil {
		return false
	}
	return actor.IsAdmin
}

// AssertCapability allows the capability or returns ErrForbidden.
func (a *SystemAuthz) AssertCapability(ctx context.Context, actor *Actor, capability string) error {
	if actor == nil {
		return fmt.Errorf("%w: no actor", ErrForbidden)
	}
	if HasAdminPrivilege(actor) {
		return nil
	}
	if capability == "" {
		return fmt.Errorf("%w: missing capability", ErrForbidden)
	}
	if len(actor.PermissionSetIDs) == 0 {
		return fmt.Errorf("%w: capability %s required", ErrForbidden, capability)
	}
	if a == nil || a.Store == nil {
		return fmt.Errorf("%w: system permission store not configured", ErrForbidden)
	}
	caps, err := a.Store.ListSystemPermissions(ctx, actor.PermissionSetIDs)
	if err != nil {
		return err
	}
	if CapabilitySatisfied(caps, capability) {
		return nil
	}
	return fmt.Errorf("%w: capability %s required", ErrForbidden, capability)
}

// EffectiveCapabilities returns the normalized union of system caps for the actor.
func (a *SystemAuthz) EffectiveCapabilities(ctx context.Context, actor *Actor) ([]string, error) {
	if actor == nil {
		return nil, nil
	}
	if HasAdminPrivilege(actor) {
		return AllSystemCapabilities(), nil
	}
	if len(actor.PermissionSetIDs) == 0 || a == nil || a.Store == nil {
		return nil, nil
	}
	caps, err := a.Store.ListSystemPermissions(ctx, actor.PermissionSetIDs)
	if err != nil {
		return nil, err
	}
	return NormalizeCapabilitySet(caps), nil
}
