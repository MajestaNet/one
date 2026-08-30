-- BP-047: outbound connector OAuth auth types, flow specs, tokens, and authorize state
ALTER TABLE install_connectors
  ADD COLUMN IF NOT EXISTS auth_type TEXT NOT NULL DEFAULT 'static_bearer',
  ADD COLUMN IF NOT EXISTS oauth_flow JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE install_connectors
  DROP CONSTRAINT IF EXISTS install_connectors_auth_type_check;
ALTER TABLE install_connectors
  ADD CONSTRAINT install_connectors_auth_type_check
    CHECK (auth_type IN ('static_bearer', 'oauth2_client_credentials', 'oauth2_authorization_code'));

ALTER TABLE install_connectors
  DROP CONSTRAINT IF EXISTS install_connectors_oauth_flow_object_check;
ALTER TABLE install_connectors
  ADD CONSTRAINT install_connectors_oauth_flow_object_check
    CHECK (jsonb_typeof(oauth_flow) = 'object');

CREATE TABLE IF NOT EXISTS install_connector_oauth_tokens (
  connector_api_name TEXT PRIMARY KEY
    REFERENCES install_connectors(api_name) ON DELETE CASCADE,
  token_ciphertext TEXT NOT NULL,
  expires_at TIMESTAMPTZ,
  refreshable BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS install_connector_oauth_states (
  state_hash TEXT PRIMARY KEY,
  connector_api_name TEXT NOT NULL
    REFERENCES install_connectors(api_name) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  code_verifier TEXT NOT NULL DEFAULT '',
  redirect_uri TEXT NOT NULL,
  config_hash TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS install_connector_oauth_states_expires_idx
  ON install_connector_oauth_states(expires_at);
