# BP-013: Majesta One JWT issuer and unified principals

- **Severity:** High
- **Status:** Partially mitigated (principals, Token Service, OIDC exchange hardening, and Slack OpenID shipped; P2 `parent_role_id` Keep unused)
- **Area:** `internal/authz`, `internal/authlogin`, `cmd/api`, `/auth/v1`, kernel migrations, Control IDE Connect, deploy docs
- **ADR:** [ADR-006](../docs/adr/006-jwt-auth.md), [ADR-015](../docs/adr/015-idp-agnostic-social-login.md), [ADR-009](../docs/adr/009-record-audit-authz-packaging.md)
- **Plan:** [idp-agnostic-login-build-plan.md](../docs/architecture/idp-agnostic-login-build-plan.md)
- **Remainder (Finish slot 6):** [06-bp-013-037-jwt-claim-sso.md](../docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md) (grouped with [BP-037](./BP-037-install-claim-customer-sso.md))

## Problem

AuthN was split between env API keys (machines collapse to `DEFAULT_OWNER_ID`) and optional Cognito-compatible OIDC for users. There was no single principal model for users, agents, and service accounts. Cognito as a One-owned **default** dependency is the wrong long-term shape for OSS Compose/Helm installs and for cloud-agnostic self-host ([BP-011](./BP-011-container-marketplace-fargate.md)).

## Why it matters

Separating `user` | `service` | `agent` exists so each can hold **customer-allowed customization grants** under one AuthZ model. Without a One-issued JWT and Role-backed principals:

- Slack / UI actions cannot safely run as the end user
- Agents on Metadata share admin-like blast radius instead of least-privilege grants
- Customers cannot connect arbitrary clients to their install with one AuthN contract
- Field / Metadata resource AuthZ (BP-003) and principal-parity customization enforcement (BP-006) have no stable principal to attach to

## Direction

Per [ADR-006](../docs/adr/006-jwt-auth.md) / [ADR-015](../docs/adr/015-idp-agnostic-social-login.md) / [ADR-009](../docs/adr/009-record-audit-authz-packaging.md) and the [build plan](../docs/architecture/idp-agnostic-login-build-plan.md):

1. Majesta One Token Service mints access JWTs; external IdPs are **exchange / social adapters**
2. Every caller is a `users` row (`user` | `service` | `agent`) with ≥1 Role → scopes; permission sets assigned to the user for object/field AuthZ
3. Effective AuthZ always loaded from DB by `sub`
4. Optional human login adapters use the thin Go social/OIDC broker; a profile email is required for JIT, and providers that attest email verification (Google / Apple / Slack) must attest it before first provision
5. Cognito is an **optional** AWS write-through adapter — remove from GA AuthN path
6. Document Okta / Entra / Keycloak / Cognito Hosted UI as OIDC exchange adapters (no embedded Keycloak)
7. Principal type does not grant powers by itself — grants do ([BP-006](./BP-006-agent-guardrails.md))
8. Durable desktop sessions use opaque refresh tokens — do not lengthen access JWT TTL ([BP-063](./BP-063-refresh-token-sessions.md))

## Phased delivery

### P0 — Contract and principals — **shipped (schema + packaging)**

- Schema: `principal_type` (`user`|`service`|`agent`), `user_roles`, `role_api_scopes`, `principal_credentials`, `identity_links` (`0008` + `0013`)
- `role_permission_sets` removed; PS grants only via `user_permission_sets`
- Seed system Roles `SystemAdmin` / `StandardUser`; bootstrap gets SystemAdmin + Admin PS
- Actor permission sets loaded from DB (direct only); Role scopes required on AuthN paths
- Bootstrap `API_KEYS` bound via `users.api_key_name` + `EnsureAPIKeyServicePrincipal`

### P1 — Token Service — **shipped (MVP)**; install claim / customer SSO — **BP-037**

