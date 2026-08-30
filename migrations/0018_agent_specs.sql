-- AgentSpec enrichment on agent_playbooks (ADR-010)
ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS instructions text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS ownership text NOT NULL DEFAULT 'custom',
  ADD COLUMN IF NOT EXISTS package_name text NOT NULL DEFAULT 'customer.default',
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_agent_playbooks_ownership ON agent_playbooks (ownership);
