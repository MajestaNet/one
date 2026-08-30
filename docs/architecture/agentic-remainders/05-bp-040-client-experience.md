# BP-040 Client Experience + OSS kits — remainder tech design + agentic build plan

**Work-order slot:** 5 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-040](../../../backlog/BP-040-client-experience-oss-kits.md)
**Track:** Finish
**Status of remainder:** Partial (Phases 1–6 landed; **R1 kit wire + tests landed**; R2 refresh/revoke/exchange and R3 Experience HTTP tests remain; `@one/react` and partner certification deferred)
**Domain agents:** `api-families` (owner — `sdk/client/` + Experience docs + Metadata experience HTTP tests). `authz-security` only if a slice extends public Connected App defaults (none required for R1). **Not** `control-ide`.
**Playbooks:** [agent-api-families.md](../agent-api-families.md) · [agent-authz.md](../agent-authz.md) · [agent-routing.md](../agent-routing.md) · [module-map.md](../module-map.md) (Client HTTP + `internal/integration`)
**Existing plans (do not duplicate):** [client-experience-build-plan.md](../client-experience-build-plan.md) · [ADR-019](../../adr/019-client-experience-oss-kits.md) · [ADR-030](../../adr/030-install-agent-runtime.md) · [ADR-025](../../adr/025-api-revision-versioning.md) ([ide-api-version-compatibility-build-plan.md](../ide-api-version-compatibility-build-plan.md) — SDK pin checkbox is **overclaimed**) · [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) (BP-063 Phase 5 kit helper) · [client-experience-security.md](../../client-experience-security.md) · [client-experience-telephony.md](../../client-experience-telephony.md)

---

## Verdict (shipped vs remainder)

Phases 0–6 of [client-experience-build-plan.md](../client-experience-build-plan.md) are **in tree**: ADR + security/telephony docs, `@one/auth` / `@one/client` scaffolds, public Connected App `client`-only defaults, Experience metadata pack/apply + Metadata CRUD, list-view sample. Partner certification and `@one/react` stay **deferred**. Control IDE Govern Experiences list **exists and is frozen** — do not add chrome ([ADR-030](../../adr/030-install-agent-runtime.md)).

The Finish remainder is **not** a second SPA host and **not** React hooks. **R1 kit wire is in tree:** `query` sends `{ object, select }`, CRUD uses `/sobjects/…`, both kits pin `One-API-Revision`, and `sdk/client/*/src/*.test.ts` cover the contract. Remaining Finish work is **R2** refresh/revoke/exchange helpers and **R3** Experience HTTP tests + template YAML.

---

## 1. Remainder inventory

