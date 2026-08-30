# ADR-006: Majesta One JWT AuthN and Role + Permission-Set AuthZ

## Status

Accepted — **amended** by [ADR-015](./015-idp-agnostic-social-login.md): Cognito is **not** the product default identity backend. One-issued JWT remains the only API bearer. P0–P1 Token Service shipped; social login broker + generic IdP exchange in progress per [idp-agnostic-login-build-plan.md](../architecture/idp-agnostic-login-build-plan.md). AuthZ packaging amended by [ADR-009](./009-record-audit-authz-packaging.md). **Amended 2026-08-20:** interactive human grants may also receive an **opaque refresh token** so desktop clients can mint a new access JWT without Sign in — [refresh-token-session-build-plan.md](../architecture/refresh-token-session-build-plan.md) / [BP-063](../../backlog/BP-063-refresh-token-sessions.md). See [BP-013](../../backlog/BP-013-jwt-unified-principals.md).

## Context

Majesta One is dedicated install: each customer runs an install (ADR-001) on their own infra (Compose / Helm first; optional AWS). Callers include:

1. Humans via Control IDE / Admin UI — **Google / Apple social login** (product default per ADR-015) or a customer OIDC IdP via token exchange.
2. Development / ops agents and integration services — Majesta One principals (`principal_credentials`); optional Cognito app-client write-through when `IDENTITY_SYNC=cognito`.
3. Machines / CI / Deploy bots — least-privilege family scopes (`client` / `metadata` / `deploy`, ADR-004).

AuthZ (Roles, permission sets, object/field grants) must stay install-local in Postgres. No external IdP (Cognito, Google, Okta, …) may become a second AuthZ system.

## Decision

### 1. Majesta One is the access-token issuer

Every API caller presents a **One-issued JWT** (`Authorization: Bearer`) on `/client`, `/metadata`, `/deploy`, and `/ops`.

**Login / identity proof** is adapter-based ([ADR-015](./015-idp-agnostic-social-login.md)):

- **Default human login:** thin Go social broker (`/auth/v1/authorize` → Google | Apple → Majesta One JWT). Email is optional for social-provisioned users.
- **Customer IdPs:** Okta / Entra / Keycloak / Cognito Hosted UI → OIDC ID token → `POST /auth/v1/token/exchange` → Majesta One JWT (documented adapters).
- **Optional AWS:** Cognito User Pool write-through (`IDENTITY_SYNC=cognito`) for operators who choose that example — **not** required for GA / Compose / Helm.
- Direct Google / Apple / Cognito / IdP access tokens are **not** authorized for family routes.

Opaque `API_KEYS` remain **bootstrap / break-glass** only, bound to distinct service principals.

### 2. Unified principals (Majesta One SoR)

Every caller is a `users` row with:

| Field | Purpose |
|---|---|
| `principal_type` | `user` \| `service` \| `agent` |
| Roles (required ≥1) | API family scopes (+ optional admin) only — see ADR-009 |
| Permission sets | Object / field AuthZ via **direct** `user_permission_sets` |
| `identity_links` | Provider `sub` (Google / Apple / OIDC / Cognito / Slack) or external app client id |

Setup path: **create principal (Client identity admin) → assign role(s) / permission set(s) → link identity (social login, exchange, or optional Cognito write-through) → issue Majesta One credential and/or complete login/exchange → call APIs as that `sub`**. Social users may omit email.

### 3. Identity admin lives on the Client API

Creating and managing people and integration accounts does **not** change customer metadata shape. Those APIs belong to **Client** (`scope: client`). Human principal and directory-tag ops use `identity.users`; `service` / `agent` principal and tagging those types use `identity.integrations`. Role / permission-set **assignment** is `authz.manage`. (`identity.manage` remains a **legacy alias** that expands to both user + integration caps — do not use it as the only gate.)

