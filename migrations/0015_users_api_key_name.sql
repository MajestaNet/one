-- Bind each env API_KEYS entry to a distinct service principal (BP-013 / BP-006 Phase 2).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS api_key_name text;

CREATE UNIQUE INDEX IF NOT EXISTS users_api_key_name_uidx
  ON users (api_key_name)
  WHERE api_key_name IS NOT NULL;
