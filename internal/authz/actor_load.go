package authz

import (
	"context"
	"fmt"
	"strings"
)

// LoadActor reconstructs a principal from a users row plus Roles and permission sets.
// Missing or unresolvable IDs fail; callers must not fall back to DEFAULT_OWNER_ID.
func LoadActor(ctx context.Context, users UserRepository, userID string) (*Actor, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("actor id required")
	}
	if users == nil {
		return nil, fmt.Errorf("user repository required")
	}
	u, err := users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load actor: %w", err)
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	if !u.CanAuthenticate() {
		if u.FrozenAt != nil {
			return nil, fmt.Errorf("user frozen")
		}
		return nil, fmt.Errorf("user inactive")
	}
	psIDs, err := users.ListPermissionSetIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load actor permission sets: %w", err)
	}
	scopes, roleAdmin, roleNames, err := users.ListRoleGrants(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load actor roles: %w", err)
	}
	return &Actor{
		ID:               u.ID,
		Email:            u.Email,
		DisplayName:      u.DisplayName,
		PrincipalType:    u.PrincipalType,
		IsAdmin:          u.IsAdmin || roleAdmin,
		PermissionSetIDs: psIDs,
		Scopes:           scopes,
		Roles:            roleNames,
		AuthMethod:       "agent_run",
	}, nil
}
