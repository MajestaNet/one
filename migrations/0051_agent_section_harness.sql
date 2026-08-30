-- BP-053: AgentSpec primary section + product harness binding
ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS primary_section TEXT
    CHECK (primary_section IS NULL OR primary_section IN
      ('operate','run','build','ship','govern','settings')),
  ADD COLUMN IF NOT EXISTS harness_id TEXT,
  ADD COLUMN IF NOT EXISTS harness_version TEXT NOT NULL DEFAULT '';

-- Starter AgentSpecs → section homes
UPDATE agent_playbooks
SET primary_section = 'govern',
    harness_id = 'harness.govern.admin',
    harness_version = '1'
WHERE api_name = 'AdminSetup'
  AND (primary_section IS NULL OR primary_section = '');

UPDATE agent_playbooks
SET primary_section = 'build',
    harness_id = 'harness.build.metadata',
    harness_version = '1'
WHERE api_name = 'MetadataBuilder'
  AND (primary_section IS NULL OR primary_section = '');

UPDATE agent_playbooks
SET primary_section = 'run',
    harness_id = 'harness.run.tools',
    harness_version = '1'
WHERE api_name = 'RunCoach'
  AND (primary_section IS NULL OR primary_section = '');

-- Remaining customs / unknown → Operate query harness
UPDATE agent_playbooks
SET primary_section = 'operate',
    harness_id = 'harness.operate.query',
    harness_version = '1'
WHERE primary_section IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_playbooks_primary_section
  ON agent_playbooks (primary_section);
