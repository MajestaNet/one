-- Automation access catalog on permission sets (ADR-014 Phase 1).
-- Every PS gets a row per metadata_automations api_name (Admin can_run=true; others deny).
-- all_automations on the permission set is the broad grant (Admin default true).

ALTER TABLE permission_sets
  ADD COLUMN IF NOT EXISTS all_automations boolean NOT NULL DEFAULT false;

UPDATE permission_sets
SET all_automations = true
WHERE api_name = 'Admin';

CREATE TABLE IF NOT EXISTS automation_permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
  automation_api_name text NOT NULL,
  can_run boolean NOT NULL DEFAULT false,
  UNIQUE (permission_set_id, automation_api_name)
);

CREATE INDEX IF NOT EXISTS automation_permissions_ps_idx
  ON automation_permissions (permission_set_id);

-- Backfill stubs for existing automations on every permission set.
INSERT INTO automation_permissions (permission_set_id, automation_api_name, can_run)
SELECT
  ps.id,
  a.api_name,
  CASE WHEN ps.api_name = 'Admin' OR ps.all_automations THEN true ELSE false END
FROM permission_sets ps
CROSS JOIN metadata_automations a
ON CONFLICT (permission_set_id, automation_api_name) DO NOTHING;
