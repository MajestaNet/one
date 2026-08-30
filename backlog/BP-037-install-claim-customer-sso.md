# BP-037: Install claim, customer SSO, and JIT

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `internal/authz`, `internal/authlogin`, `internal/httpapi`, `internal/db`, `migrations/`, `tools/control-ide`, `deploy/`
- **Plan:** [install-claim-sso-build-plan.md](../docs/architecture/install-claim-sso-build-plan.md)
- **Remainder (Finish slot 6):** [06-bp-013-037-jwt-claim-sso.md](../docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md) (grouped with [BP-013](./BP-013-jwt-unified-principals.md))
- **Related:** [BP-013](./BP-013-jwt-unified-principals.md), [ADR-015](../docs/adr/015-idp-agnostic-social-login.md), [BP-029](./BP-029-app-platform-install.md)

## Problem

Easy DigitalOcean / App Platform (and AWS/Azure Helm) installs must not rely on URL obscurity or Google/Apple as the default human path. Day-0 needs a one-time claim that works **without Control IDE**. Customers must configure their own SSO and optionally JIT thereafter.

## What shipped

1. `INSTALL_CLAIM_TOKEN` + `POST /auth/v1/install/claim` (email + password → first SystemAdmin)
2. `GET /auth/v1/install/status`; password `grant_type` on `/auth/v1/token`
3. `GET|PUT /metadata/v1/install/auth` (SSO, JIT, social enable, password login)
4. Login page: claim form when unclaimed; SSO primary when configured; social only if enabled
5. Control IDE Connect claim + Govern Install auth panel
6. Deploy/Helm/docs secrets checklist

## Remaining

Honest status after remainder inventory ([06-bp-013-037-jwt-claim-sso.md](../docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md)):

- Encrypt OIDC client secrets at rest — **shipped** (`enc:v1` via `secretcrypt`; legacy `plain:` still readable). Keep/regression only; do not re-implement.
- Full OIDC discovery edge-case hardening / multi-IdP — still open (exchange path guesses JWKS and requires Cognito `token_use`; one IdP on `organization_settings`)
- Rotate claim token after claim UX polish — hash already NULLed on claim; hosted HTML `format=redirect` and `one auth claim` still open

## Explicit non-goals

- Email magic-link / OTP
- Embedded Keycloak
- Cross-install SSO
