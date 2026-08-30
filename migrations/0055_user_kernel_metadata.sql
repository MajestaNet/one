-- BP-058 Phases 1–2: kernel User metadata object storage + customer profile JSONB.
ALTER TABLE users ADD COLUMN IF NOT EXISTS data jsonb NOT NULL DEFAULT '{}'::jsonb;
--> statement-breakpoint
ALTER TABLE users ADD COLUMN IF NOT EXISTS employee_number text;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_employee_number_uidx ON users (employee_number) WHERE employee_number IS NOT NULL;
--> statement-breakpoint
ALTER TABLE metadata_fields ADD COLUMN IF NOT EXISTS kernel_column text;
