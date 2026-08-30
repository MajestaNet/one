package db

import (
	"context"
	"fmt"
)

// ToolAccessEntry is one ToolSpec grant inside a permission set.
type ToolAccessEntry struct {
	APIName     string `json:"apiName"`
	CanOpen     bool   `json:"canOpen"`
	CanInteract bool   `json:"canInteract"`
	CanModify   bool   `json:"canModify"`
	CanPublish  bool   `json:"canPublish"`
}

// ToolAccessSection is the permission-set toolAccess payload.
type ToolAccessSection struct {
	AllTools bool              `json:"allTools"`
	Tools    []ToolAccessEntry `json:"tools"`
}

// EnsureToolInAccessCatalog upserts a tool_permissions stub for every permission set.
// Admin/all_tools receive can_open=true. Operate receives managed sales/service tools.
// Existing grants are preserved (ON CONFLICT DO NOTHING).
func EnsureToolInAccessCatalog(ctx context.Context, pool *Pool, toolAPIName string) error {
	if pool == nil || toolAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
INSERT INTO tool_permissions (permission_set_id, tool_api_name, can_open, can_interact, can_modify, can_publish)
SELECT
  ps.id,
  $1,
  CASE
    WHEN ps.api_name = $2 OR ps.all_tools THEN true
    WHEN ps.api_name = 'Operate'
      AND c.ownership = 'managed'
      AND c.package_name IN ('sales', 'service')
      THEN true
    ELSE false
  END,
  CASE
    WHEN ps.api_name = $2 OR ps.all_tools THEN true
    WHEN ps.api_name = 'Operate' AND c.ownership = 'managed' AND c.package_name IN ('sales', 'service') THEN true
    ELSE false
  END,
  ps.api_name = $2,
  ps.api_name = $2
FROM permission_sets ps
LEFT JOIN metadata_canvases c ON c.api_name = $1
ON CONFLICT (permission_set_id, tool_api_name) DO NOTHING`,
		toolAPIName, adminPermissionSetAPIName)
	return err
}

// BackfillPermissionSetToolAccess ensures one permission set has stubs for every
// ToolSpec. Admin/all_tools receive grants; Operate receives managed sales/service tools.
func BackfillPermissionSetToolAccess(ctx context.Context, pool *Pool, permissionSetID string) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	var apiName string
	var allTools bool
	if err := pool.QueryRow(ctx, `
SELECT api_name, all_tools FROM permission_sets WHERE id = $1::uuid`, permissionSetID,
	).Scan(&apiName, &allTools); err != nil {
		return fmt.Errorf("load permission set: %w", err)
	}
	_, err := pool.Exec(ctx, `
INSERT INTO tool_permissions (permission_set_id, tool_api_name, can_open, can_interact, can_modify, can_publish)
SELECT
  $1::uuid,
  c.api_name,
  CASE
    WHEN $2::boolean THEN true
    WHEN $4::boolean
      AND c.ownership = 'managed'
      AND c.package_name IN ('sales', 'service')
      THEN true
    ELSE false
  END,
  CASE
    WHEN $2::boolean THEN true
    WHEN $4::boolean AND c.ownership = 'managed' AND c.package_name IN ('sales', 'service') THEN true
    ELSE false
  END,
  $3::boolean,
  $3::boolean
FROM metadata_canvases c
ON CONFLICT (permission_set_id, tool_api_name) DO NOTHING`,
		permissionSetID, apiName == adminPermissionSetAPIName || allTools, apiName == adminPermissionSetAPIName, apiName == "Operate")
	if err != nil {
		return fmt.Errorf("backfill tool access: %w", err)
	}
	return nil
}

// RemoveToolFromAccessCatalog deletes tool_permissions rows for one ToolSpec.
func RemoveToolFromAccessCatalog(ctx context.Context, pool *Pool, toolAPIName string) error {
	if pool == nil || toolAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
DELETE FROM tool_permissions WHERE tool_api_name = $1`, toolAPIName)
	return err
}

// LoadToolAccessSection returns toolAccess for a permission set.
func LoadToolAccessSection(ctx context.Context, pool *Pool, permissionSetID string) (ToolAccessSection, error) {
	out := ToolAccessSection{
		Tools: []ToolAccessEntry{},
	}
	if pool == nil || permissionSetID == "" {
		return out, nil
	}
	if err := pool.QueryRow(ctx, `
SELECT all_tools FROM permission_sets WHERE id = $1::uuid`, permissionSetID,
	).Scan(&out.AllTools); err != nil {
		return out, err
	}
	rows, err := pool.Query(ctx, `
SELECT tool_api_name, can_open, can_interact, can_modify, can_publish
FROM tool_permissions
WHERE permission_set_id = $1::uuid
ORDER BY tool_api_name`, permissionSetID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e ToolAccessEntry
		if err := rows.Scan(&e.APIName, &e.CanOpen, &e.CanInteract, &e.CanModify, &e.CanPublish); err != nil {
			return out, err
		}
		out.Tools = append(out.Tools, e)
	}
	return out, rows.Err()
}

// SetPermissionSetAllTools updates the all_tools flag on a permission set.
func SetPermissionSetAllTools(ctx context.Context, pool *Pool, permissionSetID string, all bool) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE permission_sets SET all_tools = $2 WHERE id = $1::uuid`, permissionSetID, all)
	return err
}

// UpsertToolAccessEntries merges the ToolSpec permission matrix for a permission set.
func UpsertToolAccessEntries(ctx context.Context, pool *Pool, permissionSetID string, entries []ToolAccessEntry) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	for _, e := range entries {
		if e.APIName == "" {
			continue
		}
		canOpen := e.CanOpen || e.CanInteract || e.CanModify || e.CanPublish
		_, err := pool.Exec(ctx, `
INSERT INTO tool_permissions (permission_set_id, tool_api_name, can_open, can_interact, can_modify, can_publish)
VALUES ($1::uuid, $2, $3, $4, $5, $6)
ON CONFLICT (permission_set_id, tool_api_name) DO UPDATE SET
  can_open = EXCLUDED.can_open,
  can_interact = EXCLUDED.can_interact,
  can_modify = EXCLUDED.can_modify,
  can_publish = EXCLUDED.can_publish`,
			permissionSetID, e.APIName, canOpen, e.CanInteract, e.CanModify, e.CanPublish)
		if err != nil {
			return err
		}
	}
	return nil
}

// EnsureAdminAllTools sets all_tools=true on the Admin permission set when present.
func EnsureAdminAllTools(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE permission_sets SET all_tools = true WHERE api_name = $1`, adminPermissionSetAPIName)
	return err
}
