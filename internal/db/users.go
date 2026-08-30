package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MajestaNet/ide/internal/authz"
)

// User is a row from the users kernel table.
type User struct {
	ID             string
	Email          string
	DisplayName    string
	IsActive       bool
	IsAdmin        bool
	PrincipalType  string
	APIKeyName     *string
	OIDCSub        *string
	UserName       *string
	ExternalID     *string
	GivenName      *string
	FamilyName     *string
	PhoneNumber    *string
	Locale         *string
	Timezone       *string
	Title          *string
	Department     *string
	EmployeeNumber *string
	Data           map[string]any
	FrozenAt       *time.Time
	FrozenReason   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CanAuthenticate reports whether AuthN should accept the principal.
func (u *User) CanAuthenticate() bool {
	return u != nil && u.IsActive && u.FrozenAt == nil
}

// UserStore persists Majesta One users.
type UserStore struct {
	pool *Pool
}

// NewUserStore constructs a user store.
func NewUserStore(pool *Pool) *UserStore {
	return &UserStore{pool: pool}
}

const userSelectCols = `id, email, display_name, is_active, is_admin, COALESCE(principal_type, 'user'), api_key_name, oidc_sub, user_name, external_id, given_name, family_name, phone_number, locale, timezone, title, department, employee_number, COALESCE(data, '{}'::jsonb), frozen_at, frozen_reason, created_at, updated_at`

// GetByID loads a user by primary key.
func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	return s.scanOne(ctx, `SELECT `+userSelectCols+` FROM users WHERE id = $1`, id)
}

// GetByEmail loads a user by email.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, ErrNotFound
	}
	return s.scanOne(ctx, `SELECT `+userSelectCols+` FROM users WHERE lower(email) = lower($1)`, email)
}

// GetByOIDCSub loads a user by Cognito/OIDC subject.
func (s *UserStore) GetByOIDCSub(ctx context.Context, sub string) (*User, error) {
	return s.scanOne(ctx, `SELECT `+userSelectCols+` FROM users WHERE oidc_sub = $1`, sub)
}

// ListPermissionSetIDs returns permission set ids assigned directly to the user.
func (s *UserStore) ListPermissionSetIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT permission_set_id::text
FROM user_permission_sets
WHERE user_id = $1::uuid`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListRoleGrants returns API family scopes, admin, and role names from assigned roles.
// Assigned roles only — this query does not walk roles.parent_role_id (unused column;
// record sharing uses data_roles, ADR-016).
func (s *UserStore) ListRoleGrants(ctx context.Context, userID string) (scopes []authz.Scope, admin bool, roleNames []string, err error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.api_name, ras.scope
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
LEFT JOIN role_api_scopes ras ON ras.role_id = r.id
WHERE ur.user_id = $1::uuid`, userID)
	if err != nil {
		return nil, false, nil, err
	}
	defer rows.Close()
	seenScope := map[authz.Scope]struct{}{}
	seenRole := map[string]struct{}{}
	for rows.Next() {
		var apiName string
		var scope *string
		if err := rows.Scan(&apiName, &scope); err != nil {
			return nil, false, nil, err
		}
		if _, ok := seenRole[apiName]; !ok {
			seenRole[apiName] = struct{}{}
			roleNames = append(roleNames, apiName)
		}
		if scope == nil {
			continue
		}
		if *scope == "admin" {
			admin = true
			continue
		}
		sc := authz.Scope(*scope)
		switch sc {
		case authz.ScopeClient, authz.ScopeMetadata, authz.ScopeDeploy, authz.ScopeOps:
			if _, ok := seenScope[sc]; !ok {
				seenScope[sc] = struct{}{}
				scopes = append(scopes, sc)
			}
		}
	}
	return scopes, admin, roleNames, rows.Err()
}

