package db

import (
	"context"
	"encoding/json"

	"github.com/MajestaNet/ide/internal/authz"
)

// SystemPermStore loads permission_sets.system_permissions.
type SystemPermStore struct {
	pool *Pool
}

// NewSystemPermStore constructs a system permission store.
func NewSystemPermStore(pool *Pool) *SystemPermStore {
	return &SystemPermStore{pool: pool}
}

// ListSystemPermissions unions system_permissions across the given permission set ids.
func (s *SystemPermStore) ListSystemPermissions(ctx context.Context, permissionSetIDs []string) ([]string, error) {
	if len(permissionSetIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
SELECT COALESCE(system_permissions, '[]'::jsonb)
FROM permission_sets
WHERE id = ANY($1::uuid[])`, permissionSetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	var out []string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var caps []string
		if err := json.Unmarshal(raw, &caps); err != nil {
			continue
		}
		for _, c := range caps {
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
	return out, rows.Err()
}

// AuthzSystemPerms adapts SystemPermStore to authz.SystemPermissionStore.
type AuthzSystemPerms struct {
	Store *SystemPermStore
}

var _ authz.SystemPermissionStore = (*AuthzSystemPerms)(nil)

func (a *AuthzSystemPerms) ListSystemPermissions(ctx context.Context, permissionSetIDs []string) ([]string, error) {
	return a.Store.ListSystemPermissions(ctx, permissionSetIDs)
}