- `POST /auth/v1/token` (`grant_type=client_credentials`) — mint Majesta One HS256 JWT
- `POST /auth/v1/token` (`grant_type=password`) — human password when enabled (BP-037)
- `POST /auth/v1/install/claim` + `GET /auth/v1/install/status` — day-0 SystemAdmin (BP-037)
- `GET|PUT /metadata/v1/install/auth` — customer SSO / JIT / social enable (BP-037)
- `POST /auth/v1/token/exchange` — OIDC ID tokens → Majesta One JWT
- Google / Apple social broker — **optional** (customer enable); not product default
- Okta / Entra / Keycloak docs — [auth-adapters.md](../docs/auth-adapters.md)
- Durable Control IDE session (refresh tokens) — **[BP-063](./BP-063-refresh-token-sessions.md)** (not in this MVP)

August 2026 hardening made API-key configuration authoritative per principal,
removed raw bootstrap secrets from persisted/user-visible identity metadata,
made permission/Role lookup failures fail closed, required strict JWT/OIDC
claims and algorithms, encrypted customer OIDC client secrets at rest, and
rejected identity-link rebinding and email-only OIDC account linking. Migration
`0049` scrubs legacy bootstrap-key metadata.

The 2026-08-25 backend security review additionally made token exchange
request-local (no shared verifier mutation), issuer-scoped legacy-link migration,
JIT Role/domain/provisioning policy, and first-human SystemAdmin bootstrap
consistent with browser login. The first-human election is serialized across API
replicas so concurrent initial sign-ins cannot both become SystemAdmin.
Authentication/exposure policy-store failures now fail closed instead of falling
back to environment/default policy.

### P2 — Resource AuthZ completion (with BP-003)

- Field permissions, Metadata capabilities, system permissions — **shipped** (remainders in BP-003)
- Optional role hierarchy via `roles.parent_role_id` — **Keep (unused column)**. Sharing uses `data_roles` (ADR-016). Do not walk this column for API scopes or record visibility.

### P3 — Channel / IdP adapters

| Adapter | Status |
|---|---|
| Cognito / OIDC ID token → exchange | Shipped (generalize provider string — plan Phase 2) |
| Google / Apple social broker | **Shipped** (plan Phase 3–4) |
| Okta / Entra / Keycloak docs | **Shipped** ([auth-adapters.md](../docs/auth-adapters.md)) |
| Slack identity → exchange | **Shipped** (OpenID Connect user identity; `identity_links.provider=slack`) |
| Cognito as GA default | **Cancelled** — optional AWS adapter only |

## Scenario checklist

| Scenario | Principal | Credential | Enforced as |
|---|---|---|---|
| Control IDE (Google/Apple) | `user` + social `identity_links` | Majesta One JWT after PKCE | That user’s Roles / PS |
| Customer Okta → APIs | `user` + OIDC link | JWT after `/token/exchange` | That user |
| Customer Slack → APIs | `user` + Slack link | Short-lived JWT after exchange | That user |
| Customer admin customizes Metadata | `user` | Majesta One JWT | Scopes + Metadata capabilities |
| Integration promotes customer bundle | `service` | Client credentials → JWT | Deploy scope + grants |
| Dev agent on Metadata | `agent` | Client credentials → JWT | Agent grants — not SystemAdmin by default |

## Related

- [Remainder tech design (BP-013 + BP-037)](../docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md)
- [Build plan](../docs/architecture/idp-agnostic-login-build-plan.md)
- [BP-063](./BP-063-refresh-token-sessions.md) — refresh-token sessions / silent IDE re-auth
- [BP-003](./BP-003-enterprise-auth.md) — AuthZ remainders
- [BP-006](./BP-006-agent-guardrails.md) — principal parity customization AuthZ
- [BP-017](./BP-017-identity-directory-productionization.md) — directory / SCIM (email may still be required on SCIM create)
- [BP-011](./BP-011-container-marketplace-fargate.md) — Compose/Helm; Cognito not required
- [BP-022](./BP-022-client-access-ide-device.md) — IDE PKCE surface
- [ADR-004](../docs/adr/004-three-api-families.md) · [ADR-009](../docs/adr/009-record-audit-authz-packaging.md)
