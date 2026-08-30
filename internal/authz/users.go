package authz

import (
	"context"
	"errors"
	"time"
)

// UserRecord is the authz view of a Majesta One user.
type UserRecord struct {
	ID            string
	Email         string
	DisplayName   string
	IsActive      bool
	IsAdmin       bool
	PrincipalType string
	OIDCSub       string
	FrozenAt      *time.Time
}

// CanAuthenticate reports whether AuthN should accept the principal.
func (u *UserRecord) CanAuthenticate() bool {
	return u != nil && u.IsActive && u.FrozenAt == nil
}

// UserRepository loads/provisions users for AuthN (implemented by internal/db).
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*UserRecord, error)
	GetByEmail(ctx context.Context, email string) (*UserRecord, error)
	GetByOIDCSub(ctx context.Context, sub string) (*UserRecord, error)
	EnsureOIDCUser(ctx context.Context, id, sub, email, displayName string, autoProvision bool) (*UserRecord, error)
	ListPermissionSetIDs(ctx context.Context, userID string) ([]string, error)
	// ListRoleGrants returns scopes, admin flag, and role api_names from assigned Roles.
	// Implementations may return empty slices when no roles are assigned.
	ListRoleGrants(ctx context.Context, userID string) (scopes []Scope, admin bool, roleNames []string, err error)
	// EnsureAPIKeyServicePrincipal binds an env API key secret to a distinct
	// service user without persisting or exposing the plaintext secret.
	EnsureAPIKeyServicePrincipal(ctx context.Context, apiKeySecret string, isAdmin bool, scopes []Scope) (*UserRecord, error)
}

// CredentialRecord is an active principal_credentials row.
type CredentialRecord struct {
	ID             string
	UserID         string
	CredentialKind string
	SecretHash     string
	Label          string
}

// CredentialRepository verifies client secrets for /auth/v1/token.
type CredentialRepository interface {
	ListActiveByUserID(ctx context.Context, userID string) ([]CredentialRecord, error)
}

// ErrUserNotFound is returned by UserRepository implementations.
var ErrUserNotFound = errors.New("user not found")

// ErrCredentialNotFound is returned when no matching credential exists.
var ErrCredentialNotFound = errors.New("credential not found")

// ErrPrincipalNoRole is returned when an authenticated principal has no Role assignment.
var ErrPrincipalNoRole = errors.New("principal has no role")
