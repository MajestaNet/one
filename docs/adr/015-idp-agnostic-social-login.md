# ADR-015: IdP-agnostic login (customer SSO + claim; social optional)

## Status

Accepted — amends identity-backend defaults in [ADR-006](./006-jwt-auth.md). One-issued JWT remains the only family API bearer. AuthZ SoR remains Postgres (ADR-009).

**Amended:** day-0 **install claim** with local password; product login primary = **customer SSO** when configured; Google/Apple social is **optional** (explicit customer enable). See [install-claim-sso-build-plan.md](../architecture/install-claim-sso-build-plan.md) / [BP-037](../../backlog/BP-037-install-claim-customer-sso.md).

**Build plan (social broker):** [idp-agnostic-login-build-plan.md](../architecture/idp-agnostic-login-build-plan.md)  
**Backlog:** [BP-013](../../backlog/BP-013-jwt-unified-principals.md), [BP-037](../../backlog/BP-037-install-claim-customer-sso.md), [BP-011](../../backlog/BP-011-container-marketplace-fargate.md), [BP-022](../../backlog/BP-022-client-access-ide-device.md)

## Context

ADR-006 made Cognito the default **identity / login backend** per install while Majesta One JWT stayed the API bearer. That conflicts with the OSS / Compose+Helm center of gravity ([BP-011](../../backlog/BP-011-container-marketplace-fargate.md)): Cognito must not be a required Majesta One dependency.

We need:

1. Human login that works without AWS Cognito.
2. No embedded Keycloak / Java IdP in the product image (ADR-005).
3. A clear adapter path for customer IdPs (Okta, Entra, Keycloak, Cognito).
4. A day-0 first-admin path that works **without** Control IDE and **without** requiring Google/Apple (install claim + email/password).
5. Login UX that defaults to the **customer-configured SSO**, not consumer social buttons.

## Decision

### 1. Layers (unchanged ownership)

| Layer | Owner | Notes |
|---|---|---|
| API AuthN | Majesta One JWT (`/auth/v1/token`, exchange, ResolveBearer) | Only bearer on `/client`, `/metadata`, `/deploy`, `/ops` |
| AuthZ | Postgres Roles + permission sets | Never IdP groups |
| Login / proof | **Adapters** | Install claim + password; customer OIDC SSO; optional Google/Apple broker; optional Cognito write-through |

### 2. Day-0 = install claim (password); steady-state = customer SSO

1. Operator sets `INSTALL_CLAIM_TOKEN` (hashed into `organization_settings`).
2. `POST /auth/v1/install/claim` with token + email + password creates the first human `SystemAdmin`, stores a `password` credential, marks the install claimed, returns a Majesta One JWT.
3. Admin configures SSO via `PUT /metadata/v1/install/auth` (and/or Control IDE Govern panel).
4. Login page (`GET /auth/v1/login`): unclaimed → claim form; claimed + SSO → **Continue with {IdP}**; password form when enabled; Google/Apple **only** when `socialProviders` (or lab env) enables them.

Thin Go broker under `/auth/v1` remains for Google/Apple/`dev`/customer `oidc`. **No** Keycloak embedded. **No** email OTP / magic-link inbox.

### 3. Email is required; AuthN keys

- `users.email` is **NOT NULL** for every principal.
- Social/OIDC AuthN key remains `identity_links (provider, issuer, subject)`.
- Password AuthN uses `principal_credentials` with `credential_kind=password` (bcrypt).
- Uniqueness: `UNIQUE (lower(email))`.

### 4. External IdPs

| Adapter | Role | Default? |
|---|---|---|
| Customer OIDC (DB install auth + `/token/exchange` / authorize `provider=oidc`) | Customer Okta / Entra / Keycloak | **Yes** when configured |
| Password grant | Claimed admin (and other humans with password creds) | When `passwordLoginEnabled` |
| Google / Apple (social broker) | Optional additional providers | **No** — customer must enable |
| Cognito (`IDENTITY_SYNC=cognito`) | Optional AWS write-through | **No** |
| Slack | Later (BP-013 P3) | — |

### 5. Machines stay One-native

`service` / `agent` use Majesta One `client_credentials`. Claim/password/social do not replace machine AuthN.

### 6. JIT provisioning

Customer toggle `jitProvisionUsers` on install auth (default **off**). Shared by social callback and OIDC exchange, with `jitDefaultRole` and `allowedEmailDomains`. Env `AUTH_AUTO_PROVISION_*` remains a lab fallback when SSO is not DB-configured.

### Multi-environment principals

Unchanged: each install has its own DB; do not Deploy-promote users. Correlate via SCIM `externalId` / `userName` per install.

## Consequences

- Product default human path is **claim → password → configure SSO**, not Google-first.
- Security review includes claim-token handling, password hashing, OAuth redirect URIs, PKCE, and JIT abuse.
- Bootstrap `API_KEYS` remain break-glass.

## Non-goals

- Embedded Keycloak / Dex / Authentik in the product binary
- Email OTP / magic-link as login
- Trusting IdP groups as Majesta One Roles or permission sets
- Making Cognito or Google/Apple required on any GA checklist
- Forcing Control IDE for day-0 setup

## Related

- [ADR-006](./006-jwt-auth.md) — Majesta One JWT + principals
- [install-claim-sso-build-plan.md](../architecture/install-claim-sso-build-plan.md)
- [BP-037](../../backlog/BP-037-install-claim-customer-sso.md)
- [idp-agnostic-login-build-plan.md](../architecture/idp-agnostic-login-build-plan.md)
