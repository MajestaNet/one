package db

import (
	"context"
	"errors"

	"github.com/MajestaNet/ide/internal/authz"
)

// AuthzUsers adapts UserStore to authz.UserRepository.
type AuthzUsers struct {
	Store *UserStore
}

var _ authz.UserRepository = (*AuthzUsers)(nil)

func (a *AuthzUsers) GetByID(ctx context.Context, id string) (*authz.UserRecord, error) {
	u, err := a.Store.GetByID(ctx, id)
	return mapUser(u, err)
}

func (a *AuthzUsers) GetByEmail(ctx context.Context, email string) (*authz.UserRecord, error) {
	u, err := a.Store.GetByEmail(ctx, email)
	return mapUser(u, err)
}

func (a *AuthzUsers) GetByOIDCSub(ctx context.Context, sub string) (*authz.UserRecord, error) {
	u, err := a.Store.GetByOIDCSub(ctx, sub)
	return mapUser(u, err)
}

func (a *AuthzUsers) EnsureOIDCUser(ctx context.Context, id, sub, email, displayName string, autoProvision bool) (*authz.UserRecord, error) {
	u, err := a.Store.EnsureOIDCUser(ctx, id, sub, email, displayName, autoProvision)
	return mapUser(u, err)
}

func (a *AuthzUsers) ListPermissionSetIDs(ctx context.Context, userID string) ([]string, error) {
	return a.Store.ListPermissionSetIDs(ctx, userID)
}

func (a *AuthzUsers) ListRoleGrants(ctx context.Context, userID string) ([]authz.Scope, bool, []string, error) {
	return a.Store.ListRoleGrants(ctx, userID)
}

func (a *AuthzUsers) EnsureAPIKeyServicePrincipal(ctx context.Context, apiKeySecret string, isAdmin bool, scopes []authz.Scope) (*authz.UserRecord, error) {
	u, err := a.Store.EnsureAPIKeyServicePrincipal(ctx, apiKeySecret, isAdmin, scopes)
	return mapUser(u, err)
}

func mapUser(u *User, err error) (*authz.UserRecord, error) {
	if errors.Is(err, ErrNotFound) {
		return nil, authz.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	rec := &authz.UserRecord{
		ID:            u.ID,
		Email:         u.Email,
		DisplayName:   u.DisplayName,
		IsActive:      u.IsActive,
		IsAdmin:       u.IsAdmin,
		PrincipalType: u.PrincipalType,
		FrozenAt:      u.FrozenAt,
	}
	if u.OIDCSub != nil {
		rec.OIDCSub = *u.OIDCSub
	}
	return rec, nil
}
