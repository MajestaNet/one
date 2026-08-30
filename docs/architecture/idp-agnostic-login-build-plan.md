# IdP-agnostic login — build plan

Executable plan for product AuthN independence from Cognito: **Majesta One JWT stays the API bearer**; human login ships as a **thin Go social broker** (Google + Apple first); other IdPs are **documented exchange adapters**.

**ADR:** [ADR-015](../adr/015-idp-agnostic-social-login.md) (amends [ADR-006](../adr/006-jwt-auth.md))  
**Backlog:** [BP-013](../../backlog/BP-013-jwt-unified-principals.md) · [BP-011](../../backlog/BP-011-container-marketplace-fargate.md) · [BP-022](../../backlog/BP-022-client-access-ide-device.md) · [BP-017](../../backlog/BP-017-identity-directory-productionization.md)  
**Playbooks:** [agent-authz.md](./agent-authz.md), [agent-api-families.md](./agent-api-families.md), [agent-control-ide.md](./agent-control-ide.md), [agent-deploy.md](./agent-deploy.md)  
**Domain agents:** `authz-security` (primary), `api-families`, `control-ide`, `deploy-ops`

---

## Locked product decisions

| Topic | Choice |
|---|---|
| API bearer | Majesta One JWT only (`AUTH_JWT_*`, `/auth/v1`) |
| AuthZ SoR | Postgres Roles + permission sets — never IdP groups |
| OOTB human login | Thin Go **social broker** in `cmd/api` (`internal/authlogin`) |
| v1 social providers | **Google + Apple only** |
| Local passwords / email OTP | **Out of scope** for v1 |
| Email required for social users? | **Yes** — `users.email` NOT NULL; AuthN key still `identity_links.sub` |
| Full IdP in-process (Fosite/Hydra) | **No** — broker + Majesta One JWT mint only |
| Keycloak / Dex embedded | **No** — optional customer-run IdP via OIDC exchange docs only |
| Cognito | Optional AWS adapter (`IDENTITY_SYNC=cognito`); **not** product default |
| Customer Okta / Entra / Keycloak / Cognito Hosted UI | Documented **OIDC → `/auth/v1/token/exchange`** adapters |
| Machines (`service` / `agent`) | Majesta One `client_credentials` unchanged |
| Auto-provision on first social login | Configurable; default **off** in production values |

### Why Google/Apple-first (email required from IdP)

Control IDE and self-host installs need a human path that does not force Cognito or an operator-run mailbox. Google and Apple Sign In prove identity via `sub`; email is **required profile data** captured on first provision (including Apple private relay). Later Apple tokens may omit email — Majesta One keeps the stored value and matches on `sub`.

Enterprise SCIM create already requires email — directory provisioning ([BP-017](../../backlog/BP-017-identity-directory-productionization.md)) and social login share that invariant.

---

## Target architecture

```text
Control IDE / browser
        │  PKCE authorize
        ▼
┌───────────────────────────────┐
│  Majesta One /auth/v1             │
│  authorize → Google | Apple   │
│  callback → verify ID token   │
│  identity_links → users       │
│  mint Majesta One JWT (azp)       │
└───────────────────────────────┘
        │
        ▼
  /client /metadata /deploy /ops   ← Bearer = Majesta One JWT only
        │
        ▼
  AuthZ from Postgres by sub

Adapters (same exchange contract):
  Okta / Entra / Keycloak / Cognito ──ID token──► POST /auth/v1/token/exchange
  Slack (later) ─────────────────────────────────► exchange
  Optional: IDENTITY_SYNC=cognito write-through (AWS example only)
```

### Package ownership

| Concern | Package | Notes |
|---|---|---|
| Social broker | `internal/authlogin` (new) | Provider interface; Google + Apple impls |
| Identity write-through | `internal/identity` | Keep `Backend`; Cognito = one impl; generalize provider string |
| JWT mint/verify | `internal/authz` | Existing `OneSigner` |
| HTTP | `internal/httpapi/auth_routes.go` | Authorize/callback + exchange |
| IDE PKCE | `tools/control-ide` | Majesta One authorize URL; drop Cognito-domain fields |
| Deploy docs | `deploy/docker-compose.yml`, Helm values, optional ECS | Social client IDs as secrets; Cognito TF demoted |

---

## Current gaps (inventory)