| Endpoint | Purpose |
|---|---|
| `/client/v1/principals` | CRUD for `user` \| `service` \| `agent` (SCIM-shaped fields, freeze/unfreeze — BP-017) |
| `/client/v1/principals/{id}/credentials` | Majesta One `client_secret` issue / list / revoke |
| `/client/v1/integrations` | Connected Apps — OAuth client configs (CRUD); optional IdP app-client write-through |
| `/client/v1/integrations/{apiName}/secrets/rotate` | Rotate Majesta One (+ optional IdP) secrets for confidential clients |
| `/client/v1/integrations/{apiName}/secrets/reveal` | Admin retrieve-after for encrypted secrets (audited) |
| `/client/v1/roles` | List + customer Role CRUD |
| `/client/v1/roles/assign` | Assign a Role to a principal |
| `/client/v1/roles/unassign` | Unassign a Role (last-role guard) |
| `/client/v1/permissions/assign` | Assign a permission set to a principal |
| `/client/v1/permissions/unassign` | Unassign a permission set |
| `/client/v1/directory-tags` | Directory tags (non-AuthZ groupings); assign/unassign membership |
| `/scim/v2/Users` | SCIM 2.0 Users adapter (same SoR; [scim-provisioning.md](../architecture/scim-provisioning.md)) |
| `/scim/v2/Groups` | SCIM Group = directory tag (membership only; never Roles / PS / data roles) |

Directory productionization (SCIM-shaped user fields, freeze/unfreeze, Role create, role-required-on-create, multi-PS assign UX) is planned in [identity-directory-productionization.md](../architecture/identity-directory-productionization.md) / BP-017. Protocol SCIM ships as a Client-owned adapter at [`/scim/v2`](../architecture/scim-provisioning.md) (BP-017 Phase 4).

OOTB, `AUTO_SEED` with `SEED_CONTROL_IDE` (default on) ensures managed integration `one.controlIde` (public `authorization_code` + PKCE, including `offline_access`) for Control IDE — readable via the same list/get APIs as customer integrations. Operators may set `SEED_CONTROL_IDE=0` to skip that app; claim, MCP, password, and Client CRUD still work. Default password / claim / token-exchange azp is `one.install`, not Control IDE.

Permission-set **definitions** and Metadata object shape remain under `/metadata/v1` (`authz.manage` for creating permission sets). Role/PS **assignment** is Client identity admin (`authz.manage`) — Admin UI needs `client` scope (and usually `metadata` only when defining PS/objects). Directory tags / SCIM Groups do **not** require `authz.manage`.

### 4. JWT contract

Install-local signing keys (env / Secrets Manager). Claims:

```json
{
  "iss": "https://<install>/auth/v1",
  "sub": "<user_uuid>",
  "aud": ["one"],
  "exp": 0,
  "iat": 0,
  "principal_type": "user|service|agent",
  "scopes": ["client", "metadata"],
  "roles": ["SalesRep"],
  "admin": false
}
```

**Rules:**

- JWT proves **who** and **which family scopes** (fast reject at the mux).
- **Effective AuthZ** always resolved by `sub` from the database — never trust IdP groups or permission-set IDs in tokens.
- Role-derived scopes are authoritative when Roles are loaded; principals with zero roles are rejected.
- Admin privilege does **not** bypass missing family scopes (ADR-004).

### 4b. Refresh tokens (desktop / interactive humans)

Access JWTs stay **short-lived** (default 3600s, `AUTH_JWT_TTL_SECONDS`). Do not extend access TTL to fake “stay signed in.”

Interactive **human** grants (`authorization_code`, `password`, token-exchange, install claim) may also return an **opaque refresh token**:

- Stored as SHA-256 in kernel `refresh_tokens` (never a long-lived JWT; never decryptable ciphertext of the Bearer).
- Rotated on every successful refresh; presenting a rotated token revokes the **family**.
- Idle 30 days (sliding) and absolute 90 days from family creation (config: `AUTH_REFRESH_IDLE_SECONDS` / `AUTH_REFRESH_ABS_SECONDS`).
- Control IDE (`azp=one.controlIde`) is a **public** Connected App: it receives a refresh token when the token request includes `offline_access`, and stores it in the encrypted session file ([CIDE-10](../architecture/control-ide-security-audit.md)). Generic install sessions (`azp=one.install` from claim / password without `client_id` / token-exchange default) also receive refresh.
- `client_credentials`, API keys, and pasted access JWTs do **not** get refresh tokens.
- Browser Client Experience apps get a refresh token only when they request `offline_access` (default off).
- Effective AuthZ is re-loaded from Postgres on every refresh mint. Freeze, password change, and Sign out revoke refresh families immediately; already-issued access JWTs die at `exp`.

