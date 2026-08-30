package db

import (
	"context"
	"fmt"
)

// DataAccessObjectPermission is one object row in a permission set's data-access section.
type DataAccessObjectPermission struct {
	ObjectAPIName string `json:"objectApiName"`
	CanCreate     bool   `json:"canCreate"`
	CanRead       bool   `json:"canRead"`
	CanUpdate     bool   `json:"canUpdate"`
	CanDelete     bool   `json:"canDelete"`
	ViewAll       bool   `json:"viewAll"`
	ModifyAll     bool   `json:"modifyAll"`
}

// DataAccessFieldPermission is one field row in a permission set's data-access section.
// Configured is false only when a catalog row is missing (should be rare after FLS freeze).
type DataAccessFieldPermission struct {
	ObjectAPIName string `json:"objectApiName"`
	FieldAPIName  string `json:"fieldApiName"`
	CanRead       bool   `json:"canRead"`
	CanEdit       bool   `json:"canEdit"`
	Configured    bool   `json:"configured"`
}

// DataAccessSection is the permission-set "data access" matrix (objects + fields).
type DataAccessSection struct {
	ObjectPermissions []DataAccessObjectPermission `json:"objectPermissions"`
	FieldPermissions  []DataAccessFieldPermission  `json:"fieldPermissions"`
}

const adminPermissionSetAPIName = "Admin"

// EnsureObjectInDataAccessCatalog upserts an object_permissions stub for every permission set.
// Admin receives full CRUD + view/modify all; all other sets receive deny stubs.
// Existing grants are preserved (ON CONFLICT DO NOTHING).
func EnsureObjectInDataAccessCatalog(ctx context.Context, pool *Pool, objectAPIName string) error {
	if pool == nil || objectAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
INSERT INTO object_permissions (
  permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
)
SELECT
  ps.id,
  $1,
  CASE WHEN ps.api_name = $2 THEN true ELSE false END,
  CASE WHEN ps.api_name = $2 OR $1 = 'User' THEN true ELSE false END,
  CASE WHEN ps.api_name = $2 THEN true ELSE false END,
  CASE WHEN ps.api_name = $2 THEN true ELSE false END,
  CASE WHEN ps.api_name = $2 THEN true ELSE false END,
  CASE WHEN ps.api_name = $2 THEN true ELSE false END
FROM permission_sets ps
ON CONFLICT (permission_set_id, object_api_name) DO NOTHING`,
		objectAPIName, adminPermissionSetAPIName)
	return err
}

// EnsureUserObjectDescribeAccess grants object read on User for every permission set so
// Client/Metadata describe can list the kernel identity object. Create/update/delete stay
// Admin-only; record object CRUD is fenced by DataEngine (ADR-026).
func EnsureUserObjectDescribeAccess(ctx context.Context, pool *Pool) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
UPDATE object_permissions SET can_read = true WHERE object_api_name = 'User'`)
	return err
}

// EnsureFieldInDataAccessCatalog upserts a field_permissions stub for every permission set.
// Admin receives read+edit; all other sets receive deny stubs (deny-by-default FLS).
// Existing grants are preserved (ON CONFLICT DO NOTHING).
func EnsureFieldInDataAccessCatalog(ctx context.Context, pool *Pool, objectAPIName, fieldAPIName string) error {
	if pool == nil || objectAPIName == "" || fieldAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
INSERT INTO field_permissions (permission_set_id, object_api_name, field_api_name, can_read, can_edit)
SELECT
  ps.id, $1, $2,
  CASE WHEN ps.api_name = $3 THEN true ELSE false END,
  CASE WHEN ps.api_name = $3 THEN true ELSE false END
FROM permission_sets ps
ON CONFLICT (permission_set_id, object_api_name, field_api_name) DO NOTHING`,
		objectAPIName, fieldAPIName, adminPermissionSetAPIName)
	return err
}

// BackfillPermissionSetDataAccess ensures one permission set has object + field stubs for every
// metadata object/field. Admin receives grants; others receive deny stubs. Safe to call after create.
func BackfillPermissionSetDataAccess(ctx context.Context, pool *Pool, permissionSetID string) error {
	if pool == nil || permissionSetID == "" {
		return nil
	}
	var apiName string
	if err := pool.QueryRow(ctx, `SELECT api_name FROM permission_sets WHERE id = $1::uuid`, permissionSetID).Scan(&apiName); err != nil {
		return fmt.Errorf("load permission set: %w", err)
	}
	isAdmin := apiName == adminPermissionSetAPIName

	_, err := pool.Exec(ctx, `
INSERT INTO object_permissions (
  permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
)
SELECT $1::uuid, o.api_name, $2, ($2 OR o.api_name = 'User'), $2, $2, $2, $2
FROM metadata_objects o
ON CONFLICT (permission_set_id, object_api_name) DO NOTHING`,
		permissionSetID, isAdmin)
	if err != nil {
		return fmt.Errorf("backfill object data access: %w", err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE object_permissions SET can_read = true
WHERE permission_set_id = $1::uuid AND object_api_name = 'User'`, permissionSetID); err != nil {
		return fmt.Errorf("user describe access: %w", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO field_permissions (permission_set_id, object_api_name, field_api_name, can_read, can_edit)
SELECT $1::uuid, f.object_api_name, f.api_name, $2, $2
FROM metadata_fields f
ON CONFLICT (permission_set_id, object_api_name, field_api_name) DO NOTHING`,
		permissionSetID, isAdmin)
	if err != nil {
		return fmt.Errorf("backfill field data access: %w", err)
	}
	return nil
}

// RemoveObjectFromDataAccessCatalog deletes object and field permission rows for an object.
func RemoveObjectFromDataAccessCatalog(ctx context.Context, pool *Pool, objectAPIName string) error {
	if pool == nil || objectAPIName == "" {
		return nil
	}
	if _, err := pool.Exec(ctx, `DELETE FROM field_permissions WHERE object_api_name = $1`, objectAPIName); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name = $1`, objectAPIName)
	return err
}

