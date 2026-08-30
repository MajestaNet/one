-- BP-041: optional external-id flag on flexible object fields (upsert keys).
ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS external_id boolean NOT NULL DEFAULT false;
