# Refresh-token sessions — build plan

Executable plan so Control IDE (and other interactive human clients) stay signed in across **days**, not only for the access-JWT TTL (~1 hour). Encrypted on-disk session persistence already restores a stored JWT on Electron relaunch; this plan adds a **refresh-token grant** so a closed-and-reopened app can mint a new access JWT without showing Sign in.

**Status:** Active (Phases 1–3 shipped; Phase 4 session list + Phase 5 OSS kit optional)  
**Prerequisite:** Control IDE encrypted session persist + account chip ([PR #255](https://github.com/MajestaNet/ide/pull/255)) — close/reopen **within** access JWT TTL already stays signed in.  
**ADR:** [ADR-006](../adr/006-jwt-auth.md) (amended: opaque refresh tokens)  
**Backlog:** [BP-063](../../backlog/BP-063-refresh-token-sessions.md) · [BP-013](../../backlog/BP-013-jwt-unified-principals.md) · [BP-022](../../backlog/BP-022-client-access-ide-device.md)  
**Playbooks:** [agent-authz.md](./agent-authz.md) (primary) · [agent-api-families.md](./agent-api-families.md) · [agent-control-ide.md](./agent-control-ide.md) · [agent-data-architecture.md](./agent-data-architecture.md) (kernel SQL only)  
**Domain agents:** `authz-security` (Token Service + store) → `api-families` (`/auth/v1` wiring) → `control-ide` (silent refresh) → `db-backend-perf` only for the kernel table / indexes

Related: [control-ide-security-audit.md](./control-ide-security-audit.md) (CIDE-10 — never persist tokens as cleartext) · [client-experience-security.md](../client-experience-security.md) · [customer-connect.md](../customer-connect.md)

---

## Thesis

> Family APIs keep a **short-lived Majesta One access JWT**. Interactive human logins also receive an **opaque refresh token**. Control IDE stores both in the existing encrypted session file and silently calls `POST /auth/v1/token` (`grant_type=refresh_token`) on boot, before expiry, and after a single `401`. Effective AuthZ is still loaded from Postgres by `sub` on every mint — the refresh token is not an AuthZ document.

Do **not** lengthen `AUTH_JWT_TTL_SECONDS` to days. That would leave a stolen Bearer valid for the whole desktop session window.

```text
Sign in (password | PKCE | SSO exchange | claim)
        │
        ▼
POST /auth/v1/token  →  access_token (JWT, ~1h) + refresh_token (opaque, 30d idle)
        │
        ▼
Control IDE encrypted session.bin (OS keyring or local AES-GCM)
        │
   quit / sleep / reopen
        │
        ▼
POST /auth/v1/token  grant_type=refresh_token
        │  rotate RT, re-load Actor from DB
        ▼
new access JWT  →  family APIs unchanged (Bearer)
```

---

## Why this is the remaining gap

Control IDE now persists the session across Electron restarts (CIDE-10: no cleartext JWT). The access JWT still expires in **3600s** (`AUTH_JWT_TTL_SECONDS`, `OneSigner.TTL`). After that:

- Boot `/client/v1/me` returns `401`
- The user sees Sign in again even though they never signed out

coding agents-class desktop apps keep a **refresh credential** (or equivalent) in OS-backed storage and mint new short-lived access tokens. Majesta One Token Service today returns only `access_token` / `expires_in` / `scope` on every grant (`client_credentials`, `authorization_code`, `password`, token-exchange, install claim). Encrypted `session.bin` restore does not extend that hour.

---

## Locked product decisions

| Topic | Choice | Rationale |
|---|---|---|
| API bearer | Majesta One access JWT only on `/client`, `/metadata`, `/deploy`, `/ops` | ADR-006 unchanged |
| Refresh format | **Opaque** (crypto-random). Store **SHA-256** hex in Postgres. Never mint a long-lived refresh JWT | Revocable; theft of DB hashes is not reusable; no AuthZ in the token |
| Who gets a refresh token | Interactive **human** grants: `authorization_code`, `password`, `token_exchange`, `POST /auth/v1/install/claim`. Generic install sessions (`azp=one.install` on password / token_exchange / claim) always receive refresh. **Public** Connected Apps (including `one.controlIde`) receive refresh only when the token request includes `offline_access` | Machines already have `client_secret` / API keys |
| Who does **not** | `client_credentials`, bootstrap `API_KEYS`, Connect “paste JWT”, confidential service principals | A secret *is* the long-lived credential |
| Control IDE | Public app `one.controlIde`; request `offline_access` on PKCE token (and login/authorize). Keep sending `client_id=one.controlIde`. Encrypted session file is the store | Same rule as other public apps (BP-065) |
| Browser Client Experience | Issue refresh **only** if the Connected App is `public` **and** the token request includes scope `offline_access` (default **off**) | Avoid long-lived tokens in SPA `localStorage`; prefer BFF ([client-experience-security.md](../client-experience-security.md)) |
| Rotation | **Every** successful refresh issues a new RT and immediately revokes the presented one | OAuth 2.1; stolen RT has a short window |
| Reuse detection | Presenting a **rotated** RT revokes the **entire family** | Theft signal |
| Access TTL | Keep default **3600s** (`AUTH_JWT_TTL_SECONDS`) | Stolen Bearer window stays one hour |
| Refresh idle TTL | **30 days**, sliding on successful refresh | Multi-day reopen without Sign in |
| Refresh absolute TTL | **90 days** from family creation; then Sign in is required | Caps forever-sessions |
| AuthZ on refresh | Re-load Actor from DB (roles, scopes, freeze, `CanAuthenticate`) | JWT claims are never the SoR |
| Binding | `user_id` + `azp` + `family_id`; optional `device_id` when the IDE has enrolled one | Peer installs stay separate (each install is its own issuer) |
| Family ownership | `/auth/v1` only (`POST /auth/v1/token`, `POST /auth/v1/revoke`) | ADR-004 Auth surface |
| IDE storage | Same encrypted `session.bin` as the access JWT (CIDE-10). Never query string, never `localStorage` | Matches current session crypto |
| Sign out | IDE `setSession(null)` **and** `POST /auth/v1/revoke` for that RT | Disk delete alone leaves a live refresh at the API |
| Password / freeze / deactivate | Revoke **all** refresh families for that `user_id` | Admin kill-switch |
| API revision | Additive JSON fields; **no** `apiRevision` bump | Older IDEs ignore `refresh_token` |
| Cross-install SSO | **Out of scope** | Each install has its own JWT issuer + Postgres ([install-ide-connect-build-plan.md](./install-ide-connect-build-plan.md)) |

### Config (illustrative — finalize in Phase 1)

| Env | Default | Purpose |
|---|---|---|
| `AUTH_JWT_TTL_SECONDS` | `3600` | Access JWT (unchanged) |
| `AUTH_REFRESH_IDLE_SECONDS` | `2592000` (30d) | Sliding idle expiry |
| `AUTH_REFRESH_ABS_SECONDS` | `7776000` (90d) | Family hard cap |
| `AUTH_REFRESH_BYTES` | `32` | Raw token entropy |

Do not add a “refresh JWT signing key.” Hash + compare only.

### Who receives a refresh token (implement this helper; do not improvise)

```text
shouldIssueRefresh(azp, grant, requested_scopes, clientKind):
  if grant in {client_credentials, refresh_token} → false
  if azp == one.install
     AND grant in {password, token_exchange}     → true   # generic install session (claim + empty client_id)
  if clientKind == "public"
     AND requested_scopes contains offline_access
     AND grant in {authorization_code, password, token_exchange}
                                                 → true
  else                                           → false
```

Install claim and password grant with empty `client_id` mint `azp=one.install` → always issue refresh. Control IDE sends `client_id=one.controlIde` and `scope=offline_access` on PKCE so the public-app rule applies. A Client Experience `client_id` on password/PKCE requires `offline_access`. Confidential apps never get refresh.

---

## Code map (today — do not rediscover)

| Piece | Where |
|---|---|
| Token JSON | `internal/httpapi/auth_routes.go` `tokenResponse` (`access_token`, `token_type`, `expires_in`, `scope`) — add optional `refresh_token`, `refresh_expires_in` |
| `POST /auth/v1/token` | `handleAuthToken` switch: `authorization_code` / `password` / `client_credentials`; **add** `refresh_token`. `UNSUPPORTED_GRANT` text must list the new grant |
| Password grant | `internal/httpapi/install_claim_routes.go` `handleAuthPasswordGrant` — already returns `INVALID_GRANT` on bad password (reuse that code + wording for failed refresh) |
| Auth code | `handleAuthAuthorizationCode` in `auth_routes.go` |
| Token exchange | **Separate route** `POST /auth/v1/token/exchange` (`handleAuthTokenExchange`) — not a `grant_type` on `/token`. Issue RT on the JSON response when eligible |
| Install claim | `POST /auth/v1/install/claim` (`handleInstallClaim`) — extra `user` / `claimed` fields; still add `refresh_token` |
| Mint | `OneSigner.MintAccessToken` — unchanged; refresh is a **sibling** credential, not JWT claims |
| Discovery | `handleAuthOIDCDiscovery`: today `grant_types_supported` = `client_credentials`, `authorization_code`, `urn:ietf:params:oauth:grant-type:token-exchange` (password grant exists but is **omitted**). Add `password` and `refresh_token`; add `revocation_endpoint` |
| Rate limit | `s.authTokenLimiter` (`AUTH_TOKEN_RATE_LIMIT_PER_MINUTE`). Refresh key: `refresh:` + hex(SHA-256(raw))[:16] — never the raw token |
| Access TTL | `internal/config` `AUTH_JWT_TTL_SECONDS` default 3600; `OneSigner.TTL` |
| IDE azp | `authz.ControlIDEAzp` = `one.controlIde` (desktop Connected App) |
| Install azp | `authz.InstallAzp` = `one.install` (claim / empty password `client_id` / token-exchange default) |
| Password / freeze / deactivate hooks | `handleChangeMyPassword` (`server.go`), principal set-password, `handleFreezePrincipal`, `DeactivatePrincipal` (Client + SCIM). Call refresh `RevokeAllForUser` |
| IDE session | Encrypted `session.bin` (`tools/control-ide/src/main/sessionStore.ts`, CIDE-10). `EnvConnection` in `session.ts` has `token` + identity fields; **add** `refreshToken`, `accessExpiresAt` |
| IDE HTTP | `api.ts` `apiFetch`; Sign in / Disconnect in `govern/ConnectSection.tsx`; boot hydrate in `App.tsx` |
| Redaction | `tools/control-ide/src/renderer/errors.ts` already matches `refresh_token` |
| Kernel journal | `migrations/meta/_journal.json` last tag `0057_record_search` → **next** `0058_refresh_tokens` (confirm idx at implement time if another kernel migration landed) |

Token encoding (lock): 32 cryptographically random bytes, **base64url without padding**. Store `hex(SHA-256(raw))`. Compare with constant-time equality on the digest. **Do not** bcrypt refresh tokens (too slow for `/token`; this is a lookup key, not a password).

---

## Current gaps (inventory)

| Area | Today | Need |
|---|---|---|
| `POST /auth/v1/token` body | `access_token`, `token_type`, `expires_in`, `scope` | Optional `refresh_token`, `refresh_expires_in` |
| Grants | `client_credentials`, `authorization_code`, `password` on `/token`; exchange is `POST /auth/v1/token/exchange` | `grant_type=refresh_token` on `/token`; issue RT from eligible mint sites including exchange + claim |
| Persistence | Access JWT in encrypted `session.bin` (PR #255) | Persist `refreshToken` + `accessExpiresAt` beside the JWT |
| Revocation | Freeze / password change do not invalidate already-minted JWTs until `exp` | Refresh families revoked immediately; access JWT still dies at `exp` (acceptable) |
| Discovery | Omits `password` and `refresh_token` | Add both; document `revocation_endpoint` |
| Control IDE | Boot `/me`; `401` → Sign in | Silent refresh then retry; Sign out revokes |
| sdk/client `@one/auth` | Planned PKCE / exchange ([ADR-019](../adr/019-client-experience-oss-kits.md)) | Refresh helper **after** IDE path is proven |

---

## Package ownership

| Concern | Package | Notes |
|---|---|---|
| Hash / issue / rotate / reuse-detect | `internal/authz` (new `refresh_token.go`) | Domain logic, not HTTP |
| Postgres | `internal/db` + `migrations/0058_refresh_tokens.sql` | Kernel table; not `records` |
| HTTP | `internal/httpapi/auth_routes.go`, `install_claim_routes.go` | Thin handlers; rate limit already on `/token` |
| Password / freeze hooks | `internal/httpapi` principal + `me/password` | Call `RevokeAllForUser` |
| IDE | `tools/control-ide` `session.ts`, `api.ts`, `ConnectSection`, `App.tsx` | JWT client only |
| Config | `internal/config` | Idle / abs TTL |
| OSS kit (later) | `sdk/client` | Do not block IDE |

Plane fence: AuthZ/API agents do **not** edit `tools/control-ide/**`. IDE agents do **not** edit `internal/` / `migrations/`. Cross-plane PRs cite both playbooks.

---

## Schema (Phase 1)

Kernel table (one install = one customer DB; no SaaS `tenant_id` isolation column):

```sql
CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  family_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  azp text NOT NULL,
  token_hash text NOT NULL,
  device_id text,
  expires_at timestamptz NOT NULL,
  family_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  replaced_by uuid REFERENCES refresh_tokens(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  CONSTRAINT refresh_tokens_hash_uniq UNIQUE (token_hash)
);
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_id_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
```

- **Do not** store the token plaintext or ciphertext that can be decrypted into a usable Bearer.
- `token_hash` = hex(`SHA-256(raw)`). Compare with constant-time equality on the digest; raw token is shown **once** in the JSON response.
- `expires_at` = sliding idle deadline (`now + AUTH_REFRESH_IDLE_SECONDS` on issue and on each rotate). Never extend past `family_expires_at`.
- Rotation = insert a new row (same `family_id`, same `family_expires_at`), set `revoked_at` + `replaced_by` on the old row.
- Reuse = presented row has `revoked_at IS NOT NULL` and `replaced_by IS NOT NULL` → `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`.
- User `DELETE` cascades. **Freeze / password change / deactivate** must call `RevokeAllForUser` (CASCADE is not enough — the row still exists).

Do **not** add `credential_kind=refresh` on `principal_credentials`. That table is bcrypt secrets (`client_secret` / `password` / `bootstrap_api_key`). Refresh tokens are hashed lookup keys with rotation metadata.

---

## HTTP contract (Phase 2)

### Issue (interactive grants)

Successful `authorization_code`, `password`, `token_exchange`, and IDE-bound `install/claim` responses become:

```json
{
  "access_token": "<one jwt>",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "client metadata deploy",
  "refresh_token": "<opaque>",
  "refresh_expires_in": 2592000
}
```

Omit `refresh_token` when the grant is not eligible (table above). Clients that ignore unknown fields keep working.

### Refresh

`POST /auth/v1/token` with JSON or form (`tokenRequest` gains `refresh_token`):

| Field | Required | Notes |
|---|---|---|
| `grant_type` | yes | `refresh_token` |
| `refresh_token` | yes | Opaque value from a prior response |
| `client_id` | recommended | Must match stored `azp` when present |

Behavior:

1. Rate-limit with the existing auth token limiter (`refresh:` + hash prefix or client id).
2. Lookup by `token_hash`. Missing / idle-expired / absolute-expired / user `!CanAuthenticate()` → `401 INVALID_GRANT` (same wording as bad password — do not leak which).
3. Rotated-token reuse → revoke family → `401 INVALID_GRANT` + audit `token.refresh.reuse`.
4. `azp` mismatch vs `client_id` → `401 INVALID_GRANT`.
5. Mint access JWT from **current** DB Actor (roles/scopes/admin/freeze).
6. Insert new RT; revoke presented RT; return both tokens.
7. Audit `token.refresh` (user id, azp, family id — **never** the raw token).

### Revoke (RFC 7009-shaped)

`POST /auth/v1/revoke`

```json
{ "token": "<refresh or access>", "token_type_hint": "refresh_token" }
```

- Public endpoint (the RT *is* the capability). Always `200` even if unknown (RFC 7009), except rate-limit.
- Refresh: revoke that row; optionally the whole family when `token_type_hint` is omitted and the token is an RT (lock: **revoke the family** so Sign out kills rotated siblings).
- Access JWT hint: **no-op** for v1 (JWTs are not denylisted; they die at `exp`). Document this honestly.

### Discovery

`GET /auth/v1/.well-known/openid-configuration`:

- `grant_types_supported` adds `refresh_token`
- `revocation_endpoint` = `{issuer}/revoke`

---

## Control IDE (Phase 3)

Keep AuthZ on the install. The IDE is a JWT + refresh client.

| Change | Where |
|---|---|
| Persist `refreshToken`, `accessExpiresAt` on `EnvConnection` (keep `#255` `displayName` / `email`) | `src/renderer/session.ts` |
| Capture `refresh_token` / `expires_in` from every token response | `oauthPkce.ts`, `ConnectSection` password / claim / PKCE / exchange |
| `apiFetch`: on `401`, try **one** refresh then retry; do not loop | `api.ts` |
| Boot: if `accessExpiresAt` is within 60s **or** `/me` is `401`, refresh before showing Sign in | `App.tsx` |
| Sign out / Disconnect: `POST /auth/v1/revoke` then `setSession(null)` | `ConnectSection` |
| Redaction | Already strips `refresh_token` in `errors.ts` — keep tests |
| Storage | Main-process encrypted session file only (already the JWT store) |

Pasted advanced JWT remains access-only (no refresh) — operator reconnects when it expires. That path is break-glass, not the customer-user happy path.

Account chip stays initials + name. Refresh is invisible unless it fails (then Sign in with URL prefilled).

---

## Phases

Execute in order. Each phase is mergeable and test-gated. Do not estimate calendar time.

### Phase 0 — Docs & contracts (this change set)

**Owner:** architecture  
**Agents:** docs only  
**Status:** Shipped (this change set)

**Deliverables**

- This plan
- [ADR-006](../adr/006-jwt-auth.md) amended (opaque refresh; interactive grants only)
- [BP-063](../../backlog/BP-063-refresh-token-sessions.md) + backlog table
- Pointers from playbooks, architecture index, `customer-connect.md`, `security.md`

**Exit criteria:** An implementer can build Phase 1–3 without relitigating TTL, rotation, or who receives refresh tokens.

**PR split (implementation, after this plan merges):** three stacked PRs — kernel store (Phase 1), `/auth/v1` HTTP (Phase 2), Control IDE silent refresh (Phase 3). Do not combine Go + IDE in one PR unless the task is explicitly cross-plane.

### Phase 1 — Store + domain

**Owner:** `authz-security` + kernel SQL (`db-backend-perf` only for the table/index)  
**Packages:** `migrations/`, `internal/db`, `internal/authz`, `internal/config`

**Deliverables**

- `0058_refresh_tokens.sql` + `migrations/meta/_journal.json` (`0058_refresh_tokens`)
- `.env.example` keys for idle / abs TTL (defaults as table above)
- `RefreshTokenStore`: insert, get-by-hash, rotate, revoke-one, revoke-family, revoke-all-for-user
- `authz.IssueRefreshToken` / `RotateRefreshToken` (random bytes, SHA-256)
- Unit tests: hash round-trip, rotation, reuse detection, expiry, freeze user cannot rotate

**Exit criteria:** `go test ./internal/authz/... ./internal/db/...` with `DATABASE_URL`.

### Phase 2 — Token Service HTTP

**Owner:** `authz-security` + `api-families`  
**Packages:** `internal/httpapi/auth_routes.go`, `install_claim_routes.go`, `server.go` (`me/password`), `principal_routes.go`

**Deliverables**

- Shared mint helper: after `MintAccessToken`, optionally attach refresh (do not copy JSON construction five times)
- Issue RT on eligible grants (password, auth code, exchange, install claim)
- `grant_type=refresh_token` on `POST /auth/v1/token`
- `POST /auth/v1/revoke`
- Discovery fields
- Revoke-all on `POST /client/v1/me/password` success, admin set-password, freeze, and any path that sets `!CanAuthenticate()` (including SCIM disable)
- Integration tests via `internal/testutil` (password grant → refresh → `/me` → revoke → refresh fails; reuse kills family)
- Audit events

**Exit criteria:** `go test ./internal/httpapi/...`; `grant_types_supported` includes `refresh_token`.

### Phase 3 — Control IDE silent refresh

**Owner:** `control-ide`  
**Packages:** `tools/control-ide/**` only

**Deliverables**

- Session fields + persist
- Single-flight refresh (two 401s must not double-rotate)
- Boot / 401 / skew paths
- Disconnect revokes
- Vitest: mock token endpoint rotation; 401 then success; refresh failure → auth screen
- No JWT/refresh labels in the account chip

**Exit criteria:** `npm test` under `tools/control-ide`. Manual: sign in, quit Electron, wait past access TTL (or lower `AUTH_JWT_TTL_SECONDS` in dev), reopen → still in launcher, chip still shows the user.

### Phase 4 — Session hygiene (optional, same BP)

**Owner:** `authz-security` then `control-ide`

- `GET /client/v1/me/sessions` — active families (azp, created, last used, current device)
- `POST /client/v1/me/sessions/{familyId}/revoke`
- Settings → Account: “Sign out other devices”
- Requires `scope: client` (self)

Not required for the close/reopen job. Do not block Phase 3 on this.

### Phase 5 — OSS kit (optional, BP-040)

**Owner:** `api-families` / client-experience docs — `sdk/client` `@one/auth` refresh helper, **opt-in** `offline_access`. Browser default remains no RT in `localStorage`.

---

## Test matrix (must stay green)

| Case | Expect |
|---|---|
| Password grant as `one.controlIde` | Access + refresh |
| PKCE `authorization_code` | Access + refresh |
| Token exchange with IDE azp | Access + refresh |
| `client_credentials` | Access **only** |
| Refresh happy path | New access + new refresh; old RT invalid |
| Refresh after idle TTL | `401`; Sign in |
| Refresh after freeze | `401` |
| Refresh after password change | `401` |
| Present rotated RT | Family revoked; `401` |
| Concurrent refresh (two IDE windows) | Single-flight in IDE; server: second call is reuse → family revoke — **IDE must single-flight** so this does not fire on tab duplication |
| Revoke then refresh | `401` |
| Paste JWT Connect | No RT; 401 after access `exp` as today |
| IDE chip | Initials + name, never `refresh_token` |

**Concurrency note:** Desktop should hold a mutex around refresh. Document that two processes sharing one `session.bin` is unsupported (one Electron instance).

---

## Security notes (preserve CIDE-10 / ADR-006)

- Refresh token in logs / UI / query strings is a defect. Keep `redactSensitive`.
- Hashed at rest in Postgres; encrypted at rest in the IDE session file (same ladder as the JWT).
- Rotation + reuse detection is the theft control; idle/abs TTL is the leftover-laptop control.
- Access JWTs remain undenylisted until `exp`. Freeze is immediately effective for **refresh** and for **new** mints; in-flight access JWTs last up to one hour. Acceptable for v1; a JWT denylist is a non-goal (stateful auth at every API hop).
- `clientAccessMode=ide_users` is unsupported (BP-065). Refresh preserves the stored family `azp` (generic `one.install` or a Connected App apiName).

---

## Explicit non-goals

- Lengthening access JWT TTL to days/weeks
- Refresh JWTs or JWT denylist
- Cross-install / multi-env single sign-on (one RT unlocking peer installs)
- Email OTP / magic-link
- Requiring device certs (BP-022) for refresh
- Putting refresh tokens in SPA `localStorage` as the product default
- Issuing refresh tokens to `client_credentials` or API keys
- Embedded Keycloak / Fosite authorization server
- Changing family API `Authorization` to anything but Bearer Majesta One JWT

---

## Agent checklist before merging an implementation PR

- [ ] Read this plan + ADR-006 amend + BP-063
- [ ] Access JWT still default 1h; refresh is opaque + hashed
- [ ] Interactive grants only; `client_credentials` unchanged
- [ ] Rotation + reuse detection covered by tests
- [ ] Password change / freeze revokes families
- [ ] IDE: single-flight refresh; CIDE-10 still forbids cleartext
- [ ] No AuthZ logic copied into the IDE
- [ ] BP-063 / BP-013 status updated when a phase lands
