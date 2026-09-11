-- BP-069 / ADR-033: persisted functional description on automations.
-- Length is enforced in Go (Unicode-safe rune count ≤ 500), not a SQL CHECK.
ALTER TABLE metadata_automations
  ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';

COMMENT ON COLUMN metadata_automations.description IS
  'Short functional description (max 500 Unicode characters). Required on managed; optional on custom.';
