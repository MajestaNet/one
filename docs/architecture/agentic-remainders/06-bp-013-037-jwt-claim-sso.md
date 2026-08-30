# JWT principals + install claim / customer SSO — remainder tech design + agentic build plan

**Work-order slot:** 6 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-013](../../../backlog/BP-013-jwt-unified-principals.md) · [BP-037](../../../backlog/BP-037-install-claim-customer-sso.md)
**Track:** Finish
**Status of remainder:** Partial
**Domain agents:** `authz-security` (primary) / `api-families` (HTTP wiring) — `control-ide` only if BP-065 azp lockstep is in the same change set
**Playbooks:** [agent-authz.md](../agent-authz.md) · [agent-api-families.md](../agent-api-families.md) · [agent-routing.md](../agent-routing.md)
**Existing plans (do not duplicate):** [idp-agnostic-login-build-plan.md](../idp-agnostic-login-build-plan.md) (Phases 0–6 shipped; Phase 7 follow-on) · [install-claim-sso-build-plan.md](../install-claim-sso-build-plan.md) (Phases 0–4 shipped) · [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) ([BP-063](../../../backlog/BP-063-refresh-token-sessions.md) — **dependency, out of this remainder**) · [ide-backend-coupling-review.md](../ide-backend-coupling-review.md) ([BP-065](../../../backlog/BP-065-ide-backend-coupling.md) — claim `azp`; **cross-link, do not duplicate**) · [auth-adapters.md](../../auth-adapters.md) · [ADR-006](../../adr/006-jwt-auth.md) · [ADR-015](../../adr/015-idp-agnostic-social-login.md) · [ADR-030](../../adr/030-install-agent-runtime.md)

---

## 1. Remainder inventory

Honest shipped vs open. Do **not** re-plan Phases 0–6 of the IdP-agnostic plan, or Phases 0–4 of the install-claim plan.

