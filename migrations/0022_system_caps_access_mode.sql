-- System capability catalog uplift + device cert enrollments + integration CIDR overlays.
-- Canonical caps: identity.users, identity.integrations, authz.manage, metadata.build,
-- deploy.promote, govern.network, govern.agents, govern.audit.

-- Expand / rewrite system_permissions JSON arrays toward canonical names.
UPDATE permission_sets ps
SET system_permissions = sub.normalized
FROM (
  SELECT
    id,
    COALESCE(
      (
        SELECT jsonb_agg(DISTINCT cap ORDER BY cap)
        FROM (
          SELECT CASE elem
            WHEN 'metadata.customize' THEN 'metadata.build'
            WHEN 'metadata.packages' THEN 'metadata.build'
            WHEN 'metadata.assignAuthz' THEN 'authz.manage'
            WHEN 'metadata.network' THEN 'govern.network'
            WHEN 'agents.approve' THEN 'govern.agents'
            WHEN 'identity.manage' THEN 'identity.users'
            ELSE elem
          END AS cap
          FROM jsonb_array_elements_text(COALESCE(ps2.system_permissions, '[]'::jsonb)) AS elem
          UNION ALL
          SELECT 'identity.integrations'
          FROM jsonb_array_elements_text(COALESCE(ps2.system_permissions, '[]'::jsonb)) AS elem
          WHERE elem = 'identity.manage'
        ) expanded
        WHERE cap IS NOT NULL AND cap <> ''
      ),
      '[]'::jsonb
    ) AS normalized
  FROM permission_sets ps2
) sub
WHERE ps.id = sub.id;

-- Ensure Admin has the full canonical catalog.
UPDATE permission_sets
SET system_permissions = '["identity.users","identity.integrations","authz.manage","metadata.build","deploy.promote","govern.network","govern.agents","govern.audit"]'::jsonb
WHERE api_name = 'Admin' AND is_system = true;

-- Seed new system permission packs (idempotent).
INSERT INTO permission_sets (api_name, label, description, is_system, system_permissions)
VALUES
  ('ManageUsers', 'Manage Users', 'Manage user principals and user credentials', true, '["identity.users"]'::jsonb),
  ('ManageIntegrations', 'Manage Integrations', 'Manage Connected Apps and service/agent credentials', true, '["identity.integrations"]'::jsonb),
  ('ManagePermissions', 'Manage Permissions', 'Define permission sets and assign Roles/PS', true, '["authz.manage"]'::jsonb),
  ('Build', 'Build', 'Customize customer metadata and manage packages', true, '["metadata.build"]'::jsonb),
  ('Deploy', 'Deploy', 'Create and promote Deploy bundles', true, '["deploy.promote"]'::jsonb),
  ('Govern', 'Govern', 'Install exposure/WAF and agent approvals', true, '["govern.network","govern.agents"]'::jsonb),
  ('Operate', 'Operate', 'Operate data access (object/field grants; no system caps)', true, '[]'::jsonb)
ON CONFLICT (api_name) DO UPDATE SET
  description = EXCLUDED.description,
  system_permissions = EXCLUDED.system_permissions,
  is_system = true;

-- Align legacy pack names to canonical caps when present.
UPDATE permission_sets SET system_permissions = '["metadata.build"]'::jsonb WHERE api_name = 'MetadataCustomize' AND is_system = true;
UPDATE permission_sets SET system_permissions = '["deploy.promote"]'::jsonb WHERE api_name = 'DeployPromote' AND is_system = true;
UPDATE permission_sets SET system_permissions = '["govern.agents"]'::jsonb WHERE api_name = 'AgentsApprove' AND is_system = true;
UPDATE permission_sets SET system_permissions = '["identity.users","identity.integrations"]'::jsonb WHERE api_name = 'IdentityManage' AND is_system = true;

-- Device certificate enrollments (Phase E foundation for requireDeviceCert).
CREATE TABLE IF NOT EXISTS device_certificates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id text NOT NULL,
  label text NOT NULL DEFAULT '',
  fingerprint text NOT NULL,
  certificate_pem text NOT NULL,
  revoked_at timestamptz,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT device_certificates_device_unique UNIQUE (user_id, device_id)
);

CREATE INDEX IF NOT EXISTS device_certificates_user_active_idx
  ON device_certificates (user_id)
  WHERE revoked_at IS NULL;

-- Optional Connected App CIDR overlays (merged into exposure allowlists on apply).
ALTER TABLE integration_configs
  ADD COLUMN IF NOT EXISTS allowed_cidrs jsonb NOT NULL DEFAULT '[]'::jsonb;