| Area | Today | Need |
|---|---|---|
| Default login story | ADR-006 Cognito pool + Hosted UI | ADR-015 social broker; Cognito optional |
| `identity.Backend` | Cognito / memory / off | Keep; stop hardcoding `ProviderCognito` at call sites |
| `identity_links.provider` | Effectively `"cognito"` | `google`, `apple`, `oidc`, `cognito`, `slack`, … |
| Exchange | Cognito-shaped OIDC verify | Provider-agnostic OIDC ID-token verify + link lookup |
| `users.email` | `NOT NULL UNIQUE` (case-insensitive) | Required profile; social AuthN key = `identity_links` |
| Integrations columns | `cognito_app_client_id`, `cognito_secret_enc` | Rename/alias to IdP-agnostic external client fields (compat period OK) |
| Control IDE Connect | Cognito domain + client id | Majesta One `/auth/v1/authorize?provider=` + PKCE |
| Compose / Helm | JWT + optional `OIDC_*` | Document `AUTH_LOGIN_PROVIDERS` + Google/Apple secrets |
| Docs | Cognito-centric managed-channel leftovers | Adapter runbooks; GA checklist without Cognito |

---

## Config surface (illustrative — finalize in Phase 1)

| Env | Purpose |
|---|---|
| `AUTH_LOGIN_PROVIDERS` | Comma list: `google`, `apple` (empty = social broker disabled) |
| `AUTH_GOOGLE_CLIENT_ID` / `AUTH_GOOGLE_CLIENT_SECRET` | Google OAuth web client |
| `AUTH_APPLE_CLIENT_ID` / `AUTH_APPLE_TEAM_ID` / `AUTH_APPLE_KEY_ID` / `AUTH_APPLE_PRIVATE_KEY` | Sign in with Apple |
| `AUTH_AUTO_PROVISION_USERS` | `0` (default prod) / `1` (dev) |
| `AUTH_AUTO_PROVISION_ROLE` | Default `StandardUser` (used for humans **after** the install’s first human; the first human gets `SystemAdmin` + managed **System Admin** permission set) |
| `AUTH_LOGIN_ALLOWED_EMAIL_DOMAINS` | Optional allowlist when email present |
| `OIDC_*` | Generic customer IdP verify for `/token/exchange` (unchanged role) |
| `IDENTITY_SYNC` | `off` \| `memory` \| `cognito` — write-through only; default `off` |

Redirect URIs: `{PLATFORM_PUBLIC_URL}/auth/v1/callback/{provider}` plus Control IDE deep-link / loopback as Connected App callbacks (`one.controlIde`).

---

## Phases

Execute in order. Each phase is mergeable and test-gated.

### Phase 0 — Docs & contracts (this change set)

**Owner:** architecture / product  
**Agents:** docs only  
**Status:** In progress (this PR)

**Deliverables**

1. ADR-015 accepted
2. This build plan
3. ADR-006 amended (Cognito no longer default identity backend)
4. BP-013 / BP-011 / tech-stack / authz playbook / architecture index retargeted

**Exit criteria:** Agents reading AuthN docs pick social broker + exchange adapters, not Cognito-as-default.

---

### Phase 1 — Schema + provider-agnostic links — **shipped**

**Packages:** `migrations/`, `internal/db`, `internal/identity`, `internal/httpapi` (principals)  
**Agents:** `authz-security`, `db-backend-perf` (schema only)  
**Playbook:** [agent-authz.md](./agent-authz.md)

**Deliverables**

1. Migration: `users.email` nullable then re-required (`0027` → `0028`); unique on `lower(email)`
2. App invariant: every principal has email; social AuthN key = `identity_links`
3. Stop hardcoding `identity.ProviderCognito` in principal/integration upserts — use backend `Mode()` / explicit provider arg
4. Soft-rename integration secret columns (or dual-write getters): prefer `external_app_client_id` / `idp_secret_enc` with Cognito names as DB aliases until a later cleanup migration
5. Tests: social create requires email; reject emailless auto-provision (`LOGIN_EMAIL_REQUIRED`); email uniqueness

**Exit criteria:** `go test` on db/identity/principals; existing Cognito sync tests still pass with provider `"cognito"`.

---

### Phase 2 — Generic OIDC exchange (adapters foundation) — **shipped**