| Surface | Shipped (cite packages/tests) | Still open | Evidence |
|---|---|---|---|
| Unified principals `user` \| `service` \| `agent` | Kernel `principal_type`, `user_roles`, `role_api_scopes`, `principal_credentials`, `identity_links` | None (P0) | `migrations/0008_auth_principals.sql` + follow-ons; `internal/authz/users.go`; `internal/db/users.go` |
| Majesta One JWT Token Service | HS256 mint/verify; `iss`/`sub`/`aud=one`/`exp`/`iat`/`principal_type`/`scopes`/`roles`/`admin`/`azp`; strict alg + required claims | None for the access JWT contract | `internal/authz/jwt.go`, `jwt_test.go` (`TestOneJWTRejectsMissingRequiredIdentityClaims`) |
| `/auth/v1/token` grants | `client_credentials`, `authorization_code`+PKCE, `password`, `refresh_token` | Refresh remainder is **BP-063** (not this doc) | `internal/httpapi/auth_routes.go`, `auth_refresh.go` |
| `/auth/v1/token/exchange` | OIDC/Slack ID token → Majesta One JWT via issuer-scoped `identity_links`; missing `token_use` accepted, `access` rejected; discovery `jwks_uri`; request-local verifier/JIT policy | **One** configured customer issuer only; private-network OIDC egress policy remains | `auth_routes.go` `exchangeOIDCSubject`; `internal/authz/oidc.go` `VerifyIDToken`; `auth_token_exchange_integration_test.go` |
| Identity binding | Rebind of `(provider, issuer, subject)` rejected; new OIDC `sub` cannot attach by email match | Keep/regression only | `internal/db/identity_links.go` `Upsert`; `internal/db/users.go` `EnsureOIDCUser` (“Email is profile data, never an authentication-link key”); `migrate_integration_test.go` |
| Google / Apple social broker | `internal/authlogin` + `GET /auth/v1/authorize` / `callback/{provider}` / login page buttons when enabled | Optional GitHub/Microsoft personal social (IdP plan Phase 7 — **skip** unless a later BP asks) | `internal/authlogin/broker.go`; `auth_login_routes.go`; `auth_login_routes_test.go` |
| Okta / Entra / Keycloak / Cognito Hosted UI **docs** | Operator runbooks ending at `/token/exchange` | Runtime exchange still Cognito-shaped (`token_use`, guessed JWKS) — remainder below | `docs/auth-adapters.md` |
| Slack identity → exchange | **Shipped.** Slack OpenID ID tokens use verified email and issuer-scoped identity links; bot tokens rejected | Keep/regression only | `internal/identity/backend.go`; `internal/httpapi/auth_routes.go`; `auth_routes_test.go` |
| API Role hierarchy `roles.parent_role_id` | Column exists since kernel; **never read** in Go. Record visibility hierarchy is `data_roles` (ADR-016, shipped) | **Do not implement Salesforce-style sharing on this column.** Keep/regression: AuthZ must not walk it | `migrations/0000_kernel.sql`; `internal/authz/sharing.go` uses data-role hierarchy only; no `ParentRole` in Go |
| Object/field/system AuthZ | Shipped with BP-003 | Out of this remainder | `internal/authz/object_perms.go`, `field_perms.go`, `system_perms.go` |
| Customer OIDC client secrets at rest | **Encrypted.** `PUT /metadata/v1/install/auth` writes `enc:v1:…` via `secretcrypt.Encrypt`. GET never returns the secret (`oidcClientSecretSet` bool only). Legacy `plain:` still **readable** for upgraded rows | **Not a product remainder.** Keep/regression: never write new `plain:`; PUT of a `plain:` row re-encrypts | `internal/httpapi/install_auth_routes.go` `protectInstallAuthClientSecret` / `revealInstallAuthClientSecret`; `install_auth_security_test.go`; `docs/security.md`. **BP-037 “encrypt beyond `plain:`” is stale — BP-013 August 2026 note is correct.** |
| Install claim + password | `POST /auth/v1/install/claim`, `GET /auth/v1/install/status`, password grant, bcrypt `credential_kind=password`, rate limit, replay `409 ALREADY_CLAIMED`, outbox `install.claimed` | Browser `format=redirect` still returns JSON (no HTML success / rotate hint). No `one auth claim`. Env `INSTALL_CLAIM_TOKEN` is not operator-rotated after claim (hash already NULLed) | `install_claim_routes.go`; `db.MarkClaimed` sets `claim_token_hash = NULL`; `install_claim_routes_test.go` |
| Hosted login page | `GET /auth/v1/login`: unclaimed → claim form; claimed + SSO + PKCE → **Continue with {IdP}**; password when enabled; Google/Apple/`dev` only if enabled | Missing PKCE query still hints “Open Sign in from Control IDE”. Claim POST does not honor `format=redirect`. No post-claim “unset `INSTALL_CLAIM_TOKEN`” copy | `auth_login_page.go`, `auth_login_page_test.go` (superseded Google-default test skipped) |
| Customer SSO + JIT | `GET\|PUT /metadata/v1/install/auth` (issuer, audience, JWKS URI, client id/secret, display name, JIT, domains, social, password, provisioning) | **One** IdP row on `organization_settings`. No second concurrent customer IdP | `migrations/0034_install_claim_auth.sql`; `internal/db/install_auth.go` |
| OIDC discovery (broker + exchange) | Both paths use provider discovery/JWKS; exact issuer; HTTPS (HTTP loopback only); redirects disabled; 1 MiB document cap | Multi-IdP not modeled. Private-network SSRF/egress policy and provider-specific endpoint-auth metadata remain | `internal/authlogin/oidc_provider.go`; `internal/authz/oidc.go`; OIDC tests |
| Control IDE Connect claim / Govern SSO panel | Shipped as optional JWT client | **Do not plan new Connect chrome** (ADR-030). Claim `azp=one.controlIde` is **BP-065** | `install_claim_routes.go` `actor.Azp = authz.ControlIDEAzp`; [ide-backend-coupling-review.md](../ide-backend-coupling-review.md) |
| `one` CLI human login | `one auth login --token\|--api-key` only | **No claim subcommand** | `cmd/one/auth.go` |
| Refresh tokens | Opaque hashed RTs on interactive human grants | **BP-063** — consume as dependency; do not re-plan | `internal/authz/refresh_token.go`; `migrations/0058_refresh_tokens.sql` |

**Verified 2026-08 (this remainder):** customer OIDC secrets **are** encrypted at rest (`enc:v1:`). The BP-037 remaining bullet “Encrypt OIDC client secrets at rest (beyond `plain:` marker)” is **wrong as a product gap**. Keep a regression prompt, do not re-implement encryption.

