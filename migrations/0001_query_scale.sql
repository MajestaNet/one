ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS indexed boolean NOT NULL DEFAULT false;
ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS filterable boolean NOT NULL DEFAULT true;
ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS sortable boolean NOT NULL DEFAULT true;
CREATE INDEX IF NOT EXISTS metadata_fields_indexed_idx ON metadata_fields(object_api_name, indexed);
CREATE INDEX IF NOT EXISTS records_object_owner_idx ON records(object_api_name, owner_id);
CREATE INDEX IF NOT EXISTS records_object_created_id_idx ON records(object_api_name, created_at, id);
CREATE TABLE IF NOT EXISTS field_projections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  field_api_name text NOT NULL,
  index_name text NOT NULL UNIQUE,
  cast_type text NOT NULL DEFAULT 'text',
  status text NOT NULL DEFAULT 'pending',
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  built_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS field_projections_uniq ON field_projections(object_api_name, field_api_name);
CREATE INDEX IF NOT EXISTS field_projections_object_idx ON field_projections(object_api_name);
