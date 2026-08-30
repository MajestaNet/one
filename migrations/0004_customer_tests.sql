-- Phase D: customer test suites + runs
CREATE TABLE IF NOT EXISTS customer_tests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL,
  label text NOT NULL,
  description text,
  active boolean NOT NULL DEFAULT true,
  steps jsonb NOT NULL DEFAULT '[]'::jsonb,
  package_name text,
  ownership text NOT NULL DEFAULT 'custom',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS customer_tests_api_name_uniq ON customer_tests(api_name);
--> statement-breakpoint
CREATE TABLE IF NOT EXISTS customer_test_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  suite_api_name text NOT NULL,
  status text NOT NULL DEFAULT 'queued',
  trigger text NOT NULL DEFAULT 'api',
  results jsonb,
  summary jsonb,
  error text,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS customer_test_runs_suite_idx ON customer_test_runs(suite_api_name);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS customer_test_runs_created_idx ON customer_test_runs(created_at);
