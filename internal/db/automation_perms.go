package db

import (
	"context"

	"github.com/MajestaNet/ide/internal/authz"
)

// AutomationPermStore implements authz.AutomationPermissionStore.
type AutomationPermStore struct {
	Pool *Pool
}

var _ authz.AutomationPermissionStore = (*AutomationPermStore)(nil)

// ListByPermissionSets returns automation_permissions for the given set ids.
func (s *AutomationPermStore) ListByPermissionSets(ctx context.Context, permissionSetIDs []string) ([]authz.AutomationPermission, error) {
	if s == nil || s.Pool == nil || len(permissionSetIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT permission_set_id::text, automation_api_name, can_run
FROM automation_permissions
WHERE permission_set_id = ANY($1::uuid[])`, permissionSetIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []authz.AutomationPermission
	for rows.Next() {
		var p authz.AutomationPermission
		if err := rows.Scan(&p.PermissionSetID, &p.AutomationAPIName, &p.CanRun); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AnyAllAutomations is true when any listed permission set has all_automations=true.
func (s *AutomationPermStore) AnyAllAutomations(ctx context.Context, permissionSetIDs []string) (bool, error) {
	if s == nil || s.Pool == nil || len(permissionSetIDs) == 0 {
		return false, nil
	}
	var ok bool
	err := s.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM permission_sets
  WHERE id = ANY($1::uuid[]) AND all_automations = true
)`, permissionSetIDs).Scan(&ok)
	return ok, err
}