**Packages:** `internal/authz` (`oidc.go`), `internal/httpapi/auth_routes.go`  
**Agents:** `authz-security`, `api-families`

**Deliverables**

1. `/auth/v1/token/exchange` resolves `identity_links` by verified `(provider, issuer, sub)` — not Cognito-only
2. Map issuer → provider label (`oidc`, `cognito`, or configured name); keep Cognito issuers working
3. Strip AuthZ meaning from `cognito:groups` on any residual transitional OIDC Accept path (scopes from Roles only when Majesta One JWT minted)
4. Docs stub: “Connect Okta / Entra / Keycloak / Cognito Hosted UI” → exchange (full runbooks in Phase 6)

**Exit criteria:** Unit/integration: foreign issuer rejected; Google-shaped and Cognito-shaped ID tokens both exchange when linked; auto-provision respects knobs.

---

### Phase 3 — Thin Go social broker (Google + Apple) — **shipped**

**Packages:** `internal/authlogin` (new), `internal/httpapi/auth_routes.go`, `internal/config`, Connected App seed  
**Agents:** `authz-security`, `api-families`  
**Stack note:** add `golang.org/x/oauth2` + OIDC/JWKS verify; update [tech-stack.md](../tech-stack.md) in the same PR as the dependency

**Deliverables**

1. `LoginProvider` interface: `Name()`, `AuthCodeURL(state,pkce)`, `Exchange(ctx, code, verifier) → SubjectClaims`
2. Google + Apple implementations (ID token verify via provider JWKS; audience = Majesta One client id)
3. Routes:
   - `GET /auth/v1/authorize?provider=google|apple&client_id=…&redirect_uri=…&code_challenge=…&state=…`
   - `GET /auth/v1/callback/{provider}` — verify, link/provision, redirect to client with **authorization code** or fragment handoff that Control IDE exchanges for Majesta One JWT (prefer auth-code to Majesta One token endpoint extension, or reuse exchange with one-time Majesta One login code)
4. Preferred mint path: broker issues short-lived Majesta One JWT only after PKCE completes for registered Connected App (`azp=one.controlIde`); do not expose raw Google/Apple tokens to family APIs
5. SubjectClaims: `provider`, `issuer`, `sub`, `email` (required on first provision), `email_verified`, `name`
6. Provisioning: find by `identity_links`; else if auto-provision **and email present** → insert `users` + role + link; **first human** on the install → `SystemAdmin` + managed **System Admin** (`Admin`) permission set; later humans → `AUTH_AUTO_PROVISION_ROLE` (default `StandardUser`); missing email → `LOGIN_EMAIL_REQUIRED`; else `403` / `LOGIN_NOT_PROVISIONED`
7. Allowlist: if `AUTH_LOGIN_ALLOWED_EMAIL_DOMAINS` set and email present, enforce
8. Tests: PKCE happy path with email; Apple first login with email/relay; Apple return without email claim but stored email; emailless auto-provision rejected; replay/state mismatch; disabled provider

**Exit criteria:** Compose with fake/test provider or recorded JWKS fixtures mints Majesta One JWT; `make test` green; no Cognito required.

**Handoff token design (lock in implementation PR):** Prefer:

```text
IDE PKCE → Majesta One authorize → IdP → Majesta One callback
  → Majesta One one-time auth code → IDE POST /auth/v1/token (grant_type=authorization_code)
  → Majesta One JWT
```

Extend `/auth/v1/token` with `authorization_code` + PKCE verifier for public Connected Apps. Keep `token/exchange` for **external** IdP ID tokens (customer adapters).

---

### Phase 4 — Control IDE Connect panel — **shipped**

**Packages:** `tools/control-ide/**` only  
**Agents:** `control-ide`  
**Playbook:** [agent-control-ide.md](./agent-control-ide.md)  
**Backlog:** [BP-022](../../backlog/BP-022-client-access-ide-device.md)

**Deliverables**

1. Replace Cognito domain / client fields with: API base URL, Connected App client id (`one.controlIde`), provider buttons (Google / Apple)
2. PKCE against Majesta One `/auth/v1/authorize` (not `*.amazoncognito.com`)
3. Finish login via Majesta One `/auth/v1/token` authorization_code grant
4. Keep paste-JWT and client-credentials as break-glass
5. Tests: oauthPkce + ConnectPanel updated; no Cognito URL assumptions in happy path