---

## 2. Detailed design (remainder only)

### 2.1 Product surfaces (ADR-030)

Claim and SSO **must** work without Control IDE:

1. **Hosted** `/auth/v1/login` (HTML already in `auth_login_page.go`) + JSON Token Service.
2. **CLI** `curl` (exists) and `one auth claim` (remainder) — then `one auth login --token`.
3. Builder MCP / `one` consume the minted Majesta One JWT. Do **not** add Electron Connect chrome, Govern panels, or `one-control://` features here.

Default `azp=one.controlIde` on claim / empty password `client_id` / token-exchange fallback is **BP-065 Phase 1**. This remainder may accept `client_id` on claim (optional query/body) so CLI can mint `azp=one.cli` **without** waiting for BP-065, but must not invent a second azp policy. If BP-065 lands first, claim uses a generic install azp and Control IDE always sends `client_id=one.controlIde`. Cross-link only.

### 2.2 AuthN / AuthZ invariants (unchanged)

Cite [ADR-006](../../adr/006-jwt-auth.md) / [ADR-015](../../adr/015-idp-agnostic-social-login.md) / [ADR-009](../../adr/009-record-audit-authz-packaging.md):

- Family APIs accept **Majesta One JWT only**. Google / Apple / Slack / Okta access tokens stay 401.
- AuthZ SoR is Postgres Roles (exact family scopes, no substring) + permission sets. **Never** IdP groups.
- AuthN key for federated humans is `identity_links (provider, issuer, subject)`. Email is required profile on first provision (`users.email NOT NULL`); it is **not** a linking key.
- Principals stay `user` \| `service` \| `agent`. Slack/OIDC humans are `user`.
- No `tenant_id` on AuthZ rows (ADR-001).
- No Cognito as GA AuthN default. No embedded Keycloak.

### 2.3 BP-013 adapter remainder

#### Slack identity exchange (P3) — **shipped; regression contract**

**In scope:** Sign in with Slack **OpenID Connect** (user identity) → `identity_links.provider=slack` → Majesta One JWT.

**Out of scope:** Slack bot tokens (`xoxb-*`), Incoming Webhooks, and Events API. Those are connectors ([BP-014](../../../backlog/BP-014-agent-outbound-integrations.md)), not AuthN.

Contract:

1. Add `identity.ProviderSlack = "slack"` (exact string; no substring matching on provider names).
2. New `authlogin` provider **or** reuse `OIDCProvider` with Slack’s issuer `https://slack.com` (workspace issuers if Slack returns a distinct `iss` — store the verified `iss` on the link).
3. Customer enablement: install auth `socialProviders` may include `slack` **or** a dedicated `slackClientId` / secret on the same encrypted envelope. Prefer `socialProviders: ["slack"]` plus env `AUTH_SLACK_CLIENT_ID` / `AUTH_SLACK_CLIENT_SECRET` (lab) and Metadata fields if the customer wants DB-config (mirror Google/Apple: env for broker secrets, DB list for enable).
4. Hosted login: **Continue with Slack** only when enabled — same button pattern as Google, never default, never shown unclaimed.
5. `POST /auth/v1/token/exchange` accepts Slack **OIDC ID tokens** with existing `subject_token_type=urn:ietf:params:oauth:token-type:id_token`. Verify `iss` + `aud` + `exp` + `sub` via Slack JWKS. Do **not** require Cognito `token_use`.
6. JIT: same `jitProvisionUsers` / domain allowlist / `LOGIN_EMAIL_REQUIRED` as other brokers. Slack email must be present and verified on first provision.

Failure modes: unknown workspace issuer → 401; bot token pasted as `subject_token` → 401; disabled provider → 403 / not listed on `/login/providers`.

#### OIDC exchange hardening (shared with BP-037 discovery) — **shipped; private egress + multi-IdP remain**

`OIDCVerifier.VerifyIDToken` now accepts a missing `token_use` and rejects a present non-`id` value, matching Entra / Okta / Keycloak as well as Cognito.

Remainder contract:

