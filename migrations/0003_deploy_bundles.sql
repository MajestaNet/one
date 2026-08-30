-- Phase C: deploy bundles + promotions (same-install validate/apply)
CREATE TABLE IF NOT EXISTS deploy_bundles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  label text,
  source_install_id text NOT NULL,
  product_version text NOT NULL,
  product_version_range text NOT NULL,
  checksum text NOT NULL,
  artifact jsonb NOT NULL,
  status text NOT NULL DEFAULT 'ready',
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_bundles_created_idx ON deploy_bundles(created_at);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS deploy_promotions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  bundle_id uuid NOT NULL REFERENCES deploy_bundles(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'pending',
  dry_run boolean NOT NULL DEFAULT false,
  validation_report jsonb,
  apply_report jsonb,
  error text,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_promotions_bundle_idx ON deploy_promotions(bundle_id);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS deploy_promotions_created_idx ON deploy_promotions(created_at);
