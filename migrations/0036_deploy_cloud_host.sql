-- Agnostic Deploy cloud binding + provision audit (host-keyed).
-- Backfills from deploy_digitalocean_* (BP-030) with host='digitalocean'.
-- One database = one install (ADR-001); no customer_id column.

CREATE TABLE IF NOT EXISTS deploy_cloud_binding (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  host text NOT NULL DEFAULT '',
  app_resource_id text,
  database_resource_id text,
  region text,
  display_name text,
  provider_meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS deploy_cloud_provision_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host text NOT NULL DEFAULT '',
  peer_install_id text NOT NULL,
  install_role text,
  app_resource_id text,
  database_resource_id text,
  base_url text,
  status text NOT NULL DEFAULT 'pending',
  error text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_cloud_provision_runs_created_idx
  ON deploy_cloud_provision_runs (created_at DESC);
--> statement-breakpoint
INSERT INTO deploy_cloud_binding (
  singleton, host, app_resource_id, database_resource_id, region, display_name, updated_at, created_at
)
SELECT
  true,
  'digitalocean',
  app_id,
  database_id,
  region,
  app_name,
  updated_at,
  created_at
FROM deploy_digitalocean_cloud
WHERE singleton = true
  AND (app_id IS NOT NULL OR database_id IS NOT NULL)
ON CONFLICT (singleton) DO NOTHING;
--> statement-breakpoint
INSERT INTO deploy_cloud_provision_runs (
  id, host, peer_install_id, install_role, app_resource_id, database_resource_id,
  base_url, status, error, created_by, created_at
)
SELECT
  id,
  'digitalocean',
  peer_install_id,
  install_role,
  app_id,
  database_id,
  base_url,
  status,
  error,
  created_by,
  created_at
FROM deploy_digitalocean_provision_runs
ON CONFLICT (id) DO NOTHING;