| Check | Rule |
|---|---|
| Signature | RS256 / ES256 only (already; keep HMAC reject tests) |
| `iss` | Exact match to a **configured** provider issuer (see multi-IdP) |
| `aud` | Exact match to configured audience **or** client id (broker already tries both) |
| `exp` | Required (already) |
| `sub` | Required (already) |
| `token_use` | If **present**, must be `id` (reject `access`). If **absent**, accept for generic OIDC |
| JWKS | From configured `oidcJwksUri`, else from discovery `jwks_uri`, **never** blindly `${issuer}/.well-known/jwks.json` when discovery is available |
| Discovery | HTTPS (HTTP loopback only); issuer in the document must match configured issuer (already in broker); timeout + 1 MiB body cap (broker has size cap; copy to verifier) |

Do not map `groups` / `cognito:groups` to Roles (already ignored on mint path). Keep `resolveScopesFromOIDC` as no-DB transitional only.

#### API Role `parent_role_id` — Keep, do not build a second hierarchy

Sharing already walks **data-role** hierarchy ([ADR-016](../../adr/016-record-sharing.md)). Walking `roles.parent_role_id` for record visibility is forbidden. Walking it for **API scope inheritance** would silently widen family scopes and is not requested by GA.

Remainder: regression tests that `ListRoleGrants` / sharing evaluators never `JOIN roles.parent_role_id`. Leave the column (forward-compatible kernel). Do not add Role CRUD for parent assignment in this work-order. Document the column as unused in `docs/data-model.md` only if that file already discusses `roles` — otherwise a one-line note in the AuthZ playbook is enough when the implementing agent lands tests.

#### IdP plan Phase 7 (slice into A, skip the rest)

**Do:** rate-limit `GET /auth/v1/authorize` and `GET /auth/v1/callback/{provider}` with the existing `authTokenLimiter` (or a sibling limiter); audit `auth.login` / `auth.provision` (callback already has provision paths — emit consistent event names).

**Skip:** GitHub / Microsoft personal social; Control IDE `one-control://` auto-complete (BP-022 / frozen chrome); dual Cognito column rename (no longer a product-binary Cognito column set).

### 2.4 BP-037 claim / SSO remainder

#### Encrypt at rest — Keep / regression (already shipped)

`protectInstallAuthClientSecret` → `secretcrypt.Encrypt` → `enc:v1:`. `revealInstallAuthClientSecret` still reads `plain:` for pre-encryption rows; a subsequent PUT with a new secret (or same secret) rewrites encrypted.

Regression must keep:

- New PUT never stores `plain:` when `s.encKey()` is set.
- GET never returns the secret.
- Empty encryption key in **production** already fails config (`AUTH_JWT_SIGNING_KEY` required) — do not add a second key type.
- Optional small hardening (allowed in prompt B): if a GET/PUT sees a `plain:` value, rewrite encrypted in place (lazy migrate) and audit `install.auth.secret_upgraded`. Do **not** log the secret.

#### Multi-IdP

Today: one issuer on `organization_settings` (`oidc_issuer`, `oidc_audience`, `oidc_jwks_uri`, `oidc_client_id`, `oidc_client_secret_enc`, `oidc_display_name`). Login broker `RegisterOrReplace`s a single `provider=oidc`. Exchange uses `effectiveOIDCVerifier` (DB row else env `OIDC_*`).

Remainder schema (new numbered migration):

```text
install_identity_providers (
  id uuid PK,
  api_name text UNIQUE,          -- e.g. okta.prod, entra.corp
  kind text CHECK (kind IN ('oidc')),
  issuer text NOT NULL,
  audience text NOT NULL,
  jwks_uri text,
  authorization_endpoint text,   -- optional override
  token_endpoint text,
  client_id text,
  client_secret_enc text,        -- enc:v1 only when set
  display_name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  sort_order int NOT NULL DEFAULT 0,
  created_at / updated_at
)
```

- Unique `(issuer)` among **enabled** rows (two IdPs cannot share `iss` — exchange dispatch is by `iss`).
- Backfill: if `organization_settings.oidc_issuer` is set, insert `api_name=oidc` (or `customer.sso`) from those columns. Keep the columns as a **read fallback** for one release, then Metadata GET can project `identityProviders[]` plus deprecated flat fields.
- `PUT /metadata/v1/install/auth` accepts `identityProviders` array (create/patch/disable). Flat `oidcIssuer` etc. still update the primary (`sort_order=0`) row for compat.
- Hosted login: one **Continue with {displayName}** button per enabled OIDC row, PKCE `provider=oidc` plus `idp={api_name}` (or `provider={api_name}` if the broker registers each apiName). Exact provider match on authorize.
- Exchange: decode `iss` from the unverified payload **only to select** the provider, then verify with that provider’s JWKS. Unknown `iss` → 401. Do not try every JWKS.
- Env `OIDC_*` remains lab fallback when the table is empty.

