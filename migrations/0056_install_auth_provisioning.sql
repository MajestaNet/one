-- BP-058 Phase 4: install-local JIT/SCIM provisioning defaults + claim maps.
ALTER TABLE organization_settings
  ADD COLUMN IF NOT EXISTS provisioning jsonb NOT NULL DEFAULT '{}'::jsonb;