Honest mark of this tree (2026-08-23). Do not re-plan shipped phases.

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| ADR-019 + Path A split | Canvas vs Experience; `/auth/v1` + `/client/v1` fence | None | [ADR-019](../../adr/019-client-experience-oss-kits.md); [customer-connect.md](../../customer-connect.md) |
| Security + telephony guides | PKCE / public `client` scope / no Metadata-from-browser; vendor UI in Experience + connectors for REST | Partner certification ladder **deferred** | [client-experience-security.md](../../client-experience-security.md); [client-experience-telephony.md](../../client-experience-telephony.md) |
| `@one/auth` | `generatePKCE`, `buildAuthorizeUrl` (default scope `client`), `exchangeAuthorizationCode`; **`One-API-Revision` on token POST**; scope fence rejects metadata/deploy/ops/admin | No `refreshAccessToken` / `revokeToken` / `exchangeToken` (R2) | `sdk/client/auth/src/index.ts`; `index.test.ts` |
| `@one/client` | `createOneClient`; `CLIENT_PREFERRED_API_REVISION = 1`; header `One-API-Revision`; `query({ object, select })`; CRUD `/sobjects/…`; `describe` / `OneAPIError` / `probeVersion` | R2 unused here; R3 Experience HTTP tests | `sdk/client/client/src/index.ts`; `index.test.ts`; `internal/httpapi/server.go` `/sobjects` + `/query` |
| List-view sample | React + Vite PKCE → `client.query({ object: "Account", select: ["Name"] })` | Stores access JWT in `sessionStorage` (acceptable for a sample; not RT). Never calls `getRecord`. Hard-coded redirect `http://127.0.0.1:5174/oauth/callback` | `sdk/client/examples/list-view/src/App.tsx` |
| Public Connected Apps | Default scopes `["client"]`; reject `metadata`/`deploy`/`ops`/`admin`; public PKCE required; empty roles → `StandardUser` | Keep / regression. `offline_access` is **allowed** on public hints (needed for Experience RT) | `internal/integration/scopes.go`; `service.go` `Create` L286–289; `scopes_test.go` |
| Experience metadata | `metadata_experiences`; pack/validate/apply; `GET\|POST\|PATCH\|DELETE /metadata/v1/experiences` | **No HTTP tests** (`experience_routes.go` untested). Customer-repo template has **no** `metadata/experiences/` sample YAML | `migrations/0041_metadata_experiences.sql`; `internal/httpapi/experience_routes.go`; `internal/customerrepo/experience_pack_test.go`; `internal/customerrepo/pack.go`; `deploy/customer-repo-template/metadata/` |
| IDE Govern Experiences list | `ExperiencesPanel` + `ide.govern.experiences` | **Frozen.** Do not add chrome, edit, or new Govern tiles | `tools/control-ide/src/renderer/panels/ExperiencesPanel.tsx`; ADR-030 |
| `@one/react` | — | **Deferred** (original Phase 2). Do not start until R2/R3 are green | [client-experience-build-plan.md](../client-experience-build-plan.md) Phase 2 |
| Partner certification | — | **Deferred** (Phase 6). No product-useful remainder without a partner program | build plan Phase 6 |
| API revision pin (BP-025) | Header + const `1` on `@one/client` **and** `@one/auth`; `probeVersion`; `OneAPIError` for `API_REVISION_UNSUPPORTED`; package tests | Keep | `sdk/client/client/src/index.ts`; `sdk/client/auth/src/index.ts`; `*.test.ts`; [ide-api-version-compatibility-build-plan.md](../ide-api-version-compatibility-build-plan.md) |
| Refresh (BP-063 Phase 5) | Install issues RT on public PKCE **only** when scope includes `offline_access` (`ShouldIssueRefresh`). IDE azp bypass is BP-065, not this remainder | No `@one/auth` refresh/revoke helper (R2). List-view does not request `offline_access` (correct default) | `internal/authz/refresh_token.go`; [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) Phase 5 |
| CI / npm scripts | `sdk/` excluded from product image (`.dockerignore` `sdk`); kit `package.json` has `test` (`tsc` + `node --test`) | No workflow job for `sdk/client` | `sdk/client/*/package.json`; `.github/workflows` has no `sdk/client` |

**Plan acceptance vs code (do not re-plan shipped rows):**

| Checkbox in client-experience-build-plan | Code verdict |
|---|---|
| Phase 1 security guide | Shipped |
| Phase 2 `@one/auth` + `@one/client` scaffold | Shipped **and R1 wire-correct** (`query`/`sobjects`/pin/tests). Remainder = R2 refresh + R3 Experience HTTP tests |
| Phase 2 `@one/react` | Deferred — keep |
| Phase 3 public Connected App defaults | Shipped + unit tests |
| Phase 4 Experience YAML + Metadata API + IDE list | Pack/API shipped. IDE list frozen. HTTP tests + template YAML open |
| Phase 5 list-view sample | Shipped UI; **does not run against live `/query`** until R1 |
| Phase 6 telephony guide | Shipped. Doc still says `GET /client/v1/records/…` (wrong path). Certification deferred |

---

## 2. Detailed design (remainder only)

Cite [ADR-019](../../adr/019-client-experience-oss-kits.md) (OSS kits + customer-hosted Experiences; `/auth/v1` + `/client/v1` only), [ADR-004](../../adr/004-three-api-families.md) (no Metadata/Deploy from the browser), [ADR-025](../../adr/025-api-revision-versioning.md) (integer pin, header preferred), [ADR-006](../../adr/006-jwt-auth.md) / [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) (opaque RT; public clients need `offline_access`), [ADR-030](../../adr/030-install-agent-runtime.md) (no new Control IDE chrome; no product-hosted SPA).

Do not invent `/client/v1/records`. Do not add a Go `/x` Experience host. Do not unfreeze Govern Experiences.

### 2.1 Locked Client wire for `@one/client`

Match `internal/dataengine` + `internal/httpapi/server.go`. Additive methods only; **do not** bump `CLIENT_PREFERRED_API_REVISION` (still `1`).

