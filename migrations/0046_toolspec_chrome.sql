-- ToolSpec chrome on metadata_canvases (ADR-021 / BP-050 Phase 2).
-- Keep table name; /metadata/v1/tools is an API alias over the same rows.

ALTER TABLE metadata_canvases
  ADD COLUMN IF NOT EXISTS icon text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS sort_order integer NOT NULL DEFAULT 0;

COMMENT ON COLUMN metadata_canvases.icon IS 'Allowlisted Run rail icon id (empty = default glyph)';
COMMENT ON COLUMN metadata_canvases.sort_order IS 'Run rail sort key (ascending; then label)';
