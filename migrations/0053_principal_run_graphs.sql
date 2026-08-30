-- Principal-scoped, reference-only Run personal graphs (ADR-023 / BP-055).

CREATE TABLE IF NOT EXISTS principal_run_graphs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  principal_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  graph_key text NOT NULL DEFAULT 'home',
  title text NOT NULL DEFAULT 'My graph',
  document jsonb NOT NULL,
  revision bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (principal_id, graph_key)
);

CREATE INDEX IF NOT EXISTS idx_principal_run_graphs_principal
  ON principal_run_graphs (principal_id, updated_at DESC);