| Kit method | HTTP | Body / notes |
|---|---|---|
| `query` | `POST /client/v1/query` | `{ object, select?, filters?, sort?, relationships?, limit?, cursor?, includeDeleted?, mode? }`. `filters` is `{ field, op, value? }[]` (`eq`/`ne`/`gt`/… per `FilterOp`). Response `{ records, totalSize, done, queryPlan?, nextCursor? }` |
| `search` | `POST /client/v1/search` | Keep `{ q, objects?, limit? }` — already correct |
| `getRecord` / `createRecord` / `updateRecord` / `deleteRecord` | `GET\|POST\|PATCH\|DELETE /client/v1/sobjects/{object}[/{id}]` | **Replace** `/records/`. Create returns the created record JSON; delete may be `204` |
| `describe` / `describeObject` | `GET /client/v1/describe`, `GET /client/v1/describe/{object}` | List/object catalog for list-view field labels |
| `me` | `GET /client/v1/me` | Keep |

Do **not** wrap in this remainder: ingest jobs, composite/bulk, principals, integrations, actions, automations, agent runs, run-graphs, conversations, preferences, canvases. Those are other BPs or IDE chrome. Experiences that need `lead.convert` later call `POST /client/v1/actions/{apiName}` in a follow-on — not R1.

**Errors:** parse install JSON `{ error, message, … }` into a small `OneAPIError` (`status`, `code`, `message`, `body`). Map `API_REVISION_UNSUPPORTED` so callers can show the install `cta`. Do not stringify the raw body as the only signal.

**Fetch injection:** `OneClientConfig.fetch` / `OneAuthConfig.fetch` optional; default `globalThis.fetch`. Unit tests pass a mock; no live install required.

**Revision:** every `@one/client` and `@one/auth` family call sends `One-API-Revision: {apiRevision ?? CLIENT_PREFERRED_API_REVISION}`. Optional `probeVersion(baseUrl)` → `GET /version` (no pin; path is revision-agnostic) returning `{ productVersion, apiRevision: { min, current, recommended } }`. Kit does **not** hard-block like `one` exit 3; it sends the pin and surfaces `API_REVISION_UNSUPPORTED`. Constructor may still accept an explicit `apiRevision`.

Shared const: export `PREFERRED_API_REVISION = 1` from `@one/client` and re-export or duplicate the same integer from `@one/auth` so both packages stay publishable independently. Do not import `@one/client` from `@one/auth`.

### 2.2 `@one/auth` remainder

**R1 (must):** send `One-API-Revision` on `POST /auth/v1/token` (and later refresh/exchange/revoke). Reject `buildAuthorizeUrl` scopes that include `metadata` / `deploy` / `ops` / `admin` (throw before redirect). Default scopes stay `["client"]`.

**R2 (BP-063 Phase 5):**

| Helper | Call | Rules |
|---|---|---|
| `refreshAccessToken(config, refreshToken)` | `POST /auth/v1/token` `grant_type=refresh_token` + `client_id` + `refresh_token` | Only useful if the Connected App allowed `offline_access` **and** authorize requested it. Rotate: return new `access_token` + `refresh_token` |
| `revokeToken(config, token, hint?)` | `POST /auth/v1/revoke` | RFC 7009-shaped; treat non-rate-limit as success |
| `exchangeToken(config, subjectToken, …)` | `POST /auth/v1/token/exchange` | For BFF/SSO Experiences; not the list-view default |

Browser default remains **no** RT in `localStorage`. Document two patterns:

1. **Default Experience** — scopes `["client"]`. Access JWT in memory (sample may keep `sessionStorage` for the access token only). Re-login when it expires.
2. **Opt-in durable SPA** — scopes `["client", "offline_access"]`. Hold RT in memory or a **customer BFF** cookie; never `localStorage`. Single-flight refresh (reuse detection revokes the family — [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md)).

Do not copy Control IDE encrypted `session.bin`. Do not lengthen access JWT TTL.

### 2.3 Examples and docs

**List-view (R1):** fix `client.query({ object: "Account", select: ["Name"], limit: 25 })`. Optional: click a row → `getRecord`. Keep PKCE verifier in `sessionStorage` (needed across redirect). Document that `VITE_ONE_CLIENT_ID` is the Connected App `apiName`. Fix [client-experience-telephony.md](../../client-experience-telephony.md) screen-pop path to `/client/v1/sobjects/…` or `/client/v1/query`.

