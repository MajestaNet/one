-- BP-004: shared metadata cache epoch for multi-replica API coherence
CREATE TABLE IF NOT EXISTS metadata_cache_epoch (
  id integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  epoch bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
INSERT INTO metadata_cache_epoch (id, epoch) VALUES (1, 1) ON CONFLICT (id) DO NOTHING;
--> statement-breakpoint
-- BP-005: lease columns for FOR UPDATE SKIP LOCKED claim loops
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS locked_at timestamptz;
--> statement-breakpoint
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS locked_by text;
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS outbox_claim_idx ON outbox_events(published_at, locked_at, created_at);
--> statement-breakpoint
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS locked_at timestamptz;
--> statement-breakpoint
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS locked_by text;
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS jobs_claim_idx ON jobs(status, run_at, locked_at);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE,
  webhook_id uuid NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  delivered_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS webhook_deliveries_uniq ON webhook_deliveries(event_id, webhook_id);
