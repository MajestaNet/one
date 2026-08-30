-- Harden install exposure for installs that still have the pre-GA all-public seed.
-- Only rewrite when the row still matches the legacy all-public document exactly.
UPDATE install_exposure_policy
SET
  policy = '{
    "client":   {"mode": "public", "cidrs": []},
    "auth":     {"mode": "public", "cidrs": []},
    "metadata": {"mode": "blocked", "cidrs": []},
    "deploy":   {"mode": "blocked", "cidrs": []},
    "ops":      {"mode": "blocked", "cidrs": []}
  }'::jsonb,
  status = 'pending',
  last_error = NULL,
  updated_at = now()
WHERE id = 1
  AND policy = '{
    "client":   {"mode": "public", "cidrs": []},
    "auth":     {"mode": "public", "cidrs": []},
    "metadata": {"mode": "public", "cidrs": []},
    "deploy":   {"mode": "public", "cidrs": []},
    "ops":      {"mode": "public", "cidrs": []}
  }'::jsonb;
