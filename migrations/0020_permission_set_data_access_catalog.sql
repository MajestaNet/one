-- Backfill permission-set data-access catalog for existing installs.
-- Every permission set gets an object_permissions row for every metadata object
-- (Admin = full grant; others = deny stubs). Admin also gets field_permissions
-- for every metadata field (read+edit). Non-Admin field rows stay sparse;
-- Metadata GET expands the field matrix with allow-if-absent defaults.

INSERT INTO object_permissions (
  permission_set_id, object_api_name, can_create, can_read, can_update, can_delete, view_all, modify_all
)
SELECT
  ps.id,
  o.api_name,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END,
  CASE WHEN ps.api_name = 'Admin' THEN true ELSE false END
FROM permission_sets ps
CROSS JOIN metadata_objects o
ON CONFLICT (permission_set_id, object_api_name) DO NOTHING;

INSERT INTO field_permissions (
  permission_set_id, object_api_name, field_api_name, can_read, can_edit
)
SELECT ps.id, f.object_api_name, f.api_name, true, true
FROM permission_sets ps
CROSS JOIN metadata_fields f
WHERE ps.api_name = 'Admin'
ON CONFLICT (permission_set_id, object_api_name, field_api_name) DO NOTHING;
