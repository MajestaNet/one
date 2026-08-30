-- BP-017 R1: directory tags (Client SoR) + SCIM Groups adapter (non-AuthZ).
-- No FK to roles, permission_sets, or data_roles. No tenant_id.
CREATE TABLE directory_tags (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL,
  display_name text NOT NULL,
  external_id text,
  description text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_tags_api_name_key UNIQUE (api_name),
  CONSTRAINT directory_tags_display_name_key UNIQUE (display_name)
);
CREATE UNIQUE INDEX directory_tags_external_id_uidx
  ON directory_tags (external_id) WHERE external_id IS NOT NULL;

CREATE TABLE user_directory_tags (
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES directory_tags (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, tag_id)
);
CREATE INDEX user_directory_tags_tag_id_idx ON user_directory_tags (tag_id);