JIT / domain allowlist stay **install-global** (not per-IdP) in this remainder. Per-IdP JIT is a later BP if needed.

#### Discovery edge cases

Apply the same `validateOIDCEndpoint` + issuer-mismatch rules used by the broker to the **exchange** verifier. Additional remainder:

- Discovery HTTP client: no proxy, no redirects (already `ErrUseLastResponse`), 15s timeout, 1 MiB body.
- Reject discovery URLs with userinfo (`user:pass@`), non-HTTPS (except loopback).
- If discovery `issuer` has a trailing-slash mismatch only, normalize before compare (broker already `TrimRight`).
- Cache discovery + JWKS per issuer in-process (broker `ready` flag); invalidate on Metadata PUT.
- Document Entra: operators should set `oidcJwksUri` to `https://login.microsoftonline.com/<tid>/discovery/v2.0/keys` **or** rely on discovery — never the guessed `/.well-known/jwks.json`.

#### Claim token rotation UX (hash already burned)

`MarkClaimed` already sets `claim_token_hash = NULL`. Replay is `409`. The leftover is **operator UX**, not crypto.

1. **JSON** claim response: add `next: { "removeEnv": "INSTALL_CLAIM_TOKEN", "hint": "Claim token hash cleared; remove the env/secret so it cannot be reused on a restored unclaimed snapshot." }` — do not print the token.
2. **HTML** `format=redirect` (login form already posts this hidden field; handler ignores it today): after success, `303` to `/auth/v1/login?claimed=1` showing a success page (password/SSO next steps + “remove `INSTALL_CLAIM_TOKEN` from App Platform / Helm secrets”). Do **not** put the access JWT in a query string. Prefer Set-Cookie is **out of scope** (no session cookie AuthN). Browser claim is for day-0 humans who will next configure SSO via Metadata with the JSON path (`curl` / `one auth claim`) or paste JWT into `one auth login`.
3. **`GET /auth/v1/install/status`:** already `{ claimed }`. Add `claimTokenConfigured: false` after claim (hash null). Unclaimed with empty hash → `CLAIM_TOKEN_UNSET` on POST (already).
4. **`one auth claim`:** `one auth claim --base-url URL --claim-token TOKEN --email EMAIL --password PASS` → `POST /auth/v1/install/claim` → persist JWT via existing credential store, print rotate hint to stderr. Allowed package: `cmd/one` (small; [BP-048](../../../backlog/BP-048-one-cli.md) owns CLI shape — keep flags consistent with `one auth login`).
5. Docs: `docs/self-host.md` / `docs/builder-connect.md` already show curl; add the rotate-after-claim sentence. Do not add Control IDE copy.

Optional `client_id` on claim: if present and a registered Connected App, set `azp` to that apiName; if absent, leave current behavior (today `one.controlIde`) until BP-065 changes the default.

### 2.5 Failure modes (remainder)

| Case | HTTP | Code |
|---|---|---|
| Slack / OIDC unknown `iss` | 401 | `INVALID_TOKEN` |
| `token_use=access` | 401 | `INVALID_TOKEN` |
| Second enabled IdP with duplicate issuer | 400 on PUT | `VALIDATION_ERROR` |
| Discovery issuer mismatch | 401 / authorize 502 mapped to `OIDC_UNAVAILABLE` | do not leak IdP body |
| Claim already done | 409 | `ALREADY_CLAIMED` (keep) |
| HTML claim success | 303 | no token in URL |
| Identity rebind | 409 | keep `ErrConflict` |

### 2.6 Lockstep IDE rules

- **Do not edit** `tools/control-ide` in the BP-013/037 remainder slices.
- If claim `azp` is changed, that is [BP-065](../../../backlog/BP-065-ide-backend-coupling.md) Phase 1 (Go + IDE send `client_id`). Cross-link; do not duplicate the azp design here.
- Do not unfreeze Connect / Govern chrome.

