-- ADR-016 / BP-003: record sharing (OWD, data roles, criteria rules, materialized grants).

CREATE TABLE IF NOT EXISTS organization_settings (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  record_sharing_enabled boolean NOT NULL DEFAULT false,
  record_sharing_enabled_at timestamptz,
  CONSTRAINT org_settings_singleton CHECK (id)
);
--> statement-breakpoint
INSERT INTO organization_settings (id, record_sharing_enabled)
VALUES (true, false)
ON CONFLICT (id) DO NOTHING;
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS data_roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  parent_data_role_id uuid REFERENCES data_roles(id),
  is_system boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS data_role_id uuid REFERENCES data_roles(id);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS object_sharing_settings (
  object_api_name text PRIMARY KEY REFERENCES metadata_objects(api_name) ON DELETE CASCADE,
  default_access text NOT NULL DEFAULT 'private'
    CHECK (default_access IN ('private', 'public_read', 'public_read_write')),
  sharing_rules_enabled boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
INSERT INTO object_sharing_settings (object_api_name, default_access, sharing_rules_enabled)
SELECT api_name, 'private', false FROM metadata_objects
ON CONFLICT (object_api_name) DO NOTHING;
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS sharing_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL REFERENCES metadata_objects(api_name) ON DELETE CASCADE,
  api_name text NOT NULL,
  label text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  access_level text NOT NULL DEFAULT 'read'
    CHECK (access_level IN ('read', 'read_write')),
  shared_to_data_role_id uuid NOT NULL REFERENCES data_roles(id),
  criteria jsonb NOT NULL DEFAULT '{"filters":[]}'::jsonb,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (object_api_name, api_name)
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS record_access_grants (
  record_id uuid NOT NULL,
  object_api_name text NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  access_level text NOT NULL CHECK (access_level IN ('read', 'read_write')),
  row_cause text NOT NULL CHECK (row_cause IN ('rule', 'manual')),
  source_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
  PRIMARY KEY (record_id, user_id, row_cause, source_id)
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS record_access_grants_user_object_idx
  ON record_access_grants (user_id, object_api_name, record_id);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS record_access_grants_record_idx
  ON record_access_grants (record_id);
