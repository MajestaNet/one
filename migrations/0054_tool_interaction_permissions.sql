-- Fine-grained ToolSpec interaction/promotion grants (ADR-023 / BP-055 Phase 5).
-- Preserve existing open behavior: actors who could open a Tool may interact with it.
-- Modify/publish remain explicit and default-deny; Admin receives the complete matrix.

ALTER TABLE tool_permissions
  ADD COLUMN IF NOT EXISTS can_interact boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS can_modify boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS can_publish boolean NOT NULL DEFAULT false;

UPDATE tool_permissions
SET can_interact = can_open
WHERE can_open = true;

UPDATE tool_permissions tp
SET can_open = true,
    can_interact = true,
    can_modify = true,
    can_publish = true
FROM permission_sets ps
WHERE ps.id = tp.permission_set_id
  AND ps.api_name = 'Admin';
