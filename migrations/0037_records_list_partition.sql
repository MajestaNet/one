-- ADR-013 Tier C / BP-035: LIST-partition shared records by object_api_name.
-- PK includes partition key (Postgres requirement). Partial live-row indexes;
-- drop redundant single-column indexes from the legacy heap.

ALTER TABLE records RENAME TO records_legacy;

CREATE TABLE records (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  object_api_name text NOT NULL,
  owner_id uuid REFERENCES users(id),
  created_by_id uuid NOT NULL REFERENCES users(id),
  last_modified_by_id uuid NOT NULL REFERENCES users(id),
  data jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  PRIMARY KEY (id, object_api_name)
) PARTITION BY LIST (object_api_name);

-- Core
CREATE TABLE IF NOT EXISTS records_o_account PARTITION OF records FOR VALUES IN ('Account');
CREATE TABLE IF NOT EXISTS records_o_contact PARTITION OF records FOR VALUES IN ('Contact');
-- notes
CREATE TABLE IF NOT EXISTS records_o_note PARTITION OF records FOR VALUES IN ('Note');
-- catalog
CREATE TABLE IF NOT EXISTS records_o_product PARTITION OF records FOR VALUES IN ('Product');
CREATE TABLE IF NOT EXISTS records_o_pricelist PARTITION OF records FOR VALUES IN ('PriceList');
CREATE TABLE IF NOT EXISTS records_o_pricelistentry PARTITION OF records FOR VALUES IN ('PriceListEntry');
-- sales
CREATE TABLE IF NOT EXISTS records_o_opportunity PARTITION OF records FOR VALUES IN ('Opportunity');
CREATE TABLE IF NOT EXISTS records_o_opportunitycontactrole PARTITION OF records FOR VALUES IN ('OpportunityContactRole');
CREATE TABLE IF NOT EXISTS records_o_quote PARTITION OF records FOR VALUES IN ('Quote');
CREATE TABLE IF NOT EXISTS records_o_quoteline PARTITION OF records FOR VALUES IN ('QuoteLine');
-- service
CREATE TABLE IF NOT EXISTS records_o_case PARTITION OF records FOR VALUES IN ('Case');
CREATE TABLE IF NOT EXISTS records_o_casecomment PARTITION OF records FOR VALUES IN ('CaseComment');
CREATE TABLE IF NOT EXISTS records_o_asset PARTITION OF records FOR VALUES IN ('Asset');
CREATE TABLE IF NOT EXISTS records_o_servicecontract PARTITION OF records FOR VALUES IN ('ServiceContract');
CREATE TABLE IF NOT EXISTS records_o_entitlement PARTITION OF records FOR VALUES IN ('Entitlement');
CREATE TABLE IF NOT EXISTS records_o_contractlineitem PARTITION OF records FOR VALUES IN ('ContractLineItem');
CREATE TABLE IF NOT EXISTS records_o_workorder PARTITION OF records FOR VALUES IN ('WorkOrder');
-- obscure customer / future managed objects
CREATE TABLE IF NOT EXISTS records_default PARTITION OF records DEFAULT;

INSERT INTO records (
  id, object_api_name, owner_id, created_by_id, last_modified_by_id,
  data, created_at, updated_at, deleted_at
)
SELECT
  id, object_api_name, owner_id, created_by_id, last_modified_by_id,
  data, created_at, updated_at, deleted_at
FROM records_legacy;

DROP TABLE records_legacy;

-- Catalog of dedicated flexible LIST partitions (product/worker DDL).
CREATE TABLE IF NOT EXISTS flexible_objects (
  object_api_name text PRIMARY KEY,
  partition_name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO flexible_objects (object_api_name, partition_name) VALUES
  ('Account', 'records_o_account'),
  ('Contact', 'records_o_contact'),
  ('Note', 'records_o_note'),
  ('Product', 'records_o_product'),
  ('PriceList', 'records_o_pricelist'),
  ('PriceListEntry', 'records_o_pricelistentry'),
  ('Opportunity', 'records_o_opportunity'),
  ('OpportunityContactRole', 'records_o_opportunitycontactrole'),
  ('Quote', 'records_o_quote'),
  ('QuoteLine', 'records_o_quoteline'),
  ('Case', 'records_o_case'),
  ('CaseComment', 'records_o_casecomment'),
  ('Asset', 'records_o_asset'),
  ('ServiceContract', 'records_o_servicecontract'),
  ('Entitlement', 'records_o_entitlement'),
  ('ContractLineItem', 'records_o_contractlineitem'),
  ('WorkOrder', 'records_o_workorder')
ON CONFLICT (object_api_name) DO NOTHING;

-- Partial live-row composites (replaces non-partial + redundant single-column indexes).
CREATE INDEX IF NOT EXISTS records_object_owner_live_idx
  ON records (object_api_name, owner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS records_object_created_id_live_idx
  ON records (object_api_name, created_at, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS records_object_created_by_live_idx
  ON records (object_api_name, created_by_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS records_object_last_modified_by_live_idx
  ON records (object_api_name, last_modified_by_id) WHERE deleted_at IS NULL;

-- Expression indexes on the old heap were dropped with records_legacy; rebuild via worker.
UPDATE field_projections
SET status = 'pending', built_at = NULL, last_error = NULL
WHERE object_api_name NOT IN (SELECT object_api_name FROM high_volume_objects);
