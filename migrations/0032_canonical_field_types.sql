-- Canonical field types (BP-036): remap legacy aliases, autonumber sequences, field_options.

UPDATE metadata_fields SET field_type = 'text' WHERE field_type IN ('string');
UPDATE metadata_fields SET field_type = 'number' WHERE field_type IN ('double', 'float');
UPDATE metadata_fields SET field_type = 'lookup' WHERE field_type IN ('reference');
UPDATE metadata_fields SET field_type = 'integer' WHERE field_type IN ('int', 'long');
UPDATE metadata_fields SET field_type = 'boolean' WHERE field_type IN ('checkbox');

ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS field_options jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS autonumber_sequences (
  object_api_name text NOT NULL,
  field_api_name text NOT NULL,
  next_value bigint NOT NULL DEFAULT 1,
  PRIMARY KEY (object_api_name, field_api_name)
);

COMMENT ON TABLE autonumber_sequences IS 'Per-field auto-number counters for field_type=autonumber';
COMMENT ON COLUMN metadata_fields.field_options IS 'Type-specific options (autonumberFormat, autonumberStart, …)';
