package db

import (
	"context"
	"fmt"
)

// AutomationAccessEntry is one automation grant inside a permission set.
type AutomationAccessEntry struct {
	APIName string `json:"apiName"`
	CanRun  bool   `json:"canRun"`
}

// AutomationAccessSection is the permission-set automationAccess payload.
type AutomationAccessSection struct {
	AllAutomations bool                    `json:"allAutomations"`
	Automations    []AutomationAccessEntry `json:"automations"`
}

// EnsureAutomationInAccessCatalog upserts an automation_permissions stub for every permission set.
// Admin (or all_automations=true) receives can_run=true; all other sets receive deny stubs.
// Existing grants are preserved (ON CONFLICT DO NOTHING).
func EnsureAutomationInAccessCatalog(ctx context.Context, pool *Pool, automationAPIName string) error {
	if pool == nil || automationAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
INSERT INTO automation_permissions (permission_set_id, automation_api_name, can_run)
SELECT
  ps.id,
  $1,
  CASE WHEN ps.api_name = $2 OR ps.all_automations THEN true ELSE false END
FROM permission_sets ps
ON CONFLICT (permission_set_id, automation_api_name) DO NOTHING`,
		automationAPIName, adminPermissionSetAPIName)
	return err
}

// BackfillPermissionSetAutomationAccess ensures one permission set has stubs for every
// metadata automation. Admin / all_automations → can_run true; others deny.
func BackfillPermissionSetAutomationAccess(ctx context.Context, pool *Pool, permissionSetID string) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	var apiName string
	var allAutomations bool
	if err := pool.QueryRow(ctx, `
SELECT api_name, all_automations FROM permission_sets WHERE id = $1::uuid`, permissionSetID,
	).Scan(&apiName, &allAutomations); err != nil {
		return fmt.Errorf("load permission set: %w", err)
	}
	canRun := apiName == adminPermissionSetAPIName || allAutomations
	_, err := pool.Exec(ctx, `
INSERT INTO automation_permissions (permission_set_id, automation_api_name, can_run)
SELECT $1::uuid, a.api_name, $2
FROM metadata_automations a
ON CONFLICT (permission_set_id, automation_api_name) DO NOTHING`,
		permissionSetID, canRun)
	if err != nil {
		return fmt.Errorf("backfill automation access: %w", err)
	}
	return nil
}

// RemoveAutomationFromAccessCatalog deletes automation_permissions rows for one automation.
func RemoveAutomationFromAccessCatalog(ctx context.Context, pool *Pool, automationAPIName string) error {
	if pool == nil || automationAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
DELETE FROM automation_permissions WHERE automation_api_name = $1`, automationAPIName)
	return err
}

// LoadAutomationAccessSection returns automationAccess for a permission set.
func LoadAutomationAccessSection(ctx context.Context, pool *Pool, permissionSetID string) (AutomationAccessSection, error) {
	out := AutomationAccessSection{
		Automations: []AutomationAccessEntry{},
	}
	if pool == nil || permissionSetID == "" {
		return out, nil
	}
	if err := pool.QueryRow(ctx, `
SELECT all_automations FROM permission_sets WHERE id = $1::uuid`, permissionSetID,
	).Scan(&out.AllAutomations); err != nil {
		return out, err
	}
	rows, err := pool.Query(ctx, `
SELECT automation_api_name, can_run
FROM automation_permissions
WHERE permission_set_id = $1::uuid
ORDER BY automation_api_name`, permissionSetID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e AutomationAccessEntry
		if err := rows.Scan(&e.APIName, &e.CanRun); err != nil {
			return out, err
		}
		out.Automations = append(out.Automations, e)
	}
	return out, rows.Err()
}

// SetPermissionSetAllAutomations updates the all_automations flag on a permission set.
func SetPermissionSetAllAutomations(ctx context.Context, pool *Pool, permissionSetID string, all bool) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE permission_sets SET all_automations = $2 WHERE id = $1::uuid`, permissionSetID, all)
	return err
}

// UpsertAutomationAccessEntries merges automation can_run grants for a permission set.
func UpsertAutomationAccessEntries(ctx context.Context, pool *Pool, permissionSetID string, entries []AutomationAccessEntry) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	for _, e := range entries {
		if e.APIName == "" {
			continue
		}
		_, err := pool.Exec(ctx, `
INSERT INTO automation_permissions (permission_set_id, automation_api_name, can_run)
VALUES ($1::uuid, $2, $3)
ON CONFLICT (permission_set_id, automation_api_name) DO UPDATE SET can_run = EXCLUDED.can_run`,
			permissionSetID, e.APIName, e.CanRun)
		if err != nil {
			return err
		}
	}
	return nil
}

// EnsureAdminAllAutomations sets all_automations=true on the Admin permission set when present.
func EnsureAdminAllAutomations(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE permission_sets SET all_automations = true WHERE api_name = $1`, adminPermissionSetAPIName)
	return err
}
