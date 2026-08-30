-- Principal-scoped CanvasDocument working sets (ADR-018 Phase 4d).

CREATE TABLE IF NOT EXISTS principal_canvas_documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  principal_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  canvas_id text NOT NULL,
  title text NOT NULL,
  document jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (principal_id, canvas_id)
);

CREATE INDEX IF NOT EXISTS idx_principal_canvas_documents_principal
  ON principal_canvas_documents (principal_id, updated_at DESC);
