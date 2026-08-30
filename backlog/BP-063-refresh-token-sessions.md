# BP-063: Refresh-token sessions (desktop silent re-auth)

- **Severity:** High
- **Status:** Partially mitigated (Phases 1–3 shipped: kernel store, `/auth/v1` refresh/revoke, Control IDE silent refresh)
- **Area:** `internal/authz`, `internal/httpapi/auth_routes.go`, `internal/db`, `migrations/`, `tools/control-ide`
- **Plan:** [refresh-token-session-build-plan.md](../docs/architecture/refresh-token-session-build-plan.md)
- **ADR:** [ADR-006](../docs/adr/006-jwt-auth.md) (amended)
- **Related:** [BP-013](./BP-013-jwt-unified-principals.md) · [BP-022](./BP-022-client-access-ide-device.md) · [BP-040](./BP-040-client-experience-oss-kits.md)

## Problem

Control IDE can persist an encrypted Majesta One **access JWT** across Electron restarts ([PR #255](https://github.com/MajestaNet/ide/pull/255)), but that JWT expires in about **one hour** (`AUTH_JWT_TTL_SECONDS`). A customer user who signs in, quits the app, and reopens the next day is sent to Sign in even though they never signed out. Lengthening the access JWT would leave a stolen Bearer valid for days.

## Why it matters

Desktop apps in this class keep a refresh credential in OS-backed storage and mint new short-lived access tokens. Without a Token Service refresh grant:

- Control IDE cannot offer a durable human session
- Operators will work around it (paste long-lived keys, raise JWT TTL) and widen blast radius
- Freeze / password change cannot invalidate the *next* mint until the current JWT expires — refresh revocation is the kill-switch for the durable credential

## Direction

Per [refresh-token-session-build-plan.md](../docs/architecture/refresh-token-session-build-plan.md):

1. Keep family APIs on short-lived Majesta One access JWTs
2. Issue **opaque** refresh tokens (SHA-256 hashed in kernel `refresh_tokens`) on interactive human grants (`authorization_code`, `password`, token-exchange, IDE-bound claim)
3. Rotate on every refresh; reuse of a rotated token revokes the family
4. Control IDE stores the refresh token in the existing encrypted session file and silently refreshes on boot / 401 / skew
5. Do **not** issue refresh tokens for `client_credentials` or pasted JWTs
6. Browser Client Experience refresh is opt-in (`offline_access`); default remains no long-lived token in `localStorage`

## Phased delivery

| Phase | Work | Status |
|---|---|---|
| 0 | Plan + ADR-006 amend + this BP | Done |
| 1 | Kernel table + store + domain rotation | Done |
| 2 | `/auth/v1/token` refresh grant + `/auth/v1/revoke` | Done |
| 3 | Control IDE silent refresh | Done |
| 4 | Optional: list/revoke other session families | Open |
| 5 | Optional: `sdk/client` `@one/auth` helper | Open |

The 2026-08-25 backend review moved principal and exposure-policy validation
before rotation. A transient or corrupt policy-store result therefore fails
closed without consuming the caller's valid refresh token and losing the
replacement response.

## Explicit non-goals

- Multi-day access JWTs
- Refresh JWTs / access-token denylist
- Cross-install SSO
- Device-cert-gated refresh (BP-022 remainder)
