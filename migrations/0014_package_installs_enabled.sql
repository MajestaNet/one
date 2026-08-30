-- Soft-disable support for optional managed modules (BP-007 extension install)
ALTER TABLE package_installs
  ADD COLUMN IF NOT EXISTS enabled boolean NOT NULL DEFAULT true;
