-- IDE chrome capabilities (ide.*) + debug.read/trace on system packs.
-- Additive: unions new caps into existing system_permissions without dropping custom caps.
-- Also freezes field_permissions for deny-by-default FLS (see authz-ide-fls-build-plan.md).

-- Helper: union JSONB string arrays.
CREATE OR REPLACE FUNCTION one_jsonb_text_union(a jsonb, b jsonb)
RETURNS jsonb
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT COALESCE(
    (SELECT jsonb_agg(DISTINCT x ORDER BY x)
     FROM (
       SELECT jsonb_array_elements_text(COALESCE(a, '[]'::jsonb)) AS x
       UNION
       SELECT jsonb_array_elements_text(COALESCE(b, '[]'::jsonb)) AS x
     ) u
     WHERE x IS NOT NULL AND x <> ''),
    '[]'::jsonb
  );
$$;

-- Admin: full canonical catalog including ide.* and debug.*.
UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '[
    "identity.users","identity.integrations","authz.manage","metadata.build","deploy.promote",
    "govern.network","govern.agents","govern.audit","debug.read","debug.trace",
    "ide.operate","ide.build","ide.ship","ide.govern",
    "ide.operate.query","ide.operate.monitor","ide.operate.explorer","ide.operate.canvases",
    "ide.build.objects","ide.build.packages","ide.build.agentSpecs","ide.build.canvasSpecs","ide.build.repo",
    "ide.ship.deploy","ide.ship.env",
    "ide.govern.users","ide.govern.integrations","ide.govern.experiences",
    "ide.govern.installAuth","ide.govern.permissions","ide.govern.env"
  ]'::jsonb
)
WHERE api_name = 'Admin';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.operate","ide.operate.query","ide.operate.monitor","ide.operate.explorer","ide.operate.canvases"]'::jsonb
),
description = 'Operate data access plus Control IDE Operate chrome'
WHERE api_name = 'Operate';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["metadata.build","ide.build","ide.build.objects","ide.build.packages","ide.build.agentSpecs","ide.build.canvasSpecs","ide.build.repo"]'::jsonb
)
WHERE api_name IN ('Build', 'MetadataCustomize');

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["deploy.promote","ide.ship","ide.ship.deploy","ide.ship.env"]'::jsonb
)
WHERE api_name IN ('Deploy', 'DeployPromote');

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["govern.network","govern.agents","ide.govern","ide.govern.users","ide.govern.integrations","ide.govern.experiences","ide.govern.installAuth","ide.govern.permissions","ide.govern.env"]'::jsonb
)
WHERE api_name = 'Govern';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["identity.users","ide.govern","ide.govern.users","ide.govern.env"]'::jsonb
)
WHERE api_name = 'ManageUsers';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["identity.integrations","ide.govern","ide.govern.integrations","ide.govern.env"]'::jsonb
)
WHERE api_name = 'ManageIntegrations';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["authz.manage","ide.govern","ide.govern.permissions","ide.govern.env"]'::jsonb
)
WHERE api_name = 'ManagePermissions';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["identity.users","identity.integrations","ide.govern","ide.govern.users","ide.govern.integrations","ide.govern.env"]'::jsonb
)
WHERE api_name = 'IdentityManage';

-- Customer / custom packs: if they already hold an API capability, grant matching IDE chrome
-- so fail-closed IDE gating does not lock existing admins out of Build/Ship/Govern.
UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.build","ide.build.objects","ide.build.packages","ide.build.agentSpecs","ide.build.canvasSpecs","ide.build.repo"]'::jsonb
)
WHERE system_permissions ?| ARRAY['metadata.build', 'metadata.customize', 'metadata.packages'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.ship","ide.ship.deploy","ide.ship.env"]'::jsonb
)
WHERE system_permissions ?| ARRAY['deploy.promote'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.govern","ide.govern.users","ide.govern.env"]'::jsonb
)
WHERE system_permissions ?| ARRAY['identity.users', 'identity.manage'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.govern","ide.govern.integrations","ide.govern.env"]'::jsonb
)
WHERE system_permissions ?| ARRAY['identity.integrations', 'identity.manage'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.govern","ide.govern.permissions","ide.govern.env"]'::jsonb
)
WHERE system_permissions ?| ARRAY['authz.manage', 'metadata.assignAuthz'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.govern","ide.govern.env","ide.govern.installAuth"]'::jsonb
)
WHERE system_permissions ?| ARRAY['govern.network', 'metadata.network', 'govern.agents', 'agents.approve', 'govern.audit'];

-- FLS freeze: materialize allow-if-absent as explicit grants so flipping to deny-by-default
-- does not shrink effective access for existing permission sets.
INSERT INTO field_permissions (permission_set_id, object_api_name, field_api_name, can_read, can_edit)
SELECT ps.id, f.object_api_name, f.api_name, true, true
FROM permission_sets ps
CROSS JOIN metadata_fields f
ON CONFLICT (permission_set_id, object_api_name, field_api_name) DO NOTHING;

DROP FUNCTION IF EXISTS one_jsonb_text_union(jsonb, jsonb);
