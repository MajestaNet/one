-- BP-001 / ADR-013: drop global GIN on records.data now that expression indexes
-- (field_projections) cover hot filter paths. High-volume store never had this GIN.
-- Rollback: CREATE INDEX records_data_gin_idx ON records USING gin (data);

DROP INDEX IF EXISTS records_data_gin_idx;
