package db

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
)

// ObjectPermStore implements authz.ObjectPermissionStore.
type ObjectPermStore struct {
	Pool *Pool
}

var _ authz.ObjectPermissionStore = (*ObjectPermStore)(nil)

// ListByPermissionSets returns object_permissions for the given set ids.
func (s *ObjectPermStore) ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]authz.ObjectPermission, error) {
	if len(permissionSetIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT permission_set_id::text, object_api_name,
       can_create, can_read, can_update, can_delete, view_all, modify_all
FROM object_permissions
WHERE permission_set_id = ANY($1::uuid[])`, permissionSetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []authz.ObjectPermission
	for rows.Next() {
		var p authz.ObjectPermission
		if err := rows.Scan(
			&p.PermissionSetID, &p.ObjectAPIName,
			&p.CanCreate, &p.CanRead, &p.CanUpdate, &p.CanDelete,
			&p.ViewAll, &p.ModifyAll,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePermissionSet inserts a permission set and optional object perms (tests/helpers).
func (s *ObjectPermStore) CreatePermissionSet(ctx context.Context, apiName, label string, perms []authz.ObjectPermission) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `
INSERT INTO permission_sets (api_name, label) VALUES ($1, $2) RETURNING id::text`, apiName, label).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, p := range perms {
		_, err := s.Pool.Exec(ctx, `
INSERT INTO object_permissions (
  permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)`,
			id, p.ObjectAPIName, p.CanCreate, p.CanRead, p.CanUpdate, p.CanDelete, p.ViewAll, p.ModifyAll)
		if err != nil {
			return "", err
		}
	}
	return id, nil
}

// AssignPermissionSet links a user to a permission set.
func (s *ObjectPermStore) AssignPermissionSet(ctx context.Context, userID, permissionSetID string) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO user_permission_sets (user_id, permission_set_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`, userID, permissionSetID)
	return err
}
