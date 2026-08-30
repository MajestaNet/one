-- Pre-customer breaking cleanup (ADR-013 / BP-035 follow-through):
-- 1. Hard-delete: drop deleted_at (no soft-delete tombstones)
-- 2. Drop LIST DEFAULT traps (records_default, records_hv_message_future)
-- 3. Rebuild record_access_grants PK around (object_api_name, record_id, ...)
-- 4. Align HV btree indexes; drop redundant jobs/outbox indexes

-- ---------------------------------------------------------------------------
-- Hard-delete: purge soft-deleted rows, drop column from both stores
-- ---------------------------------------------------------------------------
DELETE FROM records WHERE deleted_at IS NOT NULL;
DELETE FROM records_hv WHERE deleted_at IS NOT NULL;

DROP INDEX IF EXISTS records_object_owner_live_idx;
DROP INDEX IF EXISTS records_object_created_id_live_idx;
DROP INDEX IF EXISTS records_object_created_by_live_idx;
DROP INDEX IF EXISTS records_object_last_modified_by_live_idx;

ALTER TABLE records DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE records_hv DROP COLUMN IF EXISTS deleted_at;

CREATE INDEX IF NOT EXISTS records_object_owner_idx
  ON records (object_api_name, owner_id);
CREATE INDEX IF NOT EXISTS records_object_created_id_idx
  ON records (object_api_name, created_at, id);
CREATE INDEX IF NOT EXISTS records_object_created_by_idx
  ON records (object_api_name, created_by_id);
CREATE INDEX IF NOT EXISTS records_object_last_modified_by_idx
  ON records (object_api_name, last_modified_by_id);

DROP INDEX IF EXISTS records_hv_object_owner_idx;
DROP INDEX IF EXISTS records_hv_object_created_id_idx;
DROP INDEX IF EXISTS records_hv_object_created_by_idx;
DROP INDEX IF EXISTS records_hv_object_last_modified_by_idx;

CREATE INDEX IF NOT EXISTS records_hv_object_owner_idx
  ON records_hv (object_api_name, owner_id);
CREATE INDEX IF NOT EXISTS records_hv_object_created_id_idx
  ON records_hv (object_api_name, created_at, id);
CREATE INDEX IF NOT EXISTS records_hv_object_created_by_idx
  ON records_hv (object_api_name, created_by_id);
CREATE INDEX IF NOT EXISTS records_hv_object_last_modified_by_idx
  ON records_hv (object_api_name, last_modified_by_id);

-- Expression indexes that predicated deleted_at must be rebuilt by worker.
UPDATE field_projections SET status = 'pending', built_at = NULL, last_error = NULL;

-- ---------------------------------------------------------------------------
-- Drop records DEFAULT partition (fail-closed: dedicated leaf required)
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  r RECORD;
  part text;
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class c
    JOIN pg_inherits i ON i.inhrelid = c.oid
    JOIN pg_class p ON p.oid = i.inhparent
    WHERE p.relname = 'records' AND c.relname = 'records_default'
  ) THEN
    FOR r IN SELECT DISTINCT object_api_name AS n FROM only records_default
    LOOP
      IF NOT EXISTS (SELECT 1 FROM flexible_objects WHERE object_api_name = r.n) THEN
        part := lower(regexp_replace('records_o_' || r.n, '[^a-z0-9_]+', '_', 'g'));
        IF length(part) > 60 THEN
          part := substr(part, 1, 60);
        END IF;
        EXECUTE format(
          'CREATE TABLE IF NOT EXISTS %I (LIKE records INCLUDING DEFAULTS INCLUDING CONSTRAINTS)',
          part
        );
        EXECUTE format(
          'WITH moved AS (
             DELETE FROM ONLY records_default WHERE object_api_name = %L RETURNING *
           )
           INSERT INTO %I SELECT * FROM moved',
          r.n, part
        );
        EXECUTE format(
          'ALTER TABLE records ATTACH PARTITION %I FOR VALUES IN (%L)',
          part, r.n
        );
        INSERT INTO flexible_objects (object_api_name, partition_name)
        VALUES (r.n, part)
        ON CONFLICT (object_api_name) DO NOTHING;
      END IF;
    END LOOP;
    ALTER TABLE records DETACH PARTITION records_default;
    DROP TABLE records_default;
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Drop Message RANGE DEFAULT (writes must land in named year partitions)
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class c
    JOIN pg_inherits i ON i.inhrelid = c.oid
    JOIN pg_class p ON p.oid = i.inhparent
    WHERE p.relname = 'records_hv_message' AND c.relname = 'records_hv_message_future'
  ) THEN
    -- Reject if DEFAULT already holds rows (would lose data / need manual move).
    IF EXISTS (SELECT 1 FROM only records_hv_message_future LIMIT 1) THEN
      RAISE EXCEPTION 'records_hv_message_future is not empty; move rows to yearly partitions before dropping DEFAULT';
    END IF;
    ALTER TABLE records_hv_message DETACH PARTITION records_hv_message_future;
    DROP TABLE records_hv_message_future;
  END IF;
END $$;

-- Ensure near-term yearly Message partitions exist (through 2033).
CREATE TABLE IF NOT EXISTS records_hv_message_2031
  PARTITION OF records_hv_message FOR VALUES FROM ('2031-01-01') TO ('2032-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2032
  PARTITION OF records_hv_message FOR VALUES FROM ('2032-01-01') TO ('2033-01-01');
CREATE TABLE IF NOT EXISTS records_hv_message_2033
  PARTITION OF records_hv_message FOR VALUES FROM ('2033-01-01') TO ('2034-01-01');

-- ---------------------------------------------------------------------------
-- record_access_grants: composite identity includes object_api_name
-- ---------------------------------------------------------------------------
CREATE TABLE record_access_grants_new (
  object_api_name text NOT NULL,
  record_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  access_level text NOT NULL CHECK (access_level IN ('read', 'read_write')),
  row_cause text NOT NULL CHECK (row_cause IN ('rule', 'manual')),
  source_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
  PRIMARY KEY (object_api_name, record_id, user_id, row_cause, source_id)
);

INSERT INTO record_access_grants_new (object_api_name, record_id, user_id, access_level, row_cause, source_id)
SELECT object_api_name, record_id, user_id, access_level, row_cause, source_id
FROM record_access_grants;

DROP TABLE record_access_grants;
ALTER TABLE record_access_grants_new RENAME TO record_access_grants;

CREATE INDEX IF NOT EXISTS record_access_grants_user_object_idx
  ON record_access_grants (user_id, object_api_name, record_id);
CREATE INDEX IF NOT EXISTS record_access_grants_object_record_idx
  ON record_access_grants (object_api_name, record_id);

-- ---------------------------------------------------------------------------
-- Drop superseded claim indexes (0000 + 0006 overlap)
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS outbox_unpublished_idx;
DROP INDEX IF EXISTS jobs_status_run_idx;
