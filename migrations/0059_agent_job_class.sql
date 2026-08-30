-- BP-064: optional AgentSpec job class (SoR); primary_section remains a compatibility alias.
ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS job_class TEXT
  CHECK (job_class IS NULL OR job_class IN
    ('query','customize','ship','govern','operate','skill'));

CREATE INDEX IF NOT EXISTS idx_agent_playbooks_job_class
  ON agent_playbooks (job_class);
