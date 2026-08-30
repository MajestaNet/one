-- ADR-015 amendment: every principal must store an email.
-- Social AuthN key remains identity_links (provider, issuer, subject).

-- Non-human rows should never have been null; synthesize if any slipped through.
UPDATE users
SET email = 'principal+' || id::text || '@one.local',
    updated_at = now()
WHERE (email IS NULL OR btrim(email) = '')
  AND principal_type IS DISTINCT FROM 'user';
--> statement-breakpoint

-- Brief 0027 window allowed social humans without email. Retire those rows so
-- NOT NULL can apply; they cannot satisfy the product invariant.
UPDATE users
SET email = 'retired+emailless+' || id::text || '@invalid.one.local',
    is_active = false,
    frozen_at = COALESCE(frozen_at, now()),
    frozen_reason = COALESCE(NULLIF(frozen_reason, ''), 'migration_0028_email_required'),
    updated_at = now()
WHERE principal_type = 'user'
  AND (email IS NULL OR btrim(email) = '');
--> statement-breakpoint

ALTER TABLE users ALTER COLUMN email SET NOT NULL;
--> statement-breakpoint

DROP INDEX IF EXISTS users_email_uidx;
--> statement-breakpoint
CREATE UNIQUE INDEX IF NOT EXISTS users_email_uidx ON users (lower(email));
--> statement-breakpoint

COMMENT ON COLUMN users.email IS 'Required contact identity; social AuthN key remains identity_links (provider, issuer, subject)';