**Record-detail / find sample (R2):** small second example **or** a second route in list-view — `search` + `getRecord` for Account. Still Vite static; still customer-hosted. No product `/x`.

**Customer-repo template (R3):** add `deploy/customer-repo-template/metadata/experiences/ListView.yaml` (`homeUrl`, `connectedAppApiName`, `allowedOrigins` for loopback + placeholder prod origin). Config only — no SPA in the template.

**Experience HTTP tests (R3):** `internal/httpapi/experience_routes_test.go` via `internal/testutil`: list empty; create; get; patch origins; delete; 404; managed-ownership 403 if a fixture exists. Scope `metadata` required.

### 2.4 Tests for kits (no live API)

Node built-in test runner after `tsc`: `"test": "tsc -p tsconfig.json && node --test dist/*.test.js"`. Put `src/index.test.ts` next to sources; **exclude** `*.test.js` from npm `files`. Mock `fetch`:

- `@one/client`: `query` posts `{ object: "Account" }` not `objectApiName`; `getRecord` path contains `/sobjects/`; header `One-API-Revision` is `1`; 400 `{ error: "API_REVISION_UNSUPPORTED" }` throws `OneAPIError` with that code.
- `@one/auth`: authorize URL has `scope=client`, `code_challenge_method=S256`; passing `scopes: ["client", "metadata"]` throws; token POST sets the revision header.

Do not add `make test-ide`. Do not call a running API. Optional later CI job is R3.

### 2.5 AuthZ / lockstep IDE

Public Connected App defaults stay as shipped. Remainder does **not** change `ShouldIssueRefresh` (IDE azp shortcut is [BP-065](../../../backlog/BP-065-ide-backend-coupling.md)). Do not edit `tools/control-ide/**`. Do not add `ide.govern.experiences` consumers.

CORS for local Vite (`127.0.0.1:5174`) is already the install `devCORSOrigin` loopback reflect — keep; do not special-case Experience origins in Go.

### 2.6 Failure modes

| Failure | Kit behavior |
|---|---|
| Query without `object` | Install 400 `VALIDATION_ERROR` — kit should never send that body after R1 |
| Pin &lt; min or &gt; current | 400 `API_REVISION_UNSUPPORTED` → `OneAPIError`; caller shows `cta` |
| Access JWT expired, no RT | 401 on Client calls; sample “Sign in” |
| Refresh reuse / idle TTL | 401 `INVALID_GRANT`; drop tokens; Sign in |
| Metadata scope in authorize | Throw in `buildAuthorizeUrl` before redirect |
| `/records/…` | Must disappear from kit + telephony doc so Experiences do not 404 |

---

## 3. Concrete agentic build plan

### Phase R1 — Kit wire-correctness + tests (next executable remainder)

- **Owner:** `api-families`
- **Packages allowed:** `sdk/client/auth/**`, `sdk/client/client/**`, `sdk/client/examples/list-view/**`, `sdk/client/README.md`, [client-experience-telephony.md](../../client-experience-telephony.md) (path typo only), [client-experience-security.md](../../client-experience-security.md) if query/sobjects examples need a one-line fix
- **Packages forbidden:** `tools/control-ide/**`, `cmd/**`, `internal/**`, `migrations/**`, `deploy/**` (except no deploy edits in R1), `backlog/README.md`, `docs/architecture/README.md`
- **Files likely to change:** `sdk/client/client/src/index.ts` (+ `index.test.ts`); `sdk/client/auth/src/index.ts` (+ `index.test.ts`); both `package.json` `test` scripts; `sdk/client/examples/list-view/src/App.tsx`; `sdk/client/README.md`; `sdk/client/client/README.md`; `docs/client-experience-telephony.md`
- **Tests:** `cd sdk/client/auth && npm test`; `cd sdk/client/client && npm test`. Manual list-view against a local install is **optional** (query body is unit-tested). No `go test`. No `make test-ide`.
- **Exit criteria:** `query` JSON uses `object`/`select`; `getRecord` uses `/sobjects/`; both kits send `One-API-Revision`; `OneAPIError` for JSON errors; authorize rejects Metadata/Deploy/Ops/admin scopes; list-view sample compiles against the new `query` signature; telephony doc no longer cites `/records/`.
- **Dependencies:** none. BP-025 middleware already shipped. BP-063 kernel already shipped.

### Phase R2 — Auth refresh/exchange + second example

