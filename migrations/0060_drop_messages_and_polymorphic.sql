-- ADR-032: retire optional messages module + polymorphic_lookup.
-- Destructive on upgraded installs that enabled Message (same class as 0012).
-- High-volume storage (records_hv) stays; only the Message RANGE leaf is dropped.

DO $$
DECLARE
  drop_objects text[] := ARRAY['Message'];
BEGIN
  DELETE FROM field_projections WHERE object_api_name = ANY (drop_objects);
  DELETE FROM field_permissions WHERE object_api_name = ANY (drop_objects);
  DELETE FROM object_permissions WHERE object_api_name = ANY (drop_objects);
  DELETE FROM metadata_validation_rules WHERE object_api_name = ANY (drop_objects);
  DELETE FROM metadata_automations WHERE object_api_name = ANY (drop_objects);
  DELETE FROM metadata_relationships
    WHERE from_object = ANY (drop_objects) OR to_object = ANY (drop_objects);
  DELETE FROM object_sharing_settings WHERE object_api_name = ANY (drop_objects);
  DELETE FROM sharing_rules WHERE object_api_name = ANY (drop_objects);
  DELETE FROM record_access_grants WHERE object_api_name = ANY (drop_objects);
  DELETE FROM metadata_fields WHERE object_api_name = ANY (drop_objects);
  DELETE FROM metadata_objects WHERE api_name = ANY (drop_objects);
  DELETE FROM records WHERE object_api_name = ANY (drop_objects);
  DELETE FROM records_hv WHERE object_api_name = ANY (drop_objects);
END $$;

-- Customer (or leftover managed) polymorphic_lookup fields: not a product type.
DELETE FROM field_permissions
 WHERE (object_api_name, field_api_name) IN (
   SELECT object_api_name, api_name FROM metadata_fields WHERE field_type = 'polymorphic_lookup'
 );
DELETE FROM field_projections
 WHERE (object_api_name, field_api_name) IN (
   SELECT object_api_name, api_name FROM metadata_fields WHERE field_type = 'polymorphic_lookup'
 );
DELETE FROM metadata_fields WHERE field_type = 'polymorphic_lookup';

DELETE FROM package_installs WHERE package_name = 'messages';
DELETE FROM high_volume_objects WHERE object_api_name = 'Message';

-- Drop Message LIST/RANGE partitions if this install created them (0019 / 0038).
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relname = 'records_hv_message'
  ) THEN
    DROP TABLE records_hv_message CASCADE;
  END IF;
END $$;