**Exit criteria:** `make test-ide`; manual Connect login against local API with social stubs.

---

### Phase 5 — Packaging defaults (Compose / Helm / ECS demotion) — **shipped**

**Packages:** `deploy/docker-compose.yml`, `deploy/helm/one/`, `sdk/aws/` (community), `.env.example`  
**Agents:** `deploy-ops`

**Deliverables**

1. Compose/Helm values examples for Google/Apple secrets + `AUTH_LOGIN_PROVIDERS`
2. Default `IDENTITY_SYNC=off`; document Cognito sync as optional AWS-only
3. ECS README: Cognito TF = optional example, not GA checklist item
4. Remove Cognito from any “required for GA” language left in packaging docs

**Exit criteria:** Fresh Compose up documents social login + Majesta One JWT without Cognito; Helm values have commented social knobs.

---

### Phase 6 — Adapter runbooks (other IdPs) — **shipped** (`docs/auth-adapters.md`)

**Packages:** docs only (`docs/auth-adapters.md` or under `docs/architecture/`)  
**Agents:** docs / `authz-security`

**Deliverables** — operator guides, each ending in Majesta One JWT:

| Adapter | Steps (summary) |
|---|---|
| Okta | OIDC app → `OIDC_ISSUER`/`AUDIENCE` → user link or auto-provision → IDE or scripted `/token/exchange` |
| Microsoft Entra | Same OIDC exchange pattern |
| Keycloak (customer-run) | Realm client → exchange; **not** One-operated |
| Cognito Hosted UI | Optional; exchange or legacy sync; not required |
| Slack | Remains BP-013 P3 remainder (bot identity → exchange) |

**Exit criteria:** Linked from ADR-015, BP-013, security.md; no implication that Majesta One ships Keycloak.

---

### Phase 7 — Hardening & cleanup (follow-on)

**Deliverables**

1. Drop dual Cognito column names after one release window (if aliased)
2. Rate-limit authorize/callback; audit `auth.login` / `auth.provision` events
3. Optional: GitHub / Microsoft personal social providers (new `LoginProvider` impls — no ADR if interface stable)
4. Deep-link `one-control://oauth/callback` auto-complete (BP-022 remainder)

---

## Explicit non-goals

- Embedding Keycloak, Dex, Authentik, or Cognito inside product images
- Password, magic-link, or SMS OTP login in this plan’s v1
- IdP groups → Roles / permission sets
- ALB / Ingress IdP authenticate for machine API traffic
- Creating social users without email
- Multi-tenant shared login pool (ADR-001: one install = one directory)
- Deploy-promoting principals between environments (use per-install SCIM)

---

## Test matrix (minimum)

| Case | Expect |
|---|---|
| Google PKCE happy path | Majesta One JWT; `azp=one.controlIde`; link `provider=google`; email stored |
| Apple PKCE, no email on **first** provision | `LOGIN_EMAIL_REQUIRED` — no user row |
| Apple return, email already stored, claim omitted | Login OK; AuthN by `sub` |
| Apple private relay email | Email stored; unique |
| Auto-provision off, unknown sub | Login rejected |
| Domain allowlist miss | Rejected when email present |
| Exchange Okta-shaped token with link | Majesta One JWT |
| Exchange foreign issuer | 401 |
| Family route with Google access token | 401 (must be Majesta One JWT) |
| `IDENTITY_SYNC=off` principal create | No Cognito calls |
| Machine client_credentials | Unchanged |

---

## Alignment checklist

- [x] ADR-015 + this plan merged (Phase 0)
- [x] Phase 1 schema email required (`0027` nullable interim → `0028` NOT NULL)
- [x] Phase 2 generic exchange
- [x] Phase 3 Google/Apple broker + auth code grant
- [x] Phase 4 Control IDE
- [x] Phase 5 packaging
- [x] Phase 6 adapter runbooks
- [ ] BP-013 marked mitigated for Cognito-default removal when Phases 1–5 land
- [x] tech-stack Auth row matches shipped libs

---

## Related

- [ADR-015](../adr/015-idp-agnostic-social-login.md)
- [ADR-006](../adr/006-jwt-auth.md)
- [customization-authz.md](./customization-authz.md)
- [identity-directory-productionization.md](./identity-directory-productionization.md)
- [scim-provisioning.md](./scim-provisioning.md)
- [security.md](../security.md)
