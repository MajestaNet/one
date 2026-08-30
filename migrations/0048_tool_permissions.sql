-- ToolSpec access catalog on permission sets (ADR-021 / BP-050 P1).
-- Every PS gets a row per metadata_canvases api_name.
-- Admin/all_tools can_open=true; Operate can_open=true for managed sales/service tools.

ALTER TABLE permission_sets
  ADD COLUMN IF NOT EXISTS all_tools boolean NOT NULL DEFAULT false;

UPDATE permission_sets
SET all_tools = true
WHERE api_name = 'Admin';

CREATE TABLE IF NOT EXISTS tool_permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
  tool_api_name text NOT NULL,
  can_open boolean NOT NULL DEFAULT false,
  UNIQUE (permission_set_id, tool_api_name)
);

CREATE INDEX IF NOT EXISTS tool_permissions_ps_idx
  ON tool_permissions (permission_set_id);

-- Backfill stubs for existing ToolSpecs on every permission set.
INSERT INTO tool_permissions (permission_set_id, tool_api_name, can_open)
SELECT
  ps.id,
  c.api_name,
  CASE
    WHEN ps.api_name = 'Admin' OR ps.all_tools THEN true
    WHEN ps.api_name = 'Operate'
      AND c.ownership = 'managed'
      AND c.package_name IN ('sales', 'service')
      THEN true
    ELSE false
  END
FROM permission_sets ps
CROSS JOIN metadata_canvases c
ON CONFLICT (permission_set_id, tool_api_name) DO NOTHING;
