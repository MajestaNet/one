-- BP-041: async Bulk ingest jobs (Client /jobs/ingest).
CREATE TABLE IF NOT EXISTS ingest_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid NOT NULL REFERENCES users(id),
  object_api_name text NOT NULL,
  operation text NOT NULL,
  external_id_field text,
  content_type text NOT NULL DEFAULT 'application/x-ndjson',
  state text NOT NULL DEFAULT 'Open',
  upload_bytes bigint NOT NULL DEFAULT 0,
  row_count int NOT NULL DEFAULT 0,
  success_count int NOT NULL DEFAULT 0,
  failure_count int NOT NULL DEFAULT 0,
  all_or_none boolean NOT NULL DEFAULT false,
  error_message text,
  payload bytea NOT NULL DEFAULT ''::bytea,
  result_success bytea NOT NULL DEFAULT ''::bytea,
  result_failed bytea NOT NULL DEFAULT ''::bytea,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

CREATE INDEX IF NOT EXISTS ingest_jobs_actor_idx ON ingest_jobs (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ingest_jobs_state_idx ON ingest_jobs (state, created_at);