Full contract: [refresh-token-session-build-plan.md](../architecture/refresh-token-session-build-plan.md).

Auth surface:

| Endpoint | Purpose |
|---|---|
| `GET /auth/v1/authorize` | Social PKCE start (Google \| Apple) — ADR-015 |
| `GET /auth/v1/callback/{provider}` | Social callback → Majesta One auth code |
| `POST /auth/v1/token` | `client_credentials` or `authorization_code` (+ PKCE) or `password` → Majesta One access JWT; interactive human grants also return an opaque `refresh_token` ([BP-063](../../backlog/BP-063-refresh-token-sessions.md)) |
| `POST /auth/v1/token` (`grant_type=refresh_token`) | Rotate refresh token → new Majesta One access JWT (Actor re-loaded from DB) |
| `POST /auth/v1/revoke` | Revoke a refresh family (RFC 7009-shaped) |
| `POST /auth/v1/token/exchange` | External IdP ID token (OIDC / Slack later) → Majesta One JWT (`azp=one.install` unless OIDC aud maps to a Connected App; refresh when eligible) |
| Discovery | `GET /auth/v1/.well-known/openid-configuration` |

### 5. Optional Cognito sync (adapter only)

When `IDENTITY_SYNC=cognito`, Client identity admin may write-through to a customer/AWS User Pool (`AdminCreateUser` / `CreateUserPoolClient`) and record `identity_links`. Lookup at exchange/login time is by `(provider, issuer, subject)` → Majesta One user (or auto-provision when enabled).

Do **not** rely on Cognito Lambda triggers as the primary consistency mechanism. Do **not** require Cognito for Compose/Helm GA.

### 6. AuthZ = Role (scopes) + Permission Set (object/field)

Unchanged from ADR-009: family scopes from Roles; object/field/system capabilities from permission sets; Deploy peer trust unchanged.

### 7. Edge

TLS at the edge (optional WAF / install exposure policy). No ALB/Ingress IdP authenticate for machine traffic. Packaging-only gateways do not move AuthZ out of Majesta One JWT.

## Consequences

- OSS / Compose / Helm installs use social broker + Majesta One JWT without AWS Cognito.
- Admin UI manages users via **Client** identity APIs; optional Cognito sync when operators enable it.
- BYO IdP is an exchange adapter; AuthZ never moves to the IdP.
- Majesta One JWT + Postgres AuthZ remain mandatory (ADR-015).
- Interactive human sessions (Control IDE) use opaque refresh tokens for multi-day reopen; family APIs still require a short-lived access JWT.

## Non-goals (near term)

- Cognito (or any cloud IdP) as a required product dependency
- Embedded Keycloak / full in-process authorization server
- Email OTP / magic-link login (password grant already exists when enabled — BP-037)
- ALB `authenticate-cognito` (or equivalent) for all API traffic
- Trusting IdP groups as AuthZ SoR
- Cognito Lambda sync triggers as primary write path

Protocol SCIM (`/scim/v2`) is accepted as a Client identity adapter — see [scim-provisioning.md](../architecture/scim-provisioning.md).

## Related

- [ADR-015: IdP-agnostic social login](./015-idp-agnostic-social-login.md)
- [IdP-agnostic login build plan](../architecture/idp-agnostic-login-build-plan.md)
- [ADR-009: Record audit + AuthZ packaging](./009-record-audit-authz-packaging.md)
- [ADR-004: Three API families](./004-three-api-families.md)
- [ADR-001: Dedicated install deploy](./001-dedicated-install.md)
- [BP-013: Majesta One JWT + unified principals](../../backlog/BP-013-jwt-unified-principals.md)
- [BP-063: Refresh-token sessions](../../backlog/BP-063-refresh-token-sessions.md)
- [Refresh-token session build plan](../architecture/refresh-token-session-build-plan.md)
- [Security posture](../security.md)
