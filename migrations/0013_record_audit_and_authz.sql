-- Record audit fields, optional OwnerId, AuthZ packaging (Role=scopes, PS=user grants),
-- principal_type human → user.

-- 1) Record audit columns + nullable owner
ALTER TABLE records ADD COLUMN IF NOT EXISTS created_by_id uuid;
ALTER TABLE records ADD COLUMN IF NOT EXISTS last_modified_by_id uuid;

UPDATE records
SET created_by_id = owner_id,
    last_modified_by_id = owner_id
WHERE created_by_id IS NULL OR last_modified_by_id IS NULL;

-- Any leftover nulls (should not happen): attach to first admin/service user.
UPDATE records
SET created_by_id = (
  SELECT id FROM users WHERE is_admin = true ORDER BY created_at ASC LIMIT 1
)
WHERE created_by_id IS NULL;

UPDATE records
SET last_modified_by_id = created_by_id
WHERE last_modified_by_id IS NULL;

ALTER TABLE records ALTER COLUMN created_by_id SET NOT NULL;
ALTER TABLE records ALTER COLUMN last_modified_by_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'records_created_by_id_fkey'
  ) THEN
    ALTER TABLE records
      ADD CONSTRAINT records_created_by_id_fkey
      FOREIGN KEY (created_by_id) REFERENCES users(id);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'records_last_modified_by_id_fkey'
  ) THEN
    ALTER TABLE records
      ADD CONSTRAINT records_last_modified_by_id_fkey
      FOREIGN KEY (last_modified_by_id) REFERENCES users(id);
  END IF;
END $$;

ALTER TABLE records ALTER COLUMN owner_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS records_object_created_by_idx ON records(object_api_name, created_by_id);
CREATE INDEX IF NOT EXISTS records_object_last_modified_by_idx ON records(object_api_name, last_modified_by_id);

-- 2) Migrate role→PS grants onto users, then drop role_permission_sets
INSERT INTO user_permission_sets (user_id, permission_set_id)
SELECT DISTINCT ur.user_id, rps.permission_set_id
FROM user_roles ur
JOIN role_permission_sets rps ON rps.role_id = ur.role_id
ON CONFLICT (user_id, permission_set_id) DO NOTHING;

DROP TABLE IF EXISTS role_permission_sets;

-- Forward-compatible stub for future system permissions on permission sets (unused in runtime).
ALTER TABLE permission_sets
  ADD COLUMN IF NOT EXISTS system_permissions jsonb NOT NULL DEFAULT '[]'::jsonb;

-- 3) principal_type: human → user
UPDATE users SET principal_type = 'user' WHERE principal_type = 'human';

ALTER TABLE users ALTER COLUMN principal_type SET DEFAULT 'user';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_principal_type_check;
ALTER TABLE users ADD CONSTRAINT users_principal_type_check
  CHECK (principal_type IN ('user', 'service', 'agent'));
