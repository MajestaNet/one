-- Agent IDE conversations + principal preferences (Agentic Run uplift).

CREATE TABLE IF NOT EXISTS agent_conversations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  principal_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  playbook_api_name text,
  mode text NOT NULL DEFAULT 'operate',
  title text NOT NULL DEFAULT 'Agent chat',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_conversations_principal
  ON agent_conversations (principal_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS agent_conversation_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id uuid NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
  role text NOT NULL,
  body text NOT NULL DEFAULT '',
  parts jsonb NOT NULL DEFAULT '[]'::jsonb,
  run_id uuid REFERENCES agent_runs(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_conversation_messages_conv
  ON agent_conversation_messages (conversation_id, created_at);

ALTER TABLE agent_runs
  ADD COLUMN IF NOT EXISTS conversation_id uuid REFERENCES agent_conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation
  ON agent_runs (conversation_id) WHERE conversation_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS principal_preferences (
  principal_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL,
  document jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (principal_id, kind)
);

CREATE INDEX IF NOT EXISTS idx_principal_preferences_principal
  ON principal_preferences (principal_id, updated_at DESC);
