-- ADR-014 Phase 5: persist packed guest sources (src/ + tests/automations) on the install.
CREATE TABLE IF NOT EXISTS customer_source_files (
  path text PRIMARY KEY,
  body text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS customer_source_files_prefix_idx ON customer_source_files (path text_pattern_ops);
