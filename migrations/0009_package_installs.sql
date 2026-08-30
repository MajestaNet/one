-- Managed package install history (core additive migrate on product upgrade)
CREATE TABLE IF NOT EXISTS package_installs (
  package_name text PRIMARY KEY,
  version text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);
