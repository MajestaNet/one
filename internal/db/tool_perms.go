package db

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
)

// ToolPermStore implements authz.ToolPermissionStore.
type ToolPermStore struct {
	Pool *Pool
}

var _ authz.ToolPermissionStore = (*ToolPermStore)(nil)

// ListByPermissionSets returns tool_permissions for the given set ids.
func (s *ToolPermStore) ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]authz.ToolPermission, error) {
	if s == nil || s.Pool == nil || len(permissionSetIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT permission_set_id::text, tool_api_name, can_open, can_interact, can_modify, can_publish
FROM tool_permissions
WHERE permission_set_id = ANY($1::uuid[])`, permissionSetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []authz.ToolPermission
	for rows.Next() {
		var p authz.ToolPermission
		if err := rows.Scan(&p.PermissionSetID, &p.ToolAPIName, &p.CanOpen, &p.CanInteract, &p.CanModify, &p.CanPublish); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AnyAllTools is true when any listed permission set has all_tools=true.
func (s *ToolPermStore) AnyAllTools(ctx context.Context, permissionSetIDs []string) (bool, error) {
	if s == nil || s.Pool == nil || len(permissionSetIDs) == 0 {
		return false, nil
	}
	var ok bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM permission_sets
  WHERE id = ANY($1::uuid[]) AND all_tools = true
)`, permissionSetIDs).Scan(&ok)
	return ok, err
}
