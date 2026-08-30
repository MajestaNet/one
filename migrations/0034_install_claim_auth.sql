-- Install claim, customer SSO settings, password credentials (BP-037).

ALTER TABLE organization_settings
  ADD COLUMN IF NOT EXISTS claimed_at timestamptz,
  ADD COLUMN IF NOT EXISTS claim_token_hash text,
  ADD COLUMN IF NOT EXISTS oidc_issuer text,
  ADD COLUMN IF NOT EXISTS oidc_audience text,
  ADD COLUMN IF NOT EXISTS oidc_jwks_uri text,
  ADD COLUMN IF NOT EXISTS oidc_display_name text,
  ADD COLUMN IF NOT EXISTS oidc_client_id text,
  ADD COLUMN IF NOT EXISTS oidc_client_secret_enc text,
  ADD COLUMN IF NOT EXISTS jit_provision_users boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS jit_default_role text NOT NULL DEFAULT 'StandardUser',
  ADD COLUMN IF NOT EXISTS allowed_email_domains text[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS social_providers text[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS password_login_enabled boolean NOT NULL DEFAULT true;
--> statement-breakpoint

ALTER TABLE principal_credentials
  DROP CONSTRAINT IF EXISTS principal_credentials_kind_check;
--> statement-breakpoint

ALTER TABLE principal_credentials
  ADD CONSTRAINT principal_credentials_kind_check
  CHECK (credential_kind IN ('client_secret', 'bootstrap_api_key', 'password'));
