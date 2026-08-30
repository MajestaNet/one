-- BP-014: install secrets, connectors, egress allowlist; AgentSpec allowed_skills
CREATE TABLE IF NOT EXISTS install_secrets (
  api_name TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  ciphertext TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS install_connectors (
  api_name TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  base_url TEXT NOT NULL,
  secret_ref TEXT REFERENCES install_secrets(api_name) ON DELETE SET NULL,
  allowed_methods JSONB NOT NULL DEFAULT '["GET","POST"]'::jsonb,
  path_prefix TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS install_egress_allowlist (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  host_pattern TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS allowed_skills JSONB NOT NULL DEFAULT '[]'::jsonb;
