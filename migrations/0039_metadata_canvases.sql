-- CanvasSpec metadata (ADR-018 Phase 3) + AgentSpec canvas allowlist.

CREATE TABLE IF NOT EXISTS metadata_canvases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  description text NOT NULL DEFAULT '',
  layout jsonb NOT NULL,
  nodes jsonb NOT NULL DEFAULT '[]'::jsonb,
  data_bindings jsonb NOT NULL DEFAULT '[]'::jsonb,
  active boolean NOT NULL DEFAULT true,
  ownership text NOT NULL DEFAULT 'custom',
  package_name text NOT NULL DEFAULT 'customer.default',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_metadata_canvases_active ON metadata_canvases (active) WHERE active = true;

ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS allowed_canvas_specs jsonb NOT NULL DEFAULT '[]'::jsonb;
