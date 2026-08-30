package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/identity"
	"github.com/MajestaNet/ide/internal/integration"
	"github.com/MajestaNet/ide/internal/metadata"
)

// Options configures bootstrap/seed.
type Options struct {
	OwnerID        string
	FeatureFlags   []string
	AutoSeed       bool
	// SkipControlIDE skips EnsureControlIDE even when AutoSeed is on (SEED_CONTROL_IDE=0).
	SkipControlIDE bool
	Identity       identity.Backend // optional; memory/adapter write-through for managed integrations
	EncryptionKey  string
	IdentityIssuer string // OIDC issuer for identity_links when an adapter is enabled
}

// Bootstrap ensures admin user, system roles, Admin permission set, feature flags,
// and the managed core data model (when AutoSeed).
func Bootstrap(ctx context.Context, pool *db.Pool, meta *metadata.Service, opts Options) error {
	if opts.OwnerID == "" {
		opts.OwnerID = "00000000-0000-4000-8000-000000000001"
	}
	slog.Info("seed: ensuring bootstrap admin and system roles")
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, opts.OwnerID, "admin@one.local", "Majesta One Admin"); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	if err := store.EnsureSystemRoles(ctx); err != nil {
		return err
	}
	if err := store.EnsureUserHasRole(ctx, opts.OwnerID, "SystemAdmin"); err != nil {
		return fmt.Errorf("assign SystemAdmin role: %w", err)
	}

	if _, err := ensureAdminPermissionSet(ctx, pool, opts.OwnerID); err != nil {
		return err
	}
	if err := ensureCapabilityPermissionSets(ctx, pool); err != nil {
		return err
	}

	flags := opts.FeatureFlags
	if len(flags) == 0 {
		flags = []string{"agents"}
	}
	for _, key := range flags {
		if _, err := pool.Exec(ctx, `
INSERT INTO feature_flags (key, enabled) VALUES ($1, true)
ON CONFLICT (key) DO UPDATE SET enabled = true`, key); err != nil {
			return fmt.Errorf("feature flag %s: %w", key, err)
		}
	}

	if !opts.AutoSeed {
		slog.Info("seed: AUTO_SEED disabled; skipping packages")
		return nil
	}

	slog.Info("seed: installing managed packages")
	// Core data model (User identity + Account + Contact) always migrates when AUTO_SEED is on.
	if err := InstallCore(ctx, meta); err != nil {
		return fmt.Errorf("core package: %w", err)
	}
	slog.Info("seed: core package migrated", "version", CorePackageVersion)

	if err := InstallAgentsStarter(ctx, meta); err != nil {
		return fmt.Errorf("agents_starter package: %w", err)
	}
	slog.Info("seed: agents_starter package migrated", "version", AgentsStarterPackageVersion)

	if err := MigrateEnabledModules(ctx, meta); err != nil {
		return fmt.Errorf("enabled modules: %w", err)
	}

	if err := EnsurePlatformSmokeSuite(ctx, pool); err != nil {
		return fmt.Errorf("platform smoke suite: %w", err)
	}
	slog.Info("seed: PlatformSmoke suite ensured")

	intSvc := &integration.Service{
		Pool:           pool,
		Identity:       opts.Identity,
		EncryptionKey:  opts.EncryptionKey,
		IdentityIssuer: opts.IdentityIssuer,
	}
	if opts.SkipControlIDE {
		slog.Info("seed: SEED_CONTROL_IDE disabled; skipping managed Control IDE integration")
	} else if err := intSvc.EnsureControlIDE(ctx); err != nil {
		return fmt.Errorf("control ide integration: %w", err)
	} else {
		slog.Info("seed: managed integration ensured", "apiName", integration.APINameControlIDE)
	}

	objs, err := meta.ListObjects(ctx)
	if err != nil {
		return err
	}
	for _, o := range objs {
		if err := db.EnsureObjectInDataAccessCatalog(ctx, pool, o.APIName); err != nil {
			return fmt.Errorf("object data access %s: %w", o.APIName, err)
		}
		fields, ferr := meta.GetFields(ctx, o.APIName)
		if ferr != nil {
			return ferr
		}
		for _, f := range fields {
			if err := db.EnsureFieldInDataAccessCatalog(ctx, pool, f.ObjectAPIName, f.APIName); err != nil {
				return fmt.Errorf("field data access %s.%s: %w", f.ObjectAPIName, f.APIName, err)
			}
		}
	}
	meta.EnqueueSearchReindex(ctx, "")

	if err := db.EnsureUserObjectDescribeAccess(ctx, pool); err != nil {
		return fmt.Errorf("user describe access: %w", err)
	}
	if err := db.EnsureAdminAllAutomations(ctx, pool); err != nil {
		return fmt.Errorf("admin all automations: %w", err)
	}
	if err := db.EnsureAdminAllTools(ctx, pool); err != nil {
		return fmt.Errorf("admin all tools: %w", err)
	}
	if err := db.EnsureAdminFullDataAccess(ctx, pool); err != nil {
		return fmt.Errorf("admin full data access: %w", err)
	}
	if err := db.SyncSystemAdminUserFlags(ctx, pool); err != nil {
		return fmt.Errorf("sync system admin flags: %w", err)
	}
	autoRows, err := pool.Query(ctx, `SELECT api_name FROM metadata_automations ORDER BY api_name`)
	if err != nil {
		return fmt.Errorf("list automations: %w", err)
	}
	defer autoRows.Close()
	for autoRows.Next() {
		var apiName string
		if err := autoRows.Scan(&apiName); err != nil {
			return err
		}
		if err := db.EnsureAutomationInAccessCatalog(ctx, pool, apiName); err != nil {
			return fmt.Errorf("automation access %s: %w", apiName, err)
		}
	}
	if err := autoRows.Err(); err != nil {
		return err
	}
	toolRows, err := pool.Query(ctx, `SELECT api_name FROM metadata_canvases ORDER BY api_name`)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	defer toolRows.Close()
	for toolRows.Next() {
		var apiName string
		if err := toolRows.Scan(&apiName); err != nil {
			return err
		}
		if err := db.EnsureToolInAccessCatalog(ctx, pool, apiName); err != nil {
			return fmt.Errorf("tool access %s: %w", apiName, err)
		}
	}
	return toolRows.Err()
}

