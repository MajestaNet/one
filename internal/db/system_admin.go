package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
)

// Managed full-access permission set (api_name stays Admin for data-access catalog wiring).
const (
	SystemAdminPermissionSetAPIName = "Admin"
	SystemAdminPermissionSetLabel   = "System Admin"
	SystemAdminRoleAPIName          = "SystemAdmin"
)

// EnsureSystemAdminPermissionSet upserts the managed System Admin permission set
// (api_name Admin) with every canonical system capability and broad product access.
func EnsureSystemAdminPermissionSet(ctx context.Context, pool *Pool) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("nil pool")
	}
	capsJSON, err := json.Marshal(authz.AllSystemCapabilities())
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions, all_automations, all_tools)
VALUES ($1, $2, $3, true, $4::jsonb, true, true)
ON CONFLICT (api_name) DO UPDATE SET
  label = EXCLUDED.label,
  description = EXCLUDED.description,
  is_system = true,
  system_permissions = EXCLUDED.system_permissions,
  all_automations = true,
  all_tools = true
RETURNING id::text`,
		SystemAdminPermissionSetAPIName,
		SystemAdminPermissionSetLabel,
		"Full object, field, automation, tool, and system access",
		string(capsJSON),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("ensure System Admin permission set: %w", err)
	}
	return id, nil
}

// AnyActiveHumanHasRole reports whether an active, unfrozen human principal has the role.
func (s *UserStore) AnyActiveHumanHasRole(ctx context.Context, roleAPIName string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM user_roles ur
JOIN users u ON u.id = ur.user_id
JOIN roles r ON r.id = ur.role_id
WHERE r.api_name = $1
  AND u.principal_type = 'user'
  AND u.is_active = true
  AND u.frozen_at IS NULL`, strings.TrimSpace(roleAPIName)).Scan(&n)
	return n > 0, err
}

// EnsureAdminFullDataAccess upgrades every object/field row on the Admin permission set
// to full grants. Repairs stale deny stubs left by ON CONFLICT DO NOTHING on upgrades.
func EnsureAdminFullDataAccess(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE object_permissions op
SET can_create = true, can_read = true, can_update = true, can_delete = true,
    view_all = true, modify_all = true
FROM permission_sets ps
WHERE op.permission_set_id = ps.id AND ps.api_name = $1`, SystemAdminPermissionSetAPIName)
	if err != nil {
		return fmt.Errorf("admin object data access: %w", err)
	}
	_, err = pool.Exec(ctx, `
UPDATE field_permissions fp
SET can_read = true, can_edit = true
FROM permission_sets ps
WHERE fp.permission_set_id = ps.id AND ps.api_name = $1`, SystemAdminPermissionSetAPIName)
	if err != nil {
		return fmt.Errorf("admin field data access: %w", err)
	}
	return nil
}

// SyncSystemAdminUserFlags sets is_admin=true for every principal holding the SystemAdmin role.
func SyncSystemAdminUserFlags(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE users u
SET is_admin = true, updated_at = now()
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = u.id AND r.api_name = $1 AND u.is_admin IS NOT TRUE`, SystemAdminRoleAPIName)
	return err
}

// GrantSystemAdminPack assigns SystemAdmin role + System Admin (Admin) permission set and is_admin.
func (s *UserStore) GrantSystemAdminPack(ctx context.Context, userID string) error {
	if err := s.EnsureSystemRoles(ctx); err != nil {
		return err
	}
	if _, err := EnsureSystemAdminPermissionSet(ctx, s.pool); err != nil {
		return err
	}
	if err := s.EnsureUserHasRole(ctx, userID, SystemAdminRoleAPIName); err != nil {
		return err
	}
	if err := s.AssignPermissionSetByAPIName(ctx, userID, SystemAdminPermissionSetAPIName); err != nil {
		return err
	}
	if err := EnsureAdminFullDataAccess(ctx, s.pool); err != nil {
		return err
	}
	if err := SyncSystemAdminUserFlags(ctx, s.pool); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
UPDATE users SET is_admin = true, updated_at = now() WHERE id = $1::uuid`, userID)
	return err
}

// EnsureInitialHumanSystemAdmin promotes the authenticating human to System Admin when
// no active human yet holds the SystemAdmin role. That covers greenfield social/local Sign in
// (so Control IDE shows all four tiles) and upgrades an already-provisioned first user on
// their next sign-in. Service/API-key SystemAdmin principals do not count.
func (s *UserStore) EnsureInitialHumanSystemAdmin(ctx context.Context, userID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("nil user store")
	}
	// Ensure the managed grants exist before entering the short serialized
	// election below. Each helper is idempotent and safe on upgrades.
	if err := s.EnsureSystemRoles(ctx); err != nil {
		return err
	}
	if _, err := EnsureSystemAdminPermissionSet(ctx, s.pool); err != nil {
		return err
	}
	if err := EnsureAdminFullDataAccess(ctx, s.pool); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize the greenfield election across API replicas. Without this lock,
	// two simultaneous first sign-ins can both observe an empty role and both be
	// promoted. The transaction-scoped lock is released on commit/rollback.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1297040206, 1)`); err != nil {
		return err
	}
	var has bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM user_roles ur
  JOIN users u ON u.id = ur.user_id
  JOIN roles r ON r.id = ur.role_id
  WHERE r.api_name = $1
    AND u.principal_type = 'user'
    AND u.is_active = true
    AND u.frozen_at IS NULL
)`, SystemAdminRoleAPIName).Scan(&has); err != nil {
		return err
	}
	if has {
		return tx.Commit(ctx)
	}
	var eligible bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM users
  WHERE id=$1::uuid AND principal_type='user' AND is_active=true AND frozen_at IS NULL
)`, userID).Scan(&eligible); err != nil {
		return err
	}
	if !eligible {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id)
SELECT $1::uuid, id FROM roles WHERE api_name=$2
ON CONFLICT (user_id, role_id) DO NOTHING`, userID, SystemAdminRoleAPIName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id)
SELECT $1::uuid, id FROM permission_sets WHERE api_name=$2
ON CONFLICT (user_id, permission_set_id) DO NOTHING`, userID, SystemAdminPermissionSetAPIName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE users SET is_admin=true, updated_at=now() WHERE id=$1::uuid`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
