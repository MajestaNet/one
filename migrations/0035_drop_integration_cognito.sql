-- Drop Cognito-shaped columns from product GA integration_configs.
-- Identity app clients are linked via identity_links (provider subject = client id).
-- AWS Cognito adapters live in sdk/aws (not product persistence).

ALTER TABLE integration_configs DROP COLUMN IF EXISTS cognito_app_client_id;
--> statement-breakpoint

ALTER TABLE integration_configs DROP COLUMN IF EXISTS cognito_secret_enc;