func ensureAdminPermissionSet(ctx context.Context, pool *db.Pool, ownerID string) (string, error) {
	id, err := db.EnsureSystemAdminPermissionSet(ctx, pool)
	if err != nil {
		return "", err
	}
	_, err = pool.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id)
VALUES ($1::uuid, $2::uuid) ON CONFLICT (user_id, permission_set_id) DO NOTHING`, ownerID, id)
	return id, err
}

type capabilityPermissionSetDef struct {
	APIName string
	Label   string
	Desc    string
	Caps    []string
}

func capabilityPermissionSetDefs() []capabilityPermissionSetDef {
	return []capabilityPermissionSetDef{
		{
			APIName: "ManageUsers",
			Label:   "Manage Users",
			Desc:    "Manage user principals and user credentials",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapIdentityUsers, authz.CapIDEGovern, authz.CapIDEGovernUsers, authz.CapIDEGovernEnv},
			),
		},
		{
			APIName: "ManageIntegrations",
			Label:   "Manage Integrations",
			Desc:    "Manage Connected Apps and service/agent credentials",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapIdentityIntegrations, authz.CapIDEGovern, authz.CapIDEGovernIntegrations, authz.CapIDEGovernEnv},
			),
		},
		{
			APIName: "ManagePermissions",
			Label:   "Manage Permissions",
			Desc:    "Define permission sets and assign Roles/PS",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapAuthzManage, authz.CapIDEGovern, authz.CapIDEGovernPermissions, authz.CapIDEGovernEnv},
			),
		},
		{
			APIName: "Build",
			Label:   "Build",
			Desc:    "Customize customer metadata and manage packages",
			Caps:    authz.MergeCapabilityLists([]string{authz.CapMetadataBuild}, authz.BuildIDECapabilities()),
		},
		{
			APIName: "Deploy",
			Label:   "Deploy",
			Desc:    "Create and promote Deploy bundles",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapDeployPromote},
				authz.ShipIDECapabilities(),
				authz.SettingsIDECapabilities(),
			),
		},
		{
			APIName: "Govern",
			Label:   "Govern",
			Desc:    "Install exposure/WAF and agent approvals",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapGovernNetwork, authz.CapGovernAgents},
				authz.GovernIDECapabilities(),
			),
		},
		{
			APIName: "Operate",
			Label:   "Operate",
			Desc:    "Operate data access plus Control IDE Operate chrome",
			Caps:    authz.OperateIDECapabilities(),
		},
		// Legacy pack names kept for existing assignments; caps remapped to canonical.
		{
			APIName: "MetadataCustomize",
			Label:   "Metadata Customize",
			Desc:    "Customer metadata customization (objects, fields, rules, automations, playbooks)",
			Caps:    authz.MergeCapabilityLists([]string{authz.CapMetadataBuild}, authz.BuildIDECapabilities()),
		},
		{
			APIName: "DeployPromote",
			Label:   "Deploy Promote",
			Desc:    "Create and promote customer-owned Deploy bundles",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapDeployPromote},
				authz.ShipIDECapabilities(),
				authz.SettingsIDECapabilities(),
			),
		},
		{
			APIName: "AgentsApprove",
			Label:   "Agents Approve",
			Desc:    "Approve agent runs awaiting human approval",
			Caps:    []string{authz.CapGovernAgents},
		},
		{
			APIName: "IdentityManage",
			Label:   "Identity Manage",
			Desc:    "Manage principals, credentials, and Role/permission-set assignment (legacy pack)",
			Caps: authz.MergeCapabilityLists(
				[]string{authz.CapIdentityUsers, authz.CapIdentityIntegrations},
				[]string{authz.CapIDEGovern, authz.CapIDEGovernUsers, authz.CapIDEGovernIntegrations, authz.CapIDEGovernEnv},
			),
		},
	}
}

func ensureCapabilityPermissionSets(ctx context.Context, pool *db.Pool) error {
	defs := capabilityPermissionSetDefs()
	for _, d := range defs {
		capsJSON, _ := json.Marshal(d.Caps)
		var id string
		var raw []byte
		err := pool.QueryRow(ctx, `
SELECT id::text, COALESCE(system_permissions, '[]'::jsonb) FROM permission_sets WHERE api_name = $1`,
			d.APIName).Scan(&id, &raw)
		if err == nil {
			var existing []string
			_ = json.Unmarshal(raw, &existing)
			merged := authz.NormalizeCapabilitySet(authz.MergeCapabilityLists(existing, d.Caps))
			mergedJSON, _ := json.Marshal(merged)
			_, _ = pool.Exec(ctx, `
UPDATE permission_sets SET system_permissions = $2::jsonb, description = $3, label = $4
WHERE id = $1::uuid`, id, string(mergedJSON), d.Desc, d.Label)
			continue
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions)
VALUES ($1, $2, $3, true, $4::jsonb)`, d.APIName, d.Label, d.Desc, string(capsJSON)); err != nil {
			return fmt.Errorf("create permission set %s: %w", d.APIName, err)
		}
	}
	return nil
}

func intPtr(n int) *int { return &n }

func strPtr(s string) *string { return &s }
