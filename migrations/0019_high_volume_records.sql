-- ADR-013: high-volume flexible store (physical isolation without rewriting records).
-- Same row shape as records; LIST by object_api_name; Message RANGE by created_at.

ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS polymorphic_type_field text;

CREATE TABLE IF NOT EXISTS records_hv (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  owner_id uuid REFERENCES users(id),
  created_by_id uuid NOT NULL REFERENCES users(id),
  last_modified_by_id uuid NOT NULL REFERENCES users(id),
  data jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  PRIMARY KEY (id, object_api_name, created_at)
) PARTITION BY LIST (object_api_name);

CREATE INDEX IF NOT EXISTS records_hv_object_owner_idx ON records_hv (object_api_name, owner_id);
CREATE INDEX IF NOT EXISTS records_hv_object_created_id_idx ON records_hv (object_api_name, created_at, id);
CREATE INDEX IF NOT EXISTS records_hv_object_created_by_idx ON records_hv (object_api_name, created_by_id);
CREATE INDEX IF NOT EXISTS records_hv_object_last_modified_by_idx ON records_hv (object_api_name, last_modified_by_id);
CREATE INDEX IF NOT EXISTS records_hv_created_brin ON records_hv USING brin (created_at);

-- Catalog of high-volume object partitions (product/worker DDL, not customer field DDL).
CREATE TABLE IF NOT EXISTS high_volume_objects (
  object_api_name text PRIMARY KEY,
  partition_name text NOT NULL UNIQUE,
  range_partitioned boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Message: LIST leaf further RANGE-partitioned by created_at (append-heavy).
CREATE TABLE IF NOT EXISTS records_hv_message
  PARTITION OF records_hv FOR VALUES IN ('Message')
  PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS records_hv_message_pre_2024
  PARTITION OF records_hv_message FOR VALUES FROM (MINVALUE) TO ('2024-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2024
  PARTITION OF records_hv_message FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2025
  PARTITION OF records_hv_message FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2026
  PARTITION OF records_hv_message FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2027
  PARTITION OF records_hv_message FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2028
  PARTITION OF records_hv_message FOR VALUES FROM ('2028-01-01') TO ('2029-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2029
  PARTITION OF records_hv_message FOR VALUES FROM ('2029-01-01') TO ('2030-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2030
  PARTITION OF records_hv_message FOR VALUES FROM ('2030-01-01') TO ('2031-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_future
  PARTITION OF records_hv_message DEFAULT;

INSERT INTO high_volume_objects (object_api_name, partition_name, range_partitioned)
VALUES ('Message', 'records_hv_message', true)
ON CONFLICT (object_api_name) DO NOTHING;