- **Owner:** `api-families`
- **Packages allowed:** `sdk/client/auth/**`, `sdk/client/examples/**`, security guide refresh section
- **Forbidden:** `tools/control-ide/**`, `internal/authz` (do not change `ShouldIssueRefresh`), product SPA host
- **Files likely to change:** `sdk/client/auth/src/index.ts`; new `sdk/client/examples/record-detail/` **or** list-view routes for search+get; `docs/client-experience-security.md` (two patterns in §2.2)
- **Tests:** auth unit tests for refresh form body + revision header; revoke POST. Example is compile-only (`npm run build` in the example)
- **Exit criteria:** `refreshAccessToken` / `revokeToken` exist; default example still **omits** `offline_access`; docs state RT must not go in `localStorage`; optional search+getRecord sample exists
- **Dependencies:** R1. Aligns BP-063 Phase 5 without touching IDE

### Phase R3 — Experience HTTP tests + template YAML + optional CI

- **Owner:** `api-families` then `deploy-ops` (template only)
- **Packages allowed:** `internal/httpapi/experience_routes.go` (tests only unless a bug is found), new `experience_routes_test.go`, `deploy/customer-repo-template/metadata/experiences/`, `.github/workflows` optional `sdk/client` job, BP-040 status notes
- **Forbidden:** `tools/control-ide/**`, product image COPY of `sdk/`
- **Tests:** `go test ./internal/httpapi -count=1` (experience cases; skip without `DATABASE_URL` like siblings). Kit `npm test` in CI if a job is added
- **Exit criteria:** create/list/get/patch/delete covered; template YAML packs; `sdk/` still in `.dockerignore`
- **Dependencies:** R1 preferred first so CI is not testing a broken query client. HTTP tests can land in parallel

---

## 4. Explicit non-goals

- `@one/react` hooks/provider (stay deferred until R1 is green; still optional after that)
- Partner certification ladder / SI badge program
- Product-hosted `/x` SPA, webpack-in-Go, or embedding Experience in Electron ([ADR-012](../../adr/012-customer-repo-and-control-ide.md) / ADR-019)
- New Control IDE Govern Experiences chrome, edit/create UI, or Operate-as-CRM ([ADR-030](../../adr/030-install-agent-runtime.md))
- Wrapping Metadata, Deploy, Ops, ingest, principals, or hosted agent runs in `@one/client`
- Changing public Connected App defaults or `ShouldIssueRefresh` (IDE azp bypass is BP-065)
- Bumping `API_REVISION_CURRENT` / per-revision adapters (BP-025 Phase 4)
- Publishing npm to the public registry as a required GA step (packages may stay `file:` in-tree)
- npm inside Deno guest ([ADR-014](../../adr/014-customer-code-automations.md))

---

## 5. Agentic implementation prompt(s)

### Prompt A — Phase R1 (kit wire + tests) — **Keep (in tree)**

Do not paste this prompt. R1 wire + `node --test` kits shipped. Next executable remainder is **Prompt B (R2 refresh/revoke/exchange)**.

### Prompt A — Phase R1 (kit wire + tests) — historical

