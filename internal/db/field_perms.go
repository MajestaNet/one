package db

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
)

// FieldPermStore implements authz.FieldPermissionStore.
type FieldPermStore struct {
	Pool *Pool
}

var _ authz.FieldPermissionStore = (*FieldPermStore)(nil)

// ListByPermissionSets returns field_permissions for the given set ids.
func (s *FieldPermStore) ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]authz.FieldPermission, error) {
	if len(permissionSetIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT permission_set_id::text, object_api_name, field_api_name, can_read, can_edit
FROM field_permissions
WHERE permission_set_id = ANY($1::uuid[])`, permissionSetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []authz.FieldPermission
	for rows.Next() {
		var p authz.FieldPermission
		if err := rows.Scan(&p.PermissionSetID, &p.ObjectAPIName, &p.FieldAPIName, &p.CanRead, &p.CanEdit); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
