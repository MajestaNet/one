-- API_KEYS are credentials, not principal metadata. Older releases copied the
-- raw bootstrap secret into users.api_key_name/email/display_name. Replace it
-- with the same domain-separated identifier used by authz.APIKeyIdentifier so
-- inactive and removed bootstrap principals are scrubbed without requiring a
-- successful login.
UPDATE users
SET api_key_name = 'apikey-' || encode(
      sha256(convert_to('one-apikey-cmp:' || api_key_name, 'UTF8')),
      'hex'
    ),
    email = 'apikey-' || encode(
      sha256(convert_to('one-apikey-cmp:' || api_key_name, 'UTF8')),
      'hex'
    ) || '@one.local',
    display_name = 'Bootstrap API Key ' || right(encode(
      sha256(convert_to('one-apikey-cmp:' || api_key_name, 'UTF8')),
      'hex'
    ), 12),
    updated_at = now()
WHERE api_key_name IS NOT NULL
  AND api_key_name <> ''
  AND api_key_name !~ '^apikey-[0-9a-f]{64}$';
