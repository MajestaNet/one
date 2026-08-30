-- ADR-015: nullable email for social users; auth login state + authorization codes
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
--> statement-breakpoint

-- Replace NOT NULL UNIQUE with partial unique index (multiple NULL emails allowed).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
--> statement-breakpoint
DROP INDEX IF EXISTS users_email_key;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_email_uidx ON users (lower(email)) WHERE email IS NOT NULL AND email <> '';
--> statement-breakpoint

-- OAuth state for Majesta One → IdP hop (CSRF + nonce + client PKCE binding).
CREATE TABLE IF NOT EXISTS auth_login_states (
  state_hash text PRIMARY KEY,
  provider text NOT NULL,
  client_id text NOT NULL,
  redirect_uri text NOT NULL,
  client_state text NOT NULL DEFAULT '',
  code_challenge text NOT NULL,
  code_challenge_method text NOT NULL DEFAULT 'S256',
  nonce text NOT NULL,
  idp_code_verifier text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT auth_login_states_method_check CHECK (code_challenge_method = 'S256')
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS auth_login_states_expires_idx ON auth_login_states (expires_at);
--> statement-breakpoint

-- One-time Majesta One authorization codes (hashed) for Connected App PKCE.
CREATE TABLE IF NOT EXISTS auth_authorization_codes (
  code_hash text PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_id text NOT NULL,
  redirect_uri text NOT NULL,
  code_challenge text NOT NULL,
  code_challenge_method text NOT NULL DEFAULT 'S256',
  azp text NOT NULL,
  identity_provider text NOT NULL DEFAULT '',
  identity_subject text NOT NULL DEFAULT '',
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT auth_authorization_codes_method_check CHECK (code_challenge_method = 'S256')
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS auth_authorization_codes_expires_idx ON auth_authorization_codes (expires_at);
--> statement-breakpoint

COMMENT ON TABLE auth_login_states IS 'Ephemeral OAuth state for social broker IdP hop (ADR-015)';
--> statement-breakpoint
COMMENT ON TABLE auth_authorization_codes IS 'One-time hashed auth codes for Connected App authorization_code grant (ADR-015)';
