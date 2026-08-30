-- BP-017 Phases 1–3: system Roles, freeze lifecycle, SCIM-shaped profile columns.
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_system boolean NOT NULL DEFAULT false;
--> statement-breakpoint
UPDATE roles SET is_system = true
WHERE api_name IN ('SystemAdmin', 'StandardUser', 'MetadataDeveloper', 'DeployBot');
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS frozen_at timestamptz;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS frozen_reason text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS user_name text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS given_name text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS family_name text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_number text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS locale text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS title text;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS department text;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_user_name_uidx ON users (user_name) WHERE user_name IS NOT NULL;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_external_id_uidx ON users (external_id) WHERE external_id IS NOT NULL;