```text
You are a Majesta One api-families agent. Implement BP-040 remainder Phase R1 only: make sdk/client speak the live Client + Auth wire and add unit tests. Do not add @one/react. Do not add Control IDE chrome. Do not host a product SPA. Do not implement refresh/exchange (that is R2).

Read first:
- docs/architecture/agentic-remainders/05-bp-040-client-experience.md (§2.1, §2.2 R1, §3 Phase R1)
- docs/architecture/client-experience-build-plan.md (Phases 1–6 shipped — do not re-plan)
- docs/adr/019-client-experience-oss-kits.md
- docs/adr/025-api-revision-versioning.md
- docs/adr/030-install-agent-runtime.md (no Govern Experiences chrome; no /x host)
- docs/client-experience-security.md
- backlog/BP-040-client-experience-oss-kits.md
- sdk/client/auth/src/index.ts
- sdk/client/client/src/index.ts
- sdk/client/examples/list-view/src/App.tsx
- internal/dataengine/query.go QueryRequest
- internal/httpapi/server.go /sobjects and handleQuery
- internal/compat/revision.go (One-API-Revision; missing header defaults to current)

Edit scope:
- sdk/client/auth/**
- sdk/client/client/**
- sdk/client/examples/list-view/**
- sdk/client/README.md and package READMEs
- docs/client-experience-telephony.md (replace /client/v1/records with /sobjects or /query)
- docs/client-experience-security.md only if a query/sobjects example is wrong

Implement:
1. @one/client query body = { object, select?, filters?, sort?, limit?, cursor?, includeDeleted?, mode? }. Remove objectApiName/fields/filter/offset.
2. getRecord/createRecord/updateRecord/deleteRecord on /client/v1/sobjects/{object}[/{id}]. Delete /records/ paths.
3. describe + describeObject on GET /client/v1/describe[/{object}].
4. OneAPIError from JSON { error, message }. Keep One-API-Revision default 1. Optional config.fetch for tests. Optional probeVersion → GET /version (no pin header).
5. @one/auth: send One-API-Revision on POST /auth/v1/token. buildAuthorizeUrl throws if scopes include metadata|deploy|ops|admin. Default scopes remain ["client"]. Optional config.fetch.
6. Tests via tsc + node --test (see remainder §2.4). Exclude *.test.js from npm files.
7. Fix list-view to client.query({ object: "Account", select: ["Name"], limit: 25 }).

Tests: cd sdk/client/auth && npm install && npm test ; cd ../client && npm install && npm test
Optional: npm run build in examples/list-view.
No go test required. No make test-ide. No tools/control-ide. No internal/ or migrations/.

Out of scope:
- refreshAccessToken / revoke / token exchange (Phase R2)
- @one/react, partner certification
- Experience Metadata HTTP tests / customer-repo-template YAML (Phase R3)
- tools/control-ide/** , cmd/** , deploy/Dockerfile
- backlog/README.md, docs/architecture/README.md
- Bumping API_REVISION_CURRENT

When done: keep BP-040 status Partially mitigated; R1 remaining bullets in the remainder doc may be marked landed in a short note on BP-040 only if you also edit that file. Commit sdk/client + the telephony path fix.
```

### Prompt B — Phase R2 (refresh helper + example)

```text
You are a Majesta One api-families agent. Implement BP-040 remainder Phase R2 only: @one/auth refresh/revoke/exchange helpers and an Experience sample that uses search+getRecord. Do not change ShouldIssueRefresh. Do not edit Control IDE.

Read first:
- docs/architecture/agentic-remainders/05-bp-040-client-experience.md (§2.2 R2, §2.3, §3 Phase R2)
- docs/architecture/refresh-token-session-build-plan.md Phase 5
- docs/client-experience-security.md
- docs/adr/006-jwt-auth.md (opaque RT)
- backlog/BP-063-refresh-token-sessions.md
- sdk/client/auth/src/index.ts (after R1)
- internal/httpapi/auth_refresh.go (call shape only — do not edit)

Edit scope: sdk/client/auth/** ; sdk/client/examples/** ; docs/client-experience-security.md

Implement the table in remainder §2.2 R2. Default list-view scopes stay ["client"] (no RT). Document opt-in offline_access + memory/BFF; never localStorage for RT. Add search+getRecord sample (new folder or list-view route). Unit-test refresh form fields + One-API-Revision.

Tests: npm test in sdk/client/auth; example npm run build.
Out of scope: tools/control-ide, internal/authz, @one/react, product /x host, partner cert, backlog/README.md.
```

### Prompt C — Phase R3 (Metadata experience tests + template)

```text
You are a Majesta One api-families agent. Implement BP-040 remainder Phase R3 only: HTTP tests for /metadata/v1/experiences and a customer-repo-template Experience YAML. Do not edit Control IDE.

Read first:
- docs/architecture/agentic-remainders/05-bp-040-client-experience.md (§2.3, §3 Phase R3)
- internal/httpapi/experience_routes.go
- internal/customerrepo/experience_pack_test.go
- internal/testutil (RequireDatabase, NewTestServer)
- deploy/customer-repo-template/README.md

Edit scope:
- internal/httpapi/experience_routes_test.go (new)
- deploy/customer-repo-template/metadata/experiences/ListView.yaml
- optional .github/workflows job: npm test under sdk/client/auth and sdk/client/client
- backlog/BP-040-client-experience-oss-kits.md status note if R1+R2+R3 are all in

Tests: go test ./internal/httpapi -count=1 (skips without DATABASE_URL).
Out of scope: tools/control-ide/**, new Govern chrome, sdk/ in Docker image, partner certification, backlog/README.md, docs/architecture/README.md.
```
