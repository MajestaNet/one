-- Drop legacy managed objects formerly seeded under crm/erp packages.
-- Core keeps only Account + Contact (User is kernel). Destructive on upgraded installs:
-- removes metadata, permissions, projections, relationships, and business records for these api_names.

DO $$
DECLARE
  legacy text[] := ARRAY[
    'Lead', 'Opportunity', 'Activity',
    'Product', 'PriceBook', 'Order', 'Invoice', 'Payment'
  ];
BEGIN
  DELETE FROM field_projections WHERE object_api_name = ANY (legacy);
  DELETE FROM field_permissions WHERE object_api_name = ANY (legacy);
  DELETE FROM object_permissions WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_validation_rules WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_automations WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_relationships
  WHERE from_object = ANY (legacy) OR to_object = ANY (legacy);
  DELETE FROM metadata_fields WHERE object_api_name = ANY (legacy);
  DELETE FROM metadata_objects WHERE api_name = ANY (legacy);
  DELETE FROM records WHERE object_api_name = ANY (legacy);
END $$;

-- Any leftover package labels from the old split (Account/Contact already remapped in 0011).
UPDATE metadata_objects SET package_name = 'core'
WHERE package_name IN ('crm', 'erp') AND api_name IN ('Account', 'Contact');
UPDATE metadata_fields SET package_name = 'core'
WHERE package_name IN ('crm', 'erp') AND object_api_name IN ('Account', 'Contact');

DELETE FROM package_installs WHERE package_name IN ('crm', 'erp');
