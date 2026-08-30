-- Integration configurations (Connected Apps) — inbound OAuth clients (ADR-006).
CREATE TABLE IF NOT EXISTS integration_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  description text NOT NULL DEFAULT '',
  principal_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  client_kind text NOT NULL CHECK (client_kind IN ('public', 'confidential')),
  oauth_flows text[] NOT NULL DEFAULT '{}',
  callback_urls text[] NOT NULL DEFAULT '{}',
  logout_urls text[] NOT NULL DEFAULT '{}',
  allowed_scopes_hint text[] NOT NULL DEFAULT '{}',
  pkce_required boolean NOT NULL DEFAULT false,
  ownership text NOT NULL DEFAULT 'custom' CHECK (ownership IN ('managed', 'custom')),
  package_name text,
  is_active boolean NOT NULL DEFAULT true,
  cognito_app_client_id text,
  one_secret_enc text,
  cognito_secret_enc text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_integration_configs_principal ON integration_configs (principal_id);
CREATE INDEX IF NOT EXISTS idx_integration_configs_ownership ON integration_configs (ownership);
CREATE INDEX IF NOT EXISTS idx_integration_configs_active ON integration_configs (is_active);
