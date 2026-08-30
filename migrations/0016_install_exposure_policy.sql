-- Install exposure policy: desired-state for path-based WAF / edge access (Metadata admin).
CREATE TABLE IF NOT EXISTS install_exposure_policy (
  id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'pending',
  last_error text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  applied_at timestamptz,
  CONSTRAINT install_exposure_policy_status_check
    CHECK (status IN ('pending', 'applied', 'error'))
);

INSERT INTO install_exposure_policy (id, policy, status)
VALUES (
  1,
  '{
    "client":   {"mode": "public", "cidrs": []},
    "auth":     {"mode": "public", "cidrs": []},
    "metadata": {"mode": "blocked", "cidrs": []},
    "deploy":   {"mode": "blocked", "cidrs": []},
    "ops":      {"mode": "blocked", "cidrs": []}
  }'::jsonb,
  'applied'
)
ON CONFLICT (id) DO NOTHING;
