-- Auth principals: Majesta One JWT Token Service (ADR-006 / BP-013 P0)
ALTER TABLE users ADD COLUMN IF NOT EXISTS principal_type text NOT NULL DEFAULT 'human';
--> statement-breakpoint
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'users_principal_type_check'
  ) THEN
    ALTER TABLE users ADD CONSTRAINT users_principal_type_check
      CHECK (principal_type IN ('human', 'service', 'agent'));
  END IF;
END $$;
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS user_roles (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS role_permission_sets (
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_set_id)
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS role_api_scopes (
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  scope text NOT NULL,
  PRIMARY KEY (role_id, scope),
  CONSTRAINT role_api_scopes_scope_check CHECK (scope IN ('client', 'metadata', 'deploy', 'admin'))
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS principal_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_kind text NOT NULL DEFAULT 'client_secret',
  secret_hash text NOT NULL,
  label text,
  expires_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT principal_credentials_kind_check CHECK (credential_kind IN ('client_secret', 'bootstrap_api_key'))
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS principal_credentials_user_id_idx ON principal_credentials(user_id);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS identity_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider text NOT NULL,
  issuer text,
  subject text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT identity_links_provider_subject_uniq UNIQUE (provider, issuer, subject)
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS identity_links_user_id_idx ON identity_links(user_id);
