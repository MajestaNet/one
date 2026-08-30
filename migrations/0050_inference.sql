-- BP-052: install inference config, BYO providers, agent run stream events
CREATE TABLE IF NOT EXISTS install_inference_providers (
  api_name TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  base_url TEXT NOT NULL,
  secret_ref TEXT REFERENCES install_secrets(api_name) ON DELETE SET NULL,
  default_model TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS install_inference_config (
  id INT PRIMARY KEY CHECK (id = 1),
  active_source TEXT NOT NULL DEFAULT 'none'
    CHECK (active_source IN ('none', 'digitalocean', 'byo')),
  do_enabled BOOLEAN NOT NULL DEFAULT false,
  do_mode TEXT CHECK (do_mode IS NULL OR do_mode IN ('dev', 'standard', 'pro')),
  default_provider_api_name TEXT REFERENCES install_inference_providers(api_name) ON DELETE SET NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO install_inference_config (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_run_events (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  seq INT NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);

CREATE INDEX IF NOT EXISTS agent_run_events_run_seq_idx ON agent_run_events (run_id, seq);