---

## 3. Concrete agentic build plan

### Phase A1 — OIDC exchange hardening (no multi-IdP table yet)

- **Owner:** `authz-security`
- **Packages allowed:** `internal/authz`, `internal/authlogin`, `internal/httpapi` (`auth_routes.go`, `auth_login_routes.go`)
- **Forbidden:** `tools/control-ide`, unrelated httpapi families, `cmd/one`
- **Files likely:** `internal/authz/oidc.go`, `oidc_test.go`; `internal/httpapi/auth_routes.go` (`exchangeOIDCSubject`, `effectiveOIDCVerifier`); reuse discovery helpers from `internal/authlogin/oidc_provider.go` (extract shared `DiscoverOIDC` if that avoids duplication)
- **Tests:** `go test ./internal/authz/... ./internal/authlogin/... ./internal/httpapi/...` — ID token **without** `token_use` accepted; `token_use=access` rejected; JWKS from discovery not `/.well-known/jwks.json` when document differs; HMAC still rejected
- **Exit:** Entra-shaped and Cognito-shaped fixtures both exchange when issuer/audience/JWKS match a **single** configured IdP
- **Depends:** none (BP-063 refresh already mints on exchange)

### Phase A2 — Slack user OpenID adapter

- **Owner:** `authz-security`
- **Packages allowed:** `internal/identity`, `internal/authlogin`, `internal/authz`, `internal/httpapi` (`auth_login_*.go`, `auth_routes.go`), `internal/config`, `docs/auth-adapters.md`
- **Forbidden:** `tools/control-ide`; Slack bot/connector packages
- **Files likely:** `internal/identity/backend.go`; new Slack provider or OIDC config preset; `auth_login_page.go` button; `docs/auth-adapters.md` Slack section
- **Tests:** authorize hidden when not enabled; PKCE happy path with recorded JWKS; exchange Slack ID token with `identity_links.provider=slack`; bot token 401
- **Exit:** `docs/auth-adapters.md` Slack is a runbook, not “later”; `GET /auth/v1/login/providers` lists `slack` only when enabled
- **Depends:** A1 (generic ID token without `token_use`)

### Phase A3 — Role `parent_role_id` regression + authorize rate-limit / audit

- **Owner:** `authz-security`
- **Packages allowed:** `internal/authz`, `internal/db` (tests), `internal/httpapi` (`auth_login_routes.go`)
- **Forbidden:** sharing redesign; Role parent CRUD UI
- **Tests:** sharing + `ListRoleGrants` unit/integration prove no parent-role walk; authorize/callback 429 after limiter; audit rows `auth.login` / `auth.provision`
- **Exit:** BP-013 P2 hierarchy marked **Keep (unused column)**; Phase 7 rate-limit/audit done
- **Depends:** none

### Phase B1 — Encrypt-at-rest Keep / optional lazy upgrade

- **Owner:** `authz-security`
- **Packages allowed:** `internal/httpapi/install_auth_routes.go`, `install_auth_security_test.go`
- **Forbidden:** new crypto scheme
- **Tests:** keep `TestInstallAuthClientSecretEncryptedAndLegacyReadable`; add: PUT with key set never persists `plain:`; optional GET lazy-upgrade
- **Exit:** BP-037 encrypt bullet rewritten to **shipped + regression**
- **Depends:** none (can run first)

### Phase B2 — Multi-IdP + discovery on exchange

- **Owner:** `authz-security` + `db-backend-perf` (migration only)
- **Packages allowed:** `migrations/`, `internal/db/install_auth.go` (+ new store), `internal/httpapi/install_auth_routes.go`, `auth_login_page.go`, `auth_login_routes.go`, `auth_routes.go`, `internal/authlogin`
- **Forbidden:** `tools/control-ide`; per-IdP JIT; cross-install SSO
- **Files likely:** new `migrations/00xx_install_identity_providers.sql` + journal; Metadata GET/PUT `identityProviders`
- **Tests:** two issuers, exchange dispatches by `iss`; duplicate enabled issuer rejected; login page two Continue buttons; backfill from flat columns; `go test ./internal/db/... ./internal/httpapi/... ./internal/authlogin/...`
- **Exit:** operators can enable Okta + Entra on one install; unknown `iss` 401
- **Depends:** A1

