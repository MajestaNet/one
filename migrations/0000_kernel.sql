CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  display_name text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  is_admin boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  parent_role_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS permission_sets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  description text,
  is_system boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_permission_sets (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS user_permission_sets_uniq ON user_permission_sets(user_id, permission_set_id);

CREATE TABLE IF NOT EXISTS object_permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
  object_api_name text NOT NULL,
  can_create boolean NOT NULL DEFAULT false,
  can_read boolean NOT NULL DEFAULT false,
  can_update boolean NOT NULL DEFAULT false,
  can_delete boolean NOT NULL DEFAULT false,
  view_all boolean NOT NULL DEFAULT false,
  modify_all boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS object_permissions_uniq ON object_permissions(permission_set_id, object_api_name);

CREATE TABLE IF NOT EXISTS field_permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  permission_set_id uuid NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
  object_api_name text NOT NULL,
  field_api_name text NOT NULL,
  can_read boolean NOT NULL DEFAULT true,
  can_edit boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS field_permissions_uniq ON field_permissions(permission_set_id, object_api_name, field_api_name);

CREATE TABLE IF NOT EXISTS metadata_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL,
  label text NOT NULL,
  plural_label text NOT NULL,
  storage_mode text NOT NULL DEFAULT 'flexible',
  package_name text,
  features jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS metadata_objects_api_name_uniq ON metadata_objects(api_name);

CREATE TABLE IF NOT EXISTS metadata_fields (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  api_name text NOT NULL,
  label text NOT NULL,
  field_type text NOT NULL,
  required boolean NOT NULL DEFAULT false,
  unique_field boolean NOT NULL DEFAULT false,
  default_value jsonb,
  length integer,
  precision integer,
  scale integer,
  picklist_values jsonb,
  reference_to text,
  relationship_name text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS metadata_fields_uniq ON metadata_fields(object_api_name, api_name);
CREATE INDEX IF NOT EXISTS metadata_fields_object_idx ON metadata_fields(object_api_name);

CREATE TABLE IF NOT EXISTS metadata_relationships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  from_object text NOT NULL,
  to_object text NOT NULL,
  relationship_type text NOT NULL,
  field_api_name text NOT NULL,
  cascade_delete boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS metadata_validation_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  api_name text NOT NULL,
  label text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  error_message text NOT NULL,
  expression jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS metadata_automations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  object_api_name text NOT NULL,
  trigger_event text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  condition jsonb,
  actions jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS records (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  owner_id uuid NOT NULL REFERENCES users(id),
  data jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS records_object_idx ON records(object_api_name);
CREATE INDEX IF NOT EXISTS records_owner_idx ON records(owner_id);
CREATE INDEX IF NOT EXISTS records_data_gin_idx ON records USING gin (data);

CREATE TABLE IF NOT EXISTS outbox_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type text NOT NULL,
  object_api_name text,
  record_id uuid,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  last_error text
);
CREATE INDEX IF NOT EXISTS outbox_unpublished_idx ON outbox_events(published_at, created_at);

CREATE TABLE IF NOT EXISTS jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'pending',
  run_at timestamptz NOT NULL DEFAULT now(),
  attempts integer NOT NULL DEFAULT 0,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS jobs_status_run_idx ON jobs(status, run_at);

CREATE TABLE IF NOT EXISTS audit_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid,
  action text NOT NULL,
  object_api_name text,
  record_id uuid,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_log_created_idx ON audit_log(created_at);

CREATE TABLE IF NOT EXISTS webhooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  url text NOT NULL,
  secret text,
  event_types jsonb NOT NULL DEFAULT '["*"]'::jsonb,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agent_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  playbook_api_name text,
  status text NOT NULL DEFAULT 'queued',
  goal text NOT NULL,
  input jsonb NOT NULL DEFAULT '{}'::jsonb,
  output jsonb,
  actor_id uuid,
  dry_run boolean NOT NULL DEFAULT false,
  error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS agent_playbooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL UNIQUE,
  label text NOT NULL,
  goal_template text NOT NULL,
  allowed_tools jsonb NOT NULL DEFAULT '[]'::jsonb,
  object_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
  require_approval boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS feature_flags (
  key text PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT true,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);
