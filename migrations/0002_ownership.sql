-- Phase B: metadata ownership (managed vs customer) + package_name on related artifacts
ALTER TABLE metadata_objects ADD COLUMN IF NOT EXISTS ownership text NOT NULL DEFAULT 'custom';
--> statement-breakpoint
ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS package_name text;
--> statement-breakpoint
ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS ownership text NOT NULL DEFAULT 'custom';
--> statement-breakpoint
ALTER TABLE metadata_validation_rules ADD COLUMN IF NOT EXISTS package_name text;
--> statement-breakpoint
ALTER TABLE metadata_validation_rules ADD COLUMN IF NOT EXISTS ownership text NOT NULL DEFAULT 'custom';
--> statement-breakpoint
ALTER TABLE metadata_automations ADD COLUMN IF NOT EXISTS package_name text;
--> statement-breakpoint
ALTER TABLE metadata_automations ADD COLUMN IF NOT EXISTS ownership text NOT NULL DEFAULT 'custom';
--> statement-breakpoint
UPDATE metadata_objects
SET ownership = 'managed'
WHERE package_name IN ('platform', 'crm', 'erp');
--> statement-breakpoint
UPDATE metadata_fields
SET ownership = 'managed'
WHERE package_name IN ('platform', 'crm', 'erp')
  AND ownership IS DISTINCT FROM 'managed';
--> statement-breakpoint
UPDATE metadata_fields AS f
SET ownership = 'managed',
    package_name = o.package_name
FROM metadata_objects AS o
WHERE f.object_api_name = o.api_name
  AND o.package_name IN ('platform', 'crm', 'erp')
  AND f.package_name IS NULL;