// RemoveFieldFromDataAccessCatalog deletes field permission rows for one field.
func RemoveFieldFromDataAccessCatalog(ctx context.Context, pool *Pool, objectAPIName, fieldAPIName string) error {
	if pool == nil || objectAPIName == "" || fieldAPIName == "" {
		return nil
	}
	_, err := pool.Exec(ctx, `
DELETE FROM field_permissions WHERE object_api_name = $1 AND field_api_name = $2`,
		objectAPIName, fieldAPIName)
	return err
}

// LoadDataAccessSection returns the full data-access matrix for a permission set.
// Object rows come from storage (catalog stubs). Field rows merge stored grants with the
// metadata field catalog; missing stored rows default to deny (deny-by-default FLS).
func LoadDataAccessSection(ctx context.Context, pool *Pool, permissionSetID string) (DataAccessSection, error) {
	out := DataAccessSection{
		ObjectPermissions: []DataAccessObjectPermission{},
		FieldPermissions:  []DataAccessFieldPermission{},
	}
	if pool == nil || permissionSetID == "" {
		return out, nil
	}

	opRows, err := pool.Query(ctx, `
SELECT object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
FROM object_permissions
WHERE permission_set_id = $1::uuid
ORDER BY object_api_name`, permissionSetID)
	if err != nil {
		return out, err
	}
	for opRows.Next() {
		var p DataAccessObjectPermission
		if err := opRows.Scan(&p.ObjectAPIName, &p.CanCreate, &p.CanRead, &p.CanUpdate, &p.CanDelete, &p.ViewAll, &p.ModifyAll); err != nil {
			opRows.Close()
			return out, err
		}
		out.ObjectPermissions = append(out.ObjectPermissions, p)
	}
	opRows.Close()
	if err := opRows.Err(); err != nil {
		return out, err
	}

	stored := map[string]DataAccessFieldPermission{}
	fpRows, err := pool.Query(ctx, `
SELECT object_api_name, field_api_name, can_read, can_edit
FROM field_permissions
WHERE permission_set_id = $1::uuid
ORDER BY object_api_name, field_api_name`, permissionSetID)
	if err != nil {
		return out, err
	}
	for fpRows.Next() {
		var p DataAccessFieldPermission
		if err := fpRows.Scan(&p.ObjectAPIName, &p.FieldAPIName, &p.CanRead, &p.CanEdit); err != nil {
			fpRows.Close()
			return out, err
		}
		p.Configured = true
		stored[p.ObjectAPIName+"\x00"+p.FieldAPIName] = p
	}
	fpRows.Close()
	if err := fpRows.Err(); err != nil {
		return out, err
	}

	metaRows, err := pool.Query(ctx, `
SELECT object_api_name, api_name
FROM metadata_fields
ORDER BY object_api_name, api_name`)
	if err != nil {
		return out, err
	}
	defer metaRows.Close()
	for metaRows.Next() {
		var objectAPIName, fieldAPIName string
		if err := metaRows.Scan(&objectAPIName, &fieldAPIName); err != nil {
			return out, err
		}
		key := objectAPIName + "\x00" + fieldAPIName
		if p, ok := stored[key]; ok {
			out.FieldPermissions = append(out.FieldPermissions, p)
			delete(stored, key)
			continue
		}
		// Deny-by-default catalog gap (should be rare after freeze / EnsureField stubs).
		out.FieldPermissions = append(out.FieldPermissions, DataAccessFieldPermission{
			ObjectAPIName: objectAPIName,
			FieldAPIName:  fieldAPIName,
			CanRead:       false,
			CanEdit:       false,
			Configured:    false,
		})
	}
	if err := metaRows.Err(); err != nil {
		return out, err
	}
	// Orphan stored rows (field deleted from metadata but grant left behind).
	for _, p := range stored {
		out.FieldPermissions = append(out.FieldPermissions, p)
	}
	return out, nil
}

// GrantAdminObjectAccess upserts full CRUD for the system Admin permission set.
// Deprecated: prefer EnsureObjectInDataAccessCatalog, which covers every permission set.
func GrantAdminObjectAccess(ctx context.Context, pool *Pool, objectAPIName string) error {
	return EnsureObjectInDataAccessCatalog(ctx, pool, objectAPIName)
}
