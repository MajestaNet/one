-- Phase E: multi-env customer identity + peer registry
ALTER TABLE deploy_bundles ADD COLUMN IF NOT EXISTS customer_id text NOT NULL DEFAULT 'local-customer';
--> statement-breakpoint
ALTER TABLE deploy_bundles ADD COLUMN IF NOT EXISTS source_install_role text;
--> statement-breakpoint
ALTER TABLE deploy_bundles ADD COLUMN IF NOT EXISTS origin text NOT NULL DEFAULT 'local';
--> statement-breakpoint
ALTER TABLE deploy_bundles ADD COLUMN IF NOT EXISTS signature text;
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_bundles_customer_idx ON deploy_bundles(customer_id);
--> statement-breakpoint
ALTER TABLE deploy_promotions ADD COLUMN IF NOT EXISTS direction text NOT NULL DEFAULT 'local';
--> statement-breakpoint
ALTER TABLE deploy_promotions ADD COLUMN IF NOT EXISTS source_install_id text;
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS deploy_peer_installs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  install_id text NOT NULL,
  customer_id text NOT NULL,
  label text,
  install_role text,
  base_url text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS deploy_peer_installs_uniq ON deploy_peer_installs(customer_id, install_id);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_peer_installs_customer_idx ON deploy_peer_installs(customer_id);
