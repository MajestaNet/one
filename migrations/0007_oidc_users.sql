-- OIDC principals (Cognito / human SSO)
ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_sub text;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_oidc_sub_uniq ON users(oidc_sub) WHERE oidc_sub IS NOT NULL;
