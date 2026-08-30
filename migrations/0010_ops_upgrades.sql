-- Ops product upgrades (ADR-007) + ops scope on roles
CREATE TABLE IF NOT EXISTS platform_upgrades (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status text NOT NULL DEFAULT 'pending',
  from_version text NOT NULL,
  to_version text NOT NULL,
  api_image text NOT NULL,
  worker_image text NOT NULL,
  previous_api_task_def text,
  previous_worker_task_def text,
  new_api_task_def text,
  new_worker_task_def text,
  test_run_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  gate_result jsonb,
  error text,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  CONSTRAINT platform_upgrades_status_check CHECK (
    status IN ('pending', 'rolling', 'gating', 'succeeded', 'failed', 'rolled_back')
  )
);
--> statement-breakpoint
CREATE INDEX IF NOT EXISTS platform_upgrades_created_idx ON platform_upgrades(created_at DESC);
--> statement-breakpoint
ALTER TABLE role_api_scopes DROP CONSTRAINT IF EXISTS role_api_scopes_scope_check;
--> statement-breakpoint
ALTER TABLE role_api_scopes ADD CONSTRAINT role_api_scopes_scope_check
  CHECK (scope IN ('client', 'metadata', 'deploy', 'ops', 'admin'));
