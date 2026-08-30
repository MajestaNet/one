-- Client Experience metadata (ADR-019 Phase 4).

CREATE TABLE IF NOT EXISTS metadata_experiences (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  description text NOT NULL DEFAULT '',
  home_url text NOT NULL DEFAULT '',
  connected_app_api_name text NOT NULL DEFAULT '',
  allowed_origins jsonb NOT NULL DEFAULT '[]'::jsonb,
  active boolean NOT NULL DEFAULT true,
  ownership text NOT NULL DEFAULT 'custom',
  package_name text NOT NULL DEFAULT 'customer.default',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_metadata_experiences_active ON metadata_experiences (active) WHERE active = true;
