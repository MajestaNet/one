-- BP-030: install-local DigitalOcean App Platform binding + provision audit
-- One database = one install (ADR-001); no customer_id column.

CREATE TABLE IF NOT EXISTS deploy_digitalocean_cloud (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  app_id text,
  database_id text,
  region text,
  app_name text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS deploy_digitalocean_provision_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  peer_install_id text NOT NULL,
  install_role text,
  app_id text,
  database_id text,
  base_url text,
  status text NOT NULL DEFAULT 'pending',
  error text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_do_provision_runs_created_idx
  ON deploy_digitalocean_provision_runs (created_at DESC);
