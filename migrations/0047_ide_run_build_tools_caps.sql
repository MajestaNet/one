-- ADR-021 / BP-050 Phase 5: union ide.build.tools + ide.run* into system permission sets.
-- Additive only — recreates one_jsonb_text_union (0045 creates and drops it inline).

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

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.run","ide.run.tools","ide.build.tools"]'::jsonb
)
WHERE api_name = 'Admin';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.run","ide.run.tools"]'::jsonb
),
description = 'Operate data access plus Control IDE Operate and Run chrome'
WHERE api_name = 'Operate';

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.build.tools"]'::jsonb
)
WHERE api_name IN ('Build', 'MetadataCustomize');

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.build.tools"]'::jsonb
)
WHERE system_permissions ?| ARRAY['metadata.build', 'metadata.customize', 'metadata.packages'];

UPDATE permission_sets
SET system_permissions = one_jsonb_text_union(
  system_permissions,
  '["ide.run","ide.run.tools"]'::jsonb
)
WHERE system_permissions ?| ARRAY['client.read', 'client.write', 'client'];

DROP FUNCTION IF EXISTS one_jsonb_text_union(jsonb, jsonb);