### Phase B3 — Claim UX + `one auth claim`

- **Owner:** `authz-security` (HTTP/HTML) then small `cmd/one` (CLI)
- **Packages allowed:** `internal/httpapi/install_claim_routes.go`, `auth_login_page.go`; `cmd/one/auth.go`; `docs/self-host.md`, `docs/builder-connect.md`
- **Forbidden:** `tools/control-ide`; cookies as AuthN; putting JWT on query string
- **Tests:** form `format=redirect` → 303, no token in Location; JSON includes rotate hint; `one auth claim` stores alias (CLI test); replay still 409; hash remains NULL
- **Exit:** day-0 works from hosted page + CLI without Control IDE; operator is told to remove `INSTALL_CLAIM_TOKEN`
- **Depends:** none (parallel with B1)

### Phase order

B1 ∥ A3 ∥ B3 → A1 → A2 and B2 (B2 needs A1). Do not block Slack on multi-IdP.

---

## 4. Explicit non-goals

- Product code in this remainder **docs** PR (design only).
- Re-implementing OIDC secret encryption (already `enc:v1`).
- Refresh-token protocol ([BP-063](../../../backlog/BP-063-refresh-token-sessions.md)).
- Neutralizing `azp=one.controlIde` ([BP-065](../../../backlog/BP-065-ide-backend-coupling.md)).
- New Control IDE Connect / Govern chrome ([ADR-030](../../adr/030-install-agent-runtime.md)).
- Email OTP / magic-link; embedded Keycloak / Dex / Authentik; Cognito as GA default.
- Cross-install SSO; Deploy-promoting users.
- Slack bot tokens as AuthN; IdP groups → Roles / permission sets.
- Implementing `roles.parent_role_id` as sharing or scope inheritance.
- GitHub / Microsoft personal social providers.
- Lengthening access JWT TTL.
- Multi-tenant `tenant_id`.

---

## 5. Agentic implementation prompt(s)

### Prompt A — BP-013 adapter remainder (Slack + OIDC exchange hardening + hierarchy Keep)

```text
You are the Majesta One authz-security agent. Implement the BP-013 adapter remainder only. Do not implement BP-037 multi-IdP or claim HTML UX unless a test requires a tiny hook.

Read first:
- docs/architecture/agent-authz.md
- docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md (§1 inventory, §2.3, Phase A1–A3)
- docs/architecture/idp-agnostic-login-build-plan.md (do not re-do Phases 0–6)
- docs/auth-adapters.md
- docs/adr/006-jwt-auth.md, docs/adr/015-idp-agnostic-social-login.md, docs/adr/016-record-sharing.md
- backlog/BP-013-jwt-unified-principals.md
- Evidence in tree: internal/authz/oidc.go VerifyIDToken (token_use=id), internal/authlogin/oidc_provider.go discovery, internal/identity/backend.go (no slack), roles.parent_role_id unused

Edit scope (allowed):
- internal/authz (oidc verify, tests; ListRoleGrants must not walk parent_role_id)
- internal/authlogin (Slack provider and/or shared DiscoverOIDC used by exchange)
- internal/identity (ProviderSlack constant only)
- internal/httpapi: auth_routes.go (exchange), auth_login_routes.go, auth_login_page.go (Slack button only)
- internal/config (AUTH_SLACK_* lab knobs)
- internal/db tests proving identity_links provider=slack and parent_role_id unused
- docs/auth-adapters.md Slack section; update BP-013 remaining adapter table
- Optional: agent-authz.md one-liner that roles.parent_role_id is unused (sharing = data_roles)

Forbidden:
- tools/control-ide
- migrations for multi-IdP (that is Prompt B)
- Re-encrypt design; claim HTML 303; cmd/one
- Slack bot tokens / connectors
- Walking roles.parent_role_id for scopes or sharing — add regression tests instead
- BP-063 refresh protocol changes; BP-065 azp default changes (you may pass through client_id if already present)

Implement in order:
1. OIDC exchange: missing token_use OK; token_use=access rejected; JWKS from discovery/jwks_uri not guessed path.
2. Slack OpenID LoginProvider + exchange + login button when enabled; identity_links.provider exact "slack".
3. Rate-limit authorize/callback; audit auth.login / auth.provision.
4. Regression: ListRoleGrants + sharing never JOIN roles.parent_role_id.

Tests:
- go test ./internal/authz/... ./internal/authlogin/... ./internal/httpapi/... ./internal/identity/...
- Focused db tests if identity_links Slack rows are asserted
- Do not run make test-ide

Out of scope: Control IDE chrome, install claim rotation UX, encrypt secrets, GitHub/Microsoft social, refresh tokens.

When done: update BP-013 P3 Slack row to shipped (or keep Remaining if Slack env fixtures cannot land); mark P2 parent_role_id as Keep (unused). Do not edit backlog/README.md unless the parent work-order says so.
```

