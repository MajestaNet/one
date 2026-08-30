-- BP-043 / BP-020: maintained search document + pg_trgm GIN (not records.data GIN).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS searchable boolean NOT NULL DEFAULT false;

ALTER TABLE records
  ADD COLUMN IF NOT EXISTS search_document text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_subtitle text NOT NULL DEFAULT '';

ALTER TABLE records_hv
  ADD COLUMN IF NOT EXISTS search_document text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_subtitle text NOT NULL DEFAULT '';

-- Parent indexes propagate to LIST partitions.
CREATE INDEX IF NOT EXISTS records_search_document_trgm_idx
  ON records USING gin (search_document gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_search_title_trgm_idx
  ON records USING gin (search_title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_hv_search_document_trgm_idx
  ON records_hv USING gin (search_document gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_hv_search_title_trgm_idx
  ON records_hv USING gin (search_title gin_trgm_ops);
