-- Remap legacy managed package names (crm) onto core for Account/Contact.
-- Legacy Lead/Opportunity/ERP objects are removed in 0012_drop_legacy_managed_objects.

UPDATE metadata_objects
SET package_name = 'core'
WHERE package_name = 'crm'
  AND api_name IN ('Account', 'Contact');

UPDATE metadata_fields
SET package_name = 'core'
WHERE package_name = 'crm'
  AND object_api_name IN ('Account', 'Contact');

UPDATE metadata_validation_rules
SET package_name = 'core'
WHERE package_name = 'crm'
  AND object_api_name IN ('Account', 'Contact');

-- Record core package install from prior crm install when present; otherwise leave for seed.
INSERT INTO package_installs (package_name, version)
SELECT 'core', COALESCE(
  (SELECT version FROM package_installs WHERE package_name = 'crm'),
  '1.0.0'
)
ON CONFLICT (package_name) DO NOTHING;

-- Drop stale install rows for packages the product no longer ships.
DELETE FROM package_installs WHERE package_name IN ('crm', 'erp');