### Prompt B — BP-037 claim / SSO remainder (multi-IdP + discovery + claim UX; encrypt Keep)

```text
You are the Majesta One authz-security agent. Implement the BP-037 remainder only. Encryption at rest already shipped (enc:v1) — do not re-implement crypto; keep regression + optional plain: lazy upgrade.

Read first:
- docs/architecture/agent-authz.md
- docs/architecture/agentic-remainders/06-bp-013-037-jwt-claim-sso.md (§1, §2.4, Phase B1–B3)
- docs/architecture/install-claim-sso-build-plan.md (Phases 0–4 shipped — remainder only)
- docs/adr/015-idp-agnostic-social-login.md, docs/adr/030-install-agent-runtime.md
- backlog/BP-037-install-claim-customer-sso.md
- Evidence: internal/httpapi/install_auth_routes.go protectInstallAuthClientSecret; install_auth_security_test.go; db.MarkClaimed NULLs claim_token_hash; auth_login_page.go posts format=redirect but handleInstallClaim always JSON; cmd/one/auth.go has login/logout only
- BP-065 owns default azp=one.controlIde — do not duplicate; optional client_id on claim is OK

Edit scope (allowed):
- migrations/ + journal (install_identity_providers)
- internal/db/install_auth.go (+ new provider store)
- internal/httpapi: install_auth_routes.go, install_claim_routes.go, auth_login_page.go, auth_login_routes.go, auth_routes.go (exchange dispatch by iss)
- internal/authlogin (per-IdP RegisterOrReplace; discovery on exchange path — share helpers with Prompt A if already merged)
- internal/secretcrypt only if tests need it (no new envelope)
- cmd/one/auth.go: `one auth claim` then reuse persistCredential
- docs/self-host.md, docs/builder-connect.md, docs/auth-adapters.md, BP-037 remaining list
- Tests under internal/httpapi, internal/db, cmd/one

Forbidden:
- tools/control-ide (no new Connect chrome)
- Slack adapter (Prompt A) unless already in tree
- Email OTP, embedded Keycloak, cross-install SSO
- JWT in redirect query string; session cookies as AuthN
- Replacing enc:v1; writing new plain: secrets
- backlog/README.md

Implement in order:
1. Keep/regression for OIDC secret encryption; optional lazy upgrade of plain: rows on GET/PUT.
2. Multi-IdP table + backfill from organization_settings oidc_* ; Metadata identityProviders; login buttons; exchange selects provider by verified-selection iss then JWKS verify (Prompt A token_use rules).
3. Claim UX: format=redirect → 303 /auth/v1/login?claimed=1 (no token in URL); JSON rotate hint; one auth claim; docs tell operators to remove INSTALL_CLAIM_TOKEN.

Tests:
- go test ./internal/httpapi/... ./internal/db/... ./internal/authlogin/... ./cmd/one/...
- Preserve TestInstallClaimAndPasswordGrant replay 409 and encrypted secret test
- Do not run make test-ide unless you touched the IDE (you must not)

Out of scope: Control IDE, BP-013 Slack, BP-063, BP-065 azp neutralization, GitHub social.

When done: rewrite BP-037 Remaining to match honesty (encrypt = shipped/Keep; list multi-IdP / claim UX as shipped or still open). Claim/SSO must work with curl, hosted /auth/v1/login, and one CLI — not Control IDE.
```

**Skip with explanation:** there is no “already done” prompt to drop entirely. Encryption is the only slice that is **Keep/regression** (Prompt B step 1), not a full skip. Role hierarchy is **Keep/regression** inside Prompt A step 4, not a third prompt.
