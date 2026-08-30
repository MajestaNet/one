-- Code automation metadata (ADR-014 Phase 2).
-- runtime: actions (legacy JSON) | code (TypeScript guest)
-- execution: async (default) | sync (same-tx; Phase 3)
-- entry_file / source: customer repo path + embedded source for code runtime
-- run_as_principal_id: required when trigger_event = schedule

ALTER TABLE metadata_automations
  ADD COLUMN IF NOT EXISTS runtime text NOT NULL DEFAULT 'actions',
  ADD COLUMN IF NOT EXISTS execution text NOT NULL DEFAULT 'async',
  ADD COLUMN IF NOT EXISTS entry_file text,
  ADD COLUMN IF NOT EXISTS source text,
  ADD COLUMN IF NOT EXISTS run_as_principal_id uuid REFERENCES users(id);

CREATE INDEX IF NOT EXISTS metadata_automations_runtime_idx
  ON metadata_automations (runtime);

COMMENT ON COLUMN metadata_automations.runtime IS 'actions | code';
COMMENT ON COLUMN metadata_automations.execution IS 'async | sync';
COMMENT ON COLUMN metadata_automations.entry_file IS 'Repo-relative path e.g. src/automations/foo.ts';
COMMENT ON COLUMN metadata_automations.source IS 'Embedded TypeScript source for code runtime';
COMMENT ON COLUMN metadata_automations.run_as_principal_id IS 'Required for schedule triggers; otherwise starter principal at runtime';