// EnsureOIDCUser upserts a user for a legacy OIDC principal by verified subject.
// Email is profile data, never an authentication-link key (ADR-015).
func (s *UserStore) EnsureOIDCUser(ctx context.Context, id, sub, email, displayName string, autoProvision bool) (*User, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	var u *User
	if existing, err := s.GetByOIDCSub(ctx, sub); err == nil {
		if email != "" {
			_, err = s.pool.Exec(ctx, `
UPDATE users SET email = $2, display_name = COALESCE(NULLIF($3, ''), display_name), updated_at = now()
WHERE id = $1 AND (email IS DISTINCT FROM $2 OR display_name IS DISTINCT FROM COALESCE(NULLIF($3, ''), display_name))`,
				existing.ID, email, displayName)
			if err != nil {
				return nil, err
			}
		} else if displayName != "" {
			_, err = s.pool.Exec(ctx, `
UPDATE users SET display_name = $2, updated_at = now()
WHERE id = $1 AND display_name IS DISTINCT FROM $2`, existing.ID, displayName)
			if err != nil {
				return nil, err
			}
		}
		u, err = s.GetByID(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if u == nil {
		if !autoProvision {
			return nil, fmt.Errorf("%w: OIDC user is not provisioned", ErrNotFound)
		}
		if email == "" {
			return nil, fmt.Errorf("%w: email is required to provision a user", ErrValidation)
		}
		if displayName == "" {
			displayName = strings.Split(email, "@")[0]
		}
		_, err := s.pool.Exec(ctx, `
INSERT INTO users (id, email, display_name, oidc_sub, is_admin, is_active, principal_type)
VALUES ($1::uuid, $2, $3, $4, false, true, 'user')
ON CONFLICT (id) DO UPDATE SET
  oidc_sub = EXCLUDED.oidc_sub,
  email = COALESCE(EXCLUDED.email, users.email),
  display_name = EXCLUDED.display_name,
  updated_at = now()`,
			id, email, displayName, sub)
		if err != nil {
			return nil, err
		}
		u, err = s.GetByOIDCSub(ctx, sub)
		if err != nil {
			return nil, err
		}
	}

	hasRole, err := s.HasAnyRole(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if !hasRole {
		if err := s.EnsureSystemRoles(ctx); err != nil {
			return nil, err
		}
		if err := s.EnsureUserHasRole(ctx, u.ID, "StandardUser"); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// UpdateEmailIfEmpty sets email when currently null/empty (social profile fill-in).
func (s *UserStore) UpdateEmailIfEmpty(ctx context.Context, userID, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
UPDATE users SET email = $2, updated_at = now()
WHERE id = $1::uuid AND (email IS NULL OR email = '')`, userID, email)
	return err
}

// CreateSocialUser inserts a human principal. Email is required (ADR-015 amendment).
func (s *UserStore) CreateSocialUser(ctx context.Context, email, displayName, roleAPIName string) (*User, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" {
		return nil, fmt.Errorf("%w: email is required", ErrValidation)
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	if roleAPIName == "" {
		roleAPIName = "StandardUser"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	u, err := scanUserRow(tx.QueryRow(ctx, `
INSERT INTO users (email, display_name, is_admin, is_active, principal_type)
VALUES ($1, $2, false, true, 'user')
RETURNING `+userSelectCols, email, displayName))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: principal unique field already exists", ErrConflict)
		}
		return nil, err
	}
	if err := s.EnsureSystemRoles(ctx); err != nil {
		return nil, err
	}
	if err := assignRoleByAPIName(ctx, tx, u.ID, roleAPIName); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	// Explicit SystemAdmin auto-provision also gets the managed System Admin permission set.
	if roleAPIName == SystemAdminRoleAPIName {
		if err := s.GrantSystemAdminPack(ctx, u.ID); err != nil {
			return nil, err
		}
	}
	return s.GetByID(ctx, u.ID)
}

// EnsureBootstrapAdmin upserts the default admin used by env API keys.
func (s *UserStore) EnsureBootstrapAdmin(ctx context.Context, id, email, displayName string) (*User, error) {
	_, err := s.pool.Exec(ctx, `
INSERT INTO users (id, email, display_name, is_admin, is_active, principal_type)
VALUES ($1::uuid, $2, $3, true, true, 'service')
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  display_name = EXCLUDED.display_name,
  is_admin = true,
  is_active = true,
  principal_type = 'service',
  updated_at = now()`,
		id, email, displayName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_, err = s.pool.Exec(ctx, `
UPDATE users SET
  is_admin = true,
  is_active = true,
  principal_type = 'service',
  display_name = $2,
  updated_at = now()
WHERE lower(email) = lower($1)`, email, displayName)
			if err != nil {
				return nil, err
			}
			return s.GetByEmail(ctx, email)
		}
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// ErrPrincipalNoRole is returned when an authenticated principal has no Role assignment.
var ErrPrincipalNoRole = authz.ErrPrincipalNoRole

// EnsureSystemRoles upserts SystemAdmin, StandardUser, MetadataDeveloper, and DeployBot with API scopes.
// Safe to call from seed and from OIDC provisioning before Bootstrap.
func (s *UserStore) EnsureSystemRoles(ctx context.Context) error {
	type roleDef struct {
		APIName string
		Label   string
		Scopes  []string
	}
	roles := []roleDef{
		{APIName: "SystemAdmin", Label: "System Admin", Scopes: []string{"client", "metadata", "deploy", "ops", "admin"}},
		{APIName: "StandardUser", Label: "Standard User", Scopes: []string{"client"}},
		{APIName: "MetadataDeveloper", Label: "Metadata Developer", Scopes: []string{"client", "metadata"}},
		{APIName: "DeployBot", Label: "Deploy Bot", Scopes: []string{"deploy"}},
	}
	for _, r := range roles {
		var id string
		err := s.pool.QueryRow(ctx, `
INSERT INTO roles (api_name, label, is_system)
VALUES ($1, $2, true)
ON CONFLICT (api_name) DO UPDATE SET
  label = EXCLUDED.label,
  is_system = true
RETURNING id::text`, r.APIName, r.Label).Scan(&id)
		if err != nil {
			return fmt.Errorf("ensure role %s: %w", r.APIName, err)
		}
		for _, scope := range r.Scopes {
			if _, err := s.pool.Exec(ctx, `
INSERT INTO role_api_scopes (role_id, scope) VALUES ($1::uuid, $2)
ON CONFLICT (role_id, scope) DO NOTHING`, id, scope); err != nil {
				return fmt.Errorf("role %s scope %s: %w", r.APIName, scope, err)
			}
		}
	}
	return nil
}

// EnsureUserHasRole assigns roleApiName to the user when not already linked.
func (s *UserStore) EnsureUserHasRole(ctx context.Context, userID, roleAPIName string) error {
	return assignRoleByAPIName(ctx, s.pool, userID, roleAPIName)
}

// AssignRoleByAPIName links a user to a role by api_name.
func (s *UserStore) AssignRoleByAPIName(ctx context.Context, userID, roleAPIName string) error {
	return s.EnsureUserHasRole(ctx, userID, roleAPIName)
}

// UnassignRoleByAPIName unlinks a role while preserving the invariant that each principal has a role.
func (s *UserStore) UnassignRoleByAPIName(ctx context.Context, userID, roleAPIName string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	roleID, err := getRoleIDByAPIName(ctx, tx, roleAPIName)
	if err != nil {
		return err
	}
	var total, assigned int
	if err := tx.QueryRow(ctx, `
SELECT
  (SELECT COUNT(*) FROM user_roles WHERE user_id = $1::uuid),
  (SELECT COUNT(*) FROM user_roles WHERE user_id = $1::uuid AND role_id = $2::uuid)`,
		userID, roleID).Scan(&total, &assigned); err != nil {
		return err
	}
	if assigned > 0 && total <= 1 {
		return fmt.Errorf("%w: cannot remove the principal's last role", ErrPrincipalRequiresRole)
	}
	if assigned > 0 {
		if _, err := tx.Exec(ctx, `
DELETE FROM user_roles WHERE user_id = $1::uuid AND role_id = $2::uuid`, userID, roleID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// HasAnyRole reports whether the user has at least one role assignment.
func (s *UserStore) HasAnyRole(ctx context.Context, userID string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_roles WHERE user_id = $1::uuid`, userID).Scan(&n)
	return n > 0, err
}

// ErrNotFound is returned when a user row is missing.
var ErrNotFound = errors.New("not found")

// GetByAPIKeyName loads a service user bound to an env API key name.
func (s *UserStore) GetByAPIKeyName(ctx context.Context, apiKeyName string) (*User, error) {
	return s.scanOne(ctx, `SELECT `+userSelectCols+` FROM users WHERE api_key_name = $1`, apiKeyName)
}

// EnsureAPIKeyServicePrincipal creates or loads a service user for an env API
// key and synchronizes its grants. Only a one-way identifier is persisted; the
// API_KEYS plaintext must never become user metadata.
func (s *UserStore) EnsureAPIKeyServicePrincipal(ctx context.Context, apiKeySecret string, isAdmin bool, scopes []authz.Scope) (*User, error) {
	if apiKeySecret == "" {
		return nil, fmt.Errorf("api key secret required")
	}
	identifier := authz.APIKeyIdentifier(apiKeySecret)
	email := identifier + "@one.local"
	display := "Bootstrap API Key " + identifier[len(identifier)-12:]

	u, err := s.GetByAPIKeyName(ctx, identifier)
	if errors.Is(err, ErrNotFound) {
		// Lazily scrub principals created by older versions, which persisted the
		// raw API key in api_key_name, email, and display_name.
		if legacy, legacyErr := s.GetByAPIKeyName(ctx, apiKeySecret); legacyErr == nil {
			_, err = s.pool.Exec(ctx, `
UPDATE users SET
  api_key_name = $2,
  email = $3,
  display_name = $4,
  is_admin = $5,
  is_active = true,
  principal_type = 'service',
  updated_at = now()
WHERE id = $1::uuid`, legacy.ID, identifier, email, display, isAdmin)
			if err != nil {
				return nil, fmt.Errorf("scrub legacy api key principal: %w", err)
			}
			u, err = s.GetByID(ctx, legacy.ID)
		} else if !errors.Is(legacyErr, ErrNotFound) {
			return nil, legacyErr
		} else {
			var id string
			// Migration 0027 replaced UNIQUE(email) with a partial unique index on lower(email).
			// ON CONFLICT (email) no longer matches; infer the expression index instead.
			err = s.pool.QueryRow(ctx, `
INSERT INTO users (email, display_name, is_admin, is_active, principal_type, api_key_name)
VALUES ($1, $2, $3, true, 'service', $4)
ON CONFLICT ((lower(email))) WHERE email IS NOT NULL AND email <> '' DO UPDATE SET
  api_key_name = EXCLUDED.api_key_name,
  display_name = EXCLUDED.display_name,
  is_admin = EXCLUDED.is_admin,
  is_active = true,
  principal_type = 'service',
  updated_at = now()
RETURNING id::text`, email, display, isAdmin, identifier).Scan(&id)
			if err != nil {
				return nil, fmt.Errorf("ensure api key principal: %w", err)
			}
			u, err = s.GetByID(ctx, id)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := s.syncAPIKeyGrants(ctx, u.ID, identifier, isAdmin, scopes); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, u.ID)
}

// syncAPIKeyGrants makes API_KEYS configuration authoritative. Replacing the
// complete assignment set prevents removed scopes/admin from surviving a key
// configuration downgrade. A per-key system Role preserves exact family scope
// combinations (including metadata-only and ops-only keys).
func (s *UserStore) syncAPIKeyGrants(ctx context.Context, userID, identifier string, isAdmin bool, scopes []authz.Scope) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	roleAPIName := "BootstrapKey_" + strings.TrimPrefix(identifier, "apikey-")
	var roleID string
	err = tx.QueryRow(ctx, `
INSERT INTO roles (api_name, label, is_system)
VALUES ($1, $2, true)
ON CONFLICT (api_name) DO UPDATE SET label = EXCLUDED.label, is_system = true
RETURNING id::text`, roleAPIName, "Bootstrap API key").Scan(&roleID)
	if err != nil {
		return fmt.Errorf("ensure api key role: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_api_scopes WHERE role_id = $1::uuid`, roleID); err != nil {
		return err
	}
	seen := map[authz.Scope]struct{}{}
	for _, scope := range scopes {
		switch scope {
		case authz.ScopeClient, authz.ScopeMetadata, authz.ScopeDeploy, authz.ScopeOps:
		default:
			return fmt.Errorf("invalid api key scope %q", scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		if _, err := tx.Exec(ctx, `
INSERT INTO role_api_scopes (role_id, scope)
VALUES ($1::uuid, $2)
ON CONFLICT (role_id, scope) DO NOTHING`, roleID, string(scope)); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return fmt.Errorf("api key requires at least one scope")
	}
	if isAdmin {
		if _, err := tx.Exec(ctx, `
INSERT INTO role_api_scopes (role_id, scope)
VALUES ($1::uuid, 'admin')
ON CONFLICT (role_id, scope) DO NOTHING`, roleID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1::uuid`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id) VALUES ($1::uuid, $2::uuid)`, userID, roleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_permission_sets WHERE user_id = $1::uuid`, userID); err != nil {
		return err
	}
	permissionSets := []string{}
	if isAdmin {
		permissionSets = append(permissionSets, "Admin")
	} else {
		if _, ok := seen[authz.ScopeMetadata]; ok {
			permissionSets = append(permissionSets, "MetadataCustomize")
		}
		if _, ok := seen[authz.ScopeDeploy]; ok {
			permissionSets = append(permissionSets, "DeployPromote")
		}
	}
	for _, apiName := range permissionSets {
		if err := assignPermissionSetByAPIName(ctx, tx, userID, apiName); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE users SET is_admin = $2, updated_at = now() WHERE id = $1::uuid`, userID, isAdmin); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PermissionSetIDByAPIName loads a permission set id or ErrNotFound.
func (s *UserStore) PermissionSetIDByAPIName(ctx context.Context, apiName string) (string, error) {
	return getPermissionSetIDByAPIName(ctx, s.pool, apiName)
}

// AssignPermissionSetByAPIName links a user to a permission set by api_name.
func (s *UserStore) AssignPermissionSetByAPIName(ctx context.Context, userID, psAPIName string) error {
	return assignPermissionSetByAPIName(ctx, s.pool, userID, psAPIName)
}

// AssignPermissionSetByID links a user to a permission set by id.
func (s *UserStore) AssignPermissionSetByID(ctx context.Context, userID, permissionSetID string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, permission_set_id) DO NOTHING`, userID, strings.TrimSpace(permissionSetID))
	return err
}

// UnassignPermissionSetByAPIName unlinks a user from a permission set by api_name.
func (s *UserStore) UnassignPermissionSetByAPIName(ctx context.Context, userID, psAPIName string) error {
	psID, err := getPermissionSetIDByAPIName(ctx, s.pool, psAPIName)
	if err != nil {
		return err
	}
	return s.UnassignPermissionSetByID(ctx, userID, psID)
}

// UnassignPermissionSetByID unlinks a user from a permission set id.
func (s *UserStore) UnassignPermissionSetByID(ctx context.Context, userID, permissionSetID string) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM user_permission_sets
WHERE user_id = $1::uuid AND permission_set_id = $2::uuid`, userID, permissionSetID)
	return err
}

// ListPermissionSetAPINames returns permission set api_names assigned directly to the user.
func (s *UserStore) ListPermissionSetAPINames(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT ps.api_name
FROM user_permission_sets ups
JOIN permission_sets ps ON ps.id = ups.permission_set_id
WHERE ups.user_id = $1::uuid
ORDER BY ps.api_name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// ReplacePermissionSetsByAPINames replaces a user's permission-set assignments by api_name.
func (s *UserStore) ReplacePermissionSetsByAPINames(ctx context.Context, userID string, names []string) error {
	names, err := normalizeGrantNames(names)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := replacePermissionSetsByAPINames(ctx, tx, userID, names); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReplaceRolesByAPINames replaces a user's role assignments by api_name.
func (s *UserStore) ReplaceRolesByAPINames(ctx context.Context, userID string, names []string) error {
	names, err := normalizeGrantNames(names)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%w: principal requires at least one role", ErrPrincipalRequiresRole)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	roleIDs := make([]string, 0, len(names))
	for _, name := range names {
		roleID, err := getRoleIDByAPIName(ctx, tx, name)
		if err != nil {
			return err
		}
		roleIDs = append(roleIDs, roleID)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1::uuid`, userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, role_id) DO NOTHING`, userID, roleID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *UserStore) scanOne(ctx context.Context, q string, arg any) (*User, error) {
	row := s.pool.QueryRow(ctx, q, arg)
	u, err := scanUserRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func scanUserRow(row pgx.Row) (*User, error) {
	var u User
	var email, apiKeyName, oidc, userName, externalID, givenName, familyName, phoneNumber, locale, timezoneName, title, department, employeeNumber, frozenReason *string
	var frozenAt *time.Time
	var dataJSON []byte
	err := row.Scan(
		&u.ID, &email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.PrincipalType,
		&apiKeyName, &oidc, &userName, &externalID, &givenName, &familyName, &phoneNumber,
		&locale, &timezoneName, &title, &department, &employeeNumber, &dataJSON, &frozenAt, &frozenReason,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email != nil {
		u.Email = *email
	}
	u.APIKeyName = apiKeyName
	u.OIDCSub = oidc
	u.UserName = userName
	u.ExternalID = externalID
	u.GivenName = givenName
	u.FamilyName = familyName
	u.PhoneNumber = phoneNumber
	u.Locale = locale
	u.Timezone = timezoneName
	u.Title = title
	u.Department = department
	u.EmployeeNumber = employeeNumber
	u.Data = map[string]any{}
	if len(dataJSON) > 0 && string(dataJSON) != "null" {
		_ = json.Unmarshal(dataJSON, &u.Data)
		if u.Data == nil {
			u.Data = map[string]any{}
		}
	}
	u.FrozenAt = frozenAt
	u.FrozenReason = frozenReason
	return &u, nil
}

// CreatePrincipalInput creates a users row for Metadata principal admin.
type CreatePrincipalInput struct {
	Email                 string
	DisplayName           string
	PrincipalType         string // user | service | agent
	IsAdmin               bool
	UserName              string
	ExternalID            string
	GivenName             string
	FamilyName            string
	PhoneNumber           string
	Locale                string
	Timezone              string
	Title                 string
	Department            string
	EmployeeNumber        string
	Data                  map[string]any
	RoleAPIName           string
	RoleAPINames          []string
	PermissionSetAPINames []string
}

// Create inserts a principal. Email must be unique.
func (s *UserStore) Create(ctx context.Context, in CreatePrincipalInput) (*User, error) {
	if in.RoleAPIName != "" || len(in.RoleAPINames) > 0 || len(in.PermissionSetAPINames) > 0 {
		return s.CreateWithGrants(ctx, in)
	}
	u, err := s.insertPrincipal(ctx, s.pool, in)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, u.ID)
}

// CreateWithGrants inserts a principal and assigns Roles and permission sets atomically.
func (s *UserStore) CreateWithGrants(ctx context.Context, in CreatePrincipalInput) (*User, error) {
	roleNames, err := normalizeGrantNames(append([]string{in.RoleAPIName}, in.RoleAPINames...))
	if err != nil {
		return nil, err
	}
	if len(roleNames) == 0 {
		return nil, fmt.Errorf("%w: at least one roleApiName is required", ErrValidation)
	}
	psNames, err := normalizeGrantNames(in.PermissionSetAPINames)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	u, err := s.insertPrincipal(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	for _, roleName := range roleNames {
		if err := assignRoleByAPIName(ctx, tx, u.ID, roleName); err != nil {
			return nil, err
		}
	}
	for _, psName := range psNames {
		if err := assignPermissionSetByAPIName(ctx, tx, u.ID, psName); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, u.ID)
}

type userExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (s *UserStore) insertPrincipal(ctx context.Context, exec userExecutor, in CreatePrincipalInput) (*User, error) {
	pt := strings.TrimSpace(in.PrincipalType)
	if pt == "" {
		pt = "user"
	}
	switch pt {
	case "user", "service", "agent":
	default:
		return nil, fmt.Errorf("%w: principalType must be user, service, or agent", ErrValidation)
	}
	userName := strings.TrimSpace(in.UserName)
	email := strings.TrimSpace(in.Email)
	if email == "" {
		if pt == "user" {
			return nil, fmt.Errorf("%w: email is required", ErrValidation)
		}
		if userName == "" {
			return nil, fmt.Errorf("%w: email or userName is required", ErrValidation)
		}
		email = synthesizePrincipalEmail(pt, userName)
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = email
	}
	u, err := scanUserRow(exec.QueryRow(ctx, `
INSERT INTO users (
  email, display_name, is_admin, is_active, principal_type,
  user_name, external_id, given_name, family_name, phone_number,
  locale, timezone, title, department, employee_number, data
) VALUES (
  $1, $2, $3, true, $4,
  $5, $6, $7, $8, $9,
  $10, $11, $12, $13, $14, $15::jsonb
)
RETURNING `+userSelectCols,
		email, display, in.IsAdmin, pt,
		nilIfBlank(userName), nilIfBlank(in.ExternalID), nilIfBlank(in.GivenName), nilIfBlank(in.FamilyName), nilIfBlank(in.PhoneNumber),
		nilIfBlank(in.Locale), nilIfBlank(in.Timezone), nilIfBlank(in.Title), nilIfBlank(in.Department),
		nilIfBlank(in.EmployeeNumber), marshalUserData(in.Data),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: principal unique field already exists", ErrConflict)
		}
		return nil, err
	}
	return u, nil
}

// ListPrincipalsFilter filters principal list queries.
type ListPrincipalsFilter struct {
	PrincipalType string // optional
	IsActive      *bool  // optional
	Email         string // optional exact match
	UserName      string // optional exact match
	ExternalID    string // optional exact match
}

// List returns principals matching the filter (ordered by created_at).
func (s *UserStore) List(ctx context.Context, f ListPrincipalsFilter) ([]User, error) {
	q := `SELECT ` + userSelectCols + ` FROM users WHERE 1=1`
	args := []any{}
	if f.PrincipalType != "" {
		args = append(args, f.PrincipalType)
		q += fmt.Sprintf(` AND principal_type = $%d`, len(args))
	}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		q += fmt.Sprintf(` AND is_active = $%d`, len(args))
	}
	if f.Email != "" {
		args = append(args, strings.TrimSpace(f.Email))
		q += fmt.Sprintf(` AND email = $%d`, len(args))
	}
	if f.UserName != "" {
		args = append(args, strings.TrimSpace(f.UserName))
		q += fmt.Sprintf(` AND user_name = $%d`, len(args))
	}
	if f.ExternalID != "" {
		args = append(args, strings.TrimSpace(f.ExternalID))
		q += fmt.Sprintf(` AND external_id = $%d`, len(args))
	}
	q += ` ORDER BY created_at ASC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// UpdatePrincipalInput patches mutable principal fields.
type UpdatePrincipalInput struct {
	Email                 *string
	UserName              *string
	ExternalID            *string
	GivenName             *string
	FamilyName            *string
	PhoneNumber           *string
	Locale                *string
	Timezone              *string
	Title                 *string
	Department            *string
	EmployeeNumber        *string
	DisplayName           *string
	DataPatch             map[string]any // nil = no JSONB change; nil values delete keys
	IsActive              *bool
	IsAdmin               *bool
	PermissionSetAPINames []string
}

// Update patches mutable principal fields.
func (s *UserStore) Update(ctx context.Context, id string, in UpdatePrincipalInput) (*User, error) {
	cur, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.IsActive != nil && *in.IsActive && cur.FrozenAt != nil {
		return nil, fmt.Errorf("%w: principal is frozen", ErrPrincipalFrozen)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	patches := []struct {
		col string
		val any
	}{
		{col: "email", val: stringPatch(in.Email, false)},
		{col: "display_name", val: stringPatch(in.DisplayName, false)},
		{col: "user_name", val: stringPatch(in.UserName, true)},
		{col: "external_id", val: stringPatch(in.ExternalID, true)},
		{col: "given_name", val: stringPatch(in.GivenName, true)},
		{col: "family_name", val: stringPatch(in.FamilyName, true)},
		{col: "phone_number", val: stringPatch(in.PhoneNumber, true)},
		{col: "locale", val: stringPatch(in.Locale, true)},
		{col: "timezone", val: stringPatch(in.Timezone, true)},
		{col: "title", val: stringPatch(in.Title, true)},
		{col: "department", val: stringPatch(in.Department, true)},
		{col: "employee_number", val: stringPatch(in.EmployeeNumber, true)},
	}
	for _, p := range patches {
		if p.val == nilPatch {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE users SET %s = $2, updated_at = now() WHERE id = $1::uuid`, p.col), id, p.val); err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("%w: principal unique field already exists", ErrConflict)
			}
			return nil, err
		}
	}
	if in.DataPatch != nil {
		merged := map[string]any{}
		for k, v := range cur.Data {
			merged[k] = v
		}
		for k, v := range in.DataPatch {
			if v == nil {
				delete(merged, k)
			} else {
				merged[k] = v
			}
		}
		payload, err := json.Marshal(merged)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE users SET data = $2::jsonb, updated_at = now() WHERE id = $1::uuid`, id, string(payload)); err != nil {
			return nil, err
		}
	}
	if in.IsActive != nil {
		if _, err := tx.Exec(ctx, `
UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1::uuid`, id, *in.IsActive); err != nil {
			return nil, err
		}
	}
	if in.IsAdmin != nil {
		if _, err := tx.Exec(ctx, `
UPDATE users SET is_admin = $2, updated_at = now() WHERE id = $1::uuid`, id, *in.IsAdmin); err != nil {
			return nil, err
		}
	}
	if in.PermissionSetAPINames != nil {
		names, err := normalizeGrantNames(in.PermissionSetAPINames)
		if err != nil {
			return nil, err
		}
		if err := replacePermissionSetsByAPINames(ctx, tx, id, names); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// FreezePrincipal blocks authentication without changing active/admin flags.
func (s *UserStore) FreezePrincipal(ctx context.Context, id, reason string) (*User, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE users
SET frozen_at = now(), frozen_reason = $2, updated_at = now()
WHERE id = $1::uuid`, id, strings.TrimSpace(reason))
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetByID(ctx, id)
}

// UnfreezePrincipal clears freeze metadata and optionally reactivates the principal.
func (s *UserStore) UnfreezePrincipal(ctx context.Context, id string, reactivate bool) (*User, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE users
SET frozen_at = NULL,
    frozen_reason = NULL,
    is_active = CASE WHEN $2 THEN true ELSE is_active END,
    updated_at = now()
WHERE id = $1::uuid`, id, reactivate)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetByID(ctx, id)
}

// DeactivatePrincipal deprovisions a principal by setting is_active=false.
func (s *UserStore) DeactivatePrincipal(ctx context.Context, id string) (*User, error) {
	hasIdentityAdmin, err := s.activePrincipalHasCapability(ctx, id, authz.CapIdentityUsers)
	if err != nil {
		return nil, err
	}
	if hasIdentityAdmin {
		n, err := s.CountActivePrincipalsWithCapability(ctx, authz.CapIdentityUsers)
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, ErrLastIdentityAdmin
		}
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE users SET is_active = false, updated_at = now() WHERE id = $1::uuid`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetByID(ctx, id)
}

// CountActivePrincipalsWithCapability counts active, non-frozen principals holding a capability.
func (s *UserStore) CountActivePrincipalsWithCapability(ctx context.Context, capability string) (int, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return 0, fmt.Errorf("%w: capability is required", ErrValidation)
	}
	var n int
	err := s.pool.QueryRow(ctx, `
SELECT COUNT(DISTINCT u.id)
FROM users u
WHERE u.is_active = true
  AND u.frozen_at IS NULL
  AND (
    u.is_admin = true
    OR EXISTS (
      SELECT 1
      FROM user_permission_sets ups
      JOIN permission_sets ps ON ps.id = ups.permission_set_id
      WHERE ups.user_id = u.id
        AND EXISTS (
          SELECT 1
          FROM jsonb_array_elements_text(
            CASE
              WHEN jsonb_typeof(ps.system_permissions) = 'array' THEN ps.system_permissions
              ELSE '[]'::jsonb
            END
          ) AS cap(value)
          WHERE cap.value = $1
        )
    )
  )`, capability).Scan(&n)
	return n, err
}

// RoleInfo is a roles row with its API scopes.
type RoleInfo struct {
	ID       string
	APIName  string
	Label    string
	IsSystem bool
	Scopes   []string
}

// ListRoles returns all roles with their role_api_scopes.
func (s *UserStore) ListRoles(ctx context.Context) ([]RoleInfo, error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.id::text, r.api_name, r.label, r.is_system, COALESCE(ras.scope, '')
FROM roles r
LEFT JOIN role_api_scopes ras ON ras.role_id = r.id
ORDER BY r.api_name, ras.scope`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]*RoleInfo{}
	var order []string
	for rows.Next() {
		var id, apiName, label, scope string
		var isSystem bool
		if err := rows.Scan(&id, &apiName, &label, &isSystem, &scope); err != nil {
			return nil, err
		}
		ri, ok := byID[id]
		if !ok {
			ri = &RoleInfo{ID: id, APIName: apiName, Label: label, IsSystem: isSystem}
			byID[id] = ri
			order = append(order, id)
		}
		if scope != "" {
			ri.Scopes = append(ri.Scopes, scope)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]RoleInfo, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// GetRoleByAPIName loads one role and its API scopes.
func (s *UserStore) GetRoleByAPIName(ctx context.Context, apiName string) (*RoleInfo, error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.id::text, r.api_name, r.label, r.is_system, COALESCE(ras.scope, '')
FROM roles r
LEFT JOIN role_api_scopes ras ON ras.role_id = r.id
WHERE r.api_name = $1
ORDER BY ras.scope`, strings.TrimSpace(apiName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out *RoleInfo
	for rows.Next() {
		var id, name, label, scope string
		var isSystem bool
		if err := rows.Scan(&id, &name, &label, &isSystem, &scope); err != nil {
			return nil, err
		}
		if out == nil {
			out = &RoleInfo{ID: id, APIName: name, Label: label, IsSystem: isSystem}
		}
		if scope != "" {
			out.Scopes = append(out.Scopes, scope)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, ErrNotFound
	}
	return out, nil
}

// CreateRole creates a customer-defined Role with validated API scopes.
func (s *UserStore) CreateRole(ctx context.Context, apiName, label string, scopes []string) (*RoleInfo, error) {
	apiName = strings.TrimSpace(apiName)
	label = strings.TrimSpace(label)
	if apiName == "" || label == "" {
		return nil, fmt.Errorf("%w: apiName and label are required", ErrValidation)
	}
	scopes, err := validateRoleScopes(scopes)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `
INSERT INTO roles (api_name, label, is_system)
VALUES ($1, $2, false)
RETURNING id::text`, apiName, label).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: role apiName already exists", ErrConflict)
		}
		return nil, err
	}
	if err := replaceRoleScopes(ctx, tx, id, scopes); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetRoleByAPIName(ctx, apiName)
}

// UpdateRole replaces a customer Role's label and scopes.
func (s *UserStore) UpdateRole(ctx context.Context, apiName, label string, scopes []string) (*RoleInfo, error) {
	apiName = strings.TrimSpace(apiName)
	label = strings.TrimSpace(label)
	if apiName == "" || label == "" {
		return nil, fmt.Errorf("%w: apiName and label are required", ErrValidation)
	}
	scopes, err := validateRoleScopes(scopes)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	role, err := getRoleForUpdate(ctx, tx, apiName)
	if err != nil {
		return nil, err
	}
	if role.IsSystem {
		return nil, fmt.Errorf("%w: system roles cannot be updated", ErrValidation)
	}
	if _, err := tx.Exec(ctx, `UPDATE roles SET label = $2 WHERE id = $1::uuid`, role.ID, label); err != nil {
		return nil, err
	}
	if err := replaceRoleScopes(ctx, tx, role.ID, scopes); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetRoleByAPIName(ctx, apiName)
}

// DeleteRole deletes a customer Role. Assigned roles require force=true.
func (s *UserStore) DeleteRole(ctx context.Context, apiName string, force bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	role, err := getRoleForUpdate(ctx, tx, strings.TrimSpace(apiName))
	if err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("%w: system roles cannot be deleted", ErrValidation)
	}
	var assigned int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_roles WHERE role_id = $1::uuid`, role.ID).Scan(&assigned); err != nil {
		return err
	}
	if assigned > 0 && !force {
		return fmt.Errorf("%w: role is assigned to principals", ErrConflict)
	}
	if assigned > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE role_id = $1::uuid`, role.ID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM roles WHERE id = $1::uuid`, role.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ErrValidation is returned for invalid principal admin inputs.
var ErrValidation = errors.New("validation error")

// ErrConflict is returned when a unique constraint is violated.
var ErrConflict = errors.New("conflict")

// ErrPrincipalFrozen is returned when a frozen principal would be reactivated for AuthN.
var ErrPrincipalFrozen = errors.New("principal is frozen")

// ErrPrincipalRequiresRole is returned when an operation would leave a principal without roles.
var ErrPrincipalRequiresRole = errors.New("PRINCIPAL_REQUIRES_ROLE")

// ErrLastIdentityAdmin is returned when deprovisioning would remove the last identity admin.
var ErrLastIdentityAdmin = errors.New("cannot deprovision last identity admin")

var nilPatch = &struct{}{}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func nilIfBlank(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func marshalUserData(data map[string]any) string {
	if data == nil {
		return "{}"
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func stringPatch(s *string, clearEmpty bool) any {
	if s == nil {
		return nilPatch
	}
	v := strings.TrimSpace(*s)
	if clearEmpty && v == "" {
		return nil
	}
	return v
}

func synthesizePrincipalEmail(principalType, userName string) string {
	local := strings.Trim(safeEmailLocalPart(userName), "+._-")
	if local == "" {
		local = strings.Trim(safeEmailLocalPart(principalType), "+._-")
	}
	if local == "" {
		local = "principal"
	}
	return "scim+" + local + "@one.local"
}

func safeEmailLocalPart(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	lastWasPlus := false
	for _, r := range raw {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if valid {
			b.WriteRune(r)
			lastWasPlus = false
			continue
		}
		if !lastWasPlus {
			b.WriteByte('+')
			lastWasPlus = true
		}
	}
	return b.String()
}

func normalizeGrantNames(names []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

func getRoleIDByAPIName(ctx context.Context, exec userExecutor, roleAPIName string) (string, error) {
	var roleID string
	err := exec.QueryRow(ctx, `SELECT id::text FROM roles WHERE api_name = $1`, strings.TrimSpace(roleAPIName)).Scan(&roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: role %s not found", ErrNotFound, roleAPIName)
	}
	return roleID, err
}

func assignRoleByAPIName(ctx context.Context, exec userExecutor, userID, roleAPIName string) error {
	roleID, err := getRoleIDByAPIName(ctx, exec, roleAPIName)
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id) VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, role_id) DO NOTHING`, userID, roleID)
	return err
}

func getRoleForUpdate(ctx context.Context, tx pgx.Tx, apiName string) (*RoleInfo, error) {
	var r RoleInfo
	err := tx.QueryRow(ctx, `
SELECT id::text, api_name, label, is_system
FROM roles
WHERE api_name = $1
FOR UPDATE`, apiName).Scan(&r.ID, &r.APIName, &r.Label, &r.IsSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func validateRoleScopes(scopes []string) ([]string, error) {
	allowed := map[string]struct{}{
		string(authz.ScopeClient):   {},
		string(authz.ScopeMetadata): {},
		string(authz.ScopeDeploy):   {},
		string(authz.ScopeOps):      {},
		"admin":                     {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := allowed[scope]; !ok {
			return nil, fmt.Errorf("%w: invalid role scope %q", ErrValidation, scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out, nil
}

func replaceRoleScopes(ctx context.Context, tx pgx.Tx, roleID string, scopes []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM role_api_scopes WHERE role_id = $1::uuid`, roleID); err != nil {
		return err
	}
	for _, scope := range scopes {
		if _, err := tx.Exec(ctx, `
INSERT INTO role_api_scopes (role_id, scope)
VALUES ($1::uuid, $2)
ON CONFLICT (role_id, scope) DO NOTHING`, roleID, scope); err != nil {
			return err
		}
	}
	return nil
}

func getPermissionSetIDByAPIName(ctx context.Context, exec userExecutor, psAPIName string) (string, error) {
	var psID string
	err := exec.QueryRow(ctx, `SELECT id::text FROM permission_sets WHERE api_name = $1`, strings.TrimSpace(psAPIName)).Scan(&psID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: permission set %s not found", ErrNotFound, psAPIName)
	}
	return psID, err
}

func assignPermissionSetByAPIName(ctx context.Context, exec userExecutor, userID, psAPIName string) error {
	psID, err := getPermissionSetIDByAPIName(ctx, exec, psAPIName)
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, permission_set_id) DO NOTHING`, userID, psID)
	return err
}

func replacePermissionSetsByAPINames(ctx context.Context, tx pgx.Tx, userID string, names []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_permission_sets WHERE user_id = $1::uuid`, userID); err != nil {
		return err
	}
	for _, name := range names {
		if err := assignPermissionSetByAPIName(ctx, tx, userID, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserStore) activePrincipalHasCapability(ctx context.Context, userID, capability string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM users u
  WHERE u.id = $1::uuid
    AND u.is_active = true
    AND u.frozen_at IS NULL
    AND (
      u.is_admin = true
      OR EXISTS (
        SELECT 1
        FROM user_permission_sets ups
        JOIN permission_sets ps ON ps.id = ups.permission_set_id
        WHERE ups.user_id = u.id
          AND EXISTS (
            SELECT 1
            FROM jsonb_array_elements_text(
            CASE
              WHEN jsonb_typeof(ps.system_permissions) = 'array' THEN ps.system_permissions
              ELSE '[]'::jsonb
            END
          ) AS cap(value)
            WHERE cap.value = $2
          )
      )
    )
)`, userID, strings.TrimSpace(capability)).Scan(&ok)
	return ok, err
}
