# @one/auth

PKCE and token exchange helpers for customer-hosted **Client Experience** apps.

**Scope:** `/auth/v1` only. Do not use for Metadata or Deploy.

- Default authorize scopes are `["client"]`.
- `buildAuthorizeUrl` **throws** if scopes include `metadata`, `deploy`, `ops`, or `admin` (before redirect).
- `POST /auth/v1/token` sends `One-API-Revision` (default `PREFERRED_API_REVISION = 1`).
- Inject `fetch` on `OneAuthConfig` for tests. Refresh / revoke / token exchange land in a later remainder (R2).

```bash
npm test   # tsc + node --test (mocked fetch; no live install)
npm run build
```

See [docs/client-experience-security.md](../../../docs/client-experience-security.md).
