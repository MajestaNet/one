# BP-025 API revision — remainder tech design + agentic build plan

**Work-order slot:** 10 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-025](../../../backlog/BP-025-ide-api-version-compatibility.md)
**Track:** Finish
**Status of remainder:** Keep (mitigated — regression guard) · Phase 4 adapters deferred · **pin gaps closed** (datapack, `@one/auth`, `one-mcp`, MCP revision tests)
**Domain agents:** `api-families` (owner). `control-ide` only if Connect/`apiFetch` pin regression is broken. `worker-jobs` only when Phase 4 persists the pin onto hosted `agent_runs`. Not GraphQL. Not family `/v2`.
**Playbooks:** [agent-api-families.md](../agent-api-families.md) · [agent-routing.md](../agent-routing.md) · [module-map.md](../module-map.md)
**Existing plans (do not duplicate):** [ide-api-version-compatibility-build-plan.md](../ide-api-version-compatibility-build-plan.md) (Phases 0–3 **shipped**) · [ADR-025](../../adr/025-api-revision-versioning.md) · [ADR-004](../../adr/004-three-api-families.md) · [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) (MCP adapter) · [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) (in-process `mcp.CallTool`) · [client-experience-build-plan.md](../client-experience-build-plan.md) (`sdk/client` pin **shipped** on `@one/client`)

---

## 1. Remainder inventory

Phases 0–3 of the original plan are **in tree**. Do not re-plan discovery, middleware, `/r{N}/`, Connect/CLI handshake, packaging `API_REVISION_*`, or `sdk/client` `CLIENT_PREFERRED_API_REVISION`. `PRODUCT_VERSION` is **not** the pin SoR (ADR-025). Family `/v2` is **not** this remainder.

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| Config window | `API_REVISION_CURRENT` / `API_REVISION_MIN` / fallback `API_REVISION`; `min` cannot exceed `current`; zero values coerce to 1 | None | `internal/config/config.go` `APIRevisionWindow`; `config_test.go` |
| Discovery | Unauth `GET /version`; `GET /client/v1/me`; `GET /deploy/v1/environment`. `apiRevision.{min,current,recommended}` (`recommended` = alias of `current`); `httpApi` family map; `productVersion` | None for this BP | `internal/httpapi/revision.go` `compatDiscoveryFields`; `handleVersion` / `handleMe`; `internal/compat/revision.go` `MarshalJSON`; `revision_test.go` |
| Middleware | Family + `/mcp` + flat `/v1`: header, else `/r{N}/` rewrite, else default `current`. Out of window → `400` `API_REVISION_UNSUPPORTED` + `min`/`current`/`cta`. `/version` `/healthz` `/readyz` `/scim/v2` skipped | Keep | `applyAPIRevision`; `compat.PathRequiresRevision`; `TestAPIRevisionMiddleware`; `TestAPIRevisionCompatMatrix`; `TestMCPAPIRevision` |
| `/r{N}/` alias | Rewrites onto stable family path **before** mux match; header wins over path | `/mcp/r{N}` not asserted (prefix is in `FamilyPathPrefixes`) | `compat.SplitRevisionPath`; `cloneWithPath`; `revision_test.go` `path alias rN` |
| Context stash | `httpapi.APIRevisionFromContext` | **Unused.** Key lives in `httpapi` — `mcp` / `agentloop` / worker cannot read it without an import cycle. Phase 4 must move the key to `internal/compat` | `internal/httpapi/revision.go` |
| Control IDE | Manifest `preferredApiRevision=1` / `minApiRevision=1` + soft product window; Connect hard-gates revision; `apiFetch` sends header when pin set | Header omitted if `apiRevisionPin` unset (defaults to install `current` — allowed for curl, not for Connect) | `tools/control-ide/src/renderer/compat.ts`; `api.ts`; `api.test.ts`; `ConnectSection.tsx` |
| `one` CLI | Probe `/version`; `negotiateCliPin`; persist `apiRevisionPin`; `orgGET`/`orgPOST` send header; `--api-revision` / `--force-compat`; exit 3 | None for pin (datapack `OrgClient.ApiRevisionPin` **landed**) | `cmd/one/compat.go`; `org.go`; `internal/datapack/apply.go`; `TestOrgClientDoJSONSetsAPIRevisionHeader` |
| `@one/client` | Constructor `apiRevision` default `CLIENT_PREFERRED_API_REVISION = 1`; every fetch sets `One-API-Revision`; package tests | Keep | `sdk/client/client/src/index.ts`; `index.test.ts` |
| `@one/auth` | PKCE + `/auth/v1/token` exchange **sends `One-API-Revision`** | Keep | `sdk/client/auth/src/index.ts`; `index.test.ts` |
| MCP HTTP | `POST /mcp` goes through `applyAPIRevision`. Hosts send the header. `tools/one-mcp` pins on token mint, family HTTP, and `mcpRpc` | Keep / regression | `internal/httpapi/mcp_routes.go`; `TestMCPAPIRevision`; `tools/one-mcp/src/one-client.ts` |
| Hosted `/agents/runs` | In-process `mcp.CallTool` (no second HTTP hop) | Run row has **no** stored pin. Worker resume has no revision on `context`. Fine while all pins share behavior; **blocker for Phase 4** | `internal/agentloop/loop.go`; `migrations/0000_kernel.sql` `agent_runs` |
| Packaging | Compose / Helm / App Spec / `.env.example` set `API_REVISION_*` (today `1`/`1`) | Revision changelog is a **checklist sentence**, not a file | `deploy/docker-compose.yml`; `deploy/helm/one/values.yaml`; `deploy/digitalocean/app.yaml`; `docs/release-cicd.md` |
| Phase 4 adapters | Plumbing comment only | **Deferred until a real Client wire break is declared.** Do not fake a branch | `APIRevisionFromContext` godoc; original plan Phase 4 |
| `minControlIdeVersion` | — | **Stay deferred.** Pin math is the SoR; do not add an IDE floor unless pin negotiation is wrong in production | ADR-025 §4 optional |

---

## 2. Detailed design (remainder only)

### 2.1 Regression-guard contract (all clients)

This is the **Keep** contract. Every implementing client (Control IDE, `one`, `@one/client`, `@one/auth`, MCP hosts, `tools/one-mcp`, datapack) must keep these rules. Raw curl may omit the pin.

| Rule | Locked value |
|---|---|
| Pin SoR | Integer `apiRevision`. Never parse `PRODUCT_VERSION`. Never `/client/v12`. Never family `/v2` for graduated breakage ([ADR-025](../../adr/025-api-revision-versioning.md) axes A/B/C) |
| Discovery | Unauth `GET /version` → `apiRevision.{min,current,recommended}` + `httpApi` + `productVersion`. Echo on `GET /client/v1/me`. Deploy `/environment` is optional (often 403 for human JWTs) |
| `recommended` | Alias of `current` only. Builders may pin either |
| Transport | `One-API-Revision: {N}` on every family and `POST /mcp` call. Optional `/client/v1/r{N}/…` (and `/mcp/r{N}`) has the same semantics. Header wins if both present |
| Omitted pin | Install treats as `current`. Allowed for curl / legacy scripts. **Implementing clients must still send an explicit pin** so they stay on old behavior after a bump |
| Window | `ok` iff pin ∈ `[min, current]`. Else HTTP `400`, `error=API_REVISION_UNSUPPORTED`, body fields `pin`, `min`, `current`, `cta` |
| Handshake (IDE / `one`) | Hard-block outside the window (CLI exit 3 unless `--force-compat`). Product tested-against is **warn only** (`supportedProductMinors` default 2) |
| Additive change | New optional JSON field, new route, new capability bit → **no** revision bump |
| Breaking change | Behavior a pinned client would mis-parse or mis-act → bump `API_REVISION_CURRENT`; keep an adapter for every pin still ≥ `min` |
| Sunset | Raise `API_REVISION_MIN` only with ≥ one product/IDE release notice ([release-cicd.md](../../release-cicd.md)) |
| MCP | Adapter over existing family HTTP ([ADR-010](../../adr/010-customer-agentic-platform.md)). Tool JSON must follow the **same** pin as the inbound `POST /mcp` (or the stored run pin for hosted loop). No MCP-only version namespace |
| SCIM | Stays `/scim/v2`; revision-agnostic |
| Default image window | `min=1`, `current=1` until the first declared Client wire break |

**Client pin table (today’s tree vs required):**

| Client | Must send | Today |
|---|---|---|
| Control IDE `apiFetch` | Session `apiRevisionPin` | Yes when Connect persisted the pin |
| `one` org HTTP | Org `apiRevisionPin` | Yes (`orgGET` / `orgPOST`) |
| `one datapack` | Same org pin on Client upsert HTTP | **No** — Finish gap |
| `@one/client` | Package const or constructor option | Yes |
| `@one/auth` | Header on `/auth/v1/token` (and future Auth family calls) | **No** — Finish gap |
| MCP host (`POST /mcp`) | Header from `GET /version` (`recommended`/`current`) | Documented in `builder-connect.md` / scaffold `skills/connect`; product tests omit it |
| `tools/one-mcp` | Header on token mint, family HTTP, and `mcpRpc` | **No** — Finish gap |
| Hosted `agentloop` | Persist inbound pin on the run; stash on worker `context` before `mcp.CallTool` | **Not stored** — required when Phase 4 lands, not before |

### 2.2 Finish remainder (real gaps)

Do **not** reopen Connect SoR. These are missing **explicit pins** (or tests) on clients that already exist.

1. **`internal/datapack.OrgClient`** — `doJSON` sets `Authorization` only. After `one auth login` negotiated pin `N`, `one datapack apply` talks Client upsert with omitted pin → install serves `current`. When `current` later diverges, datapack silently gets the new wire. Fix: `ApiRevisionPin int` on `OrgClient`; `cmd/one/datapack.go` copies `resolveOrgAuth().ApiRevisionPin`. **Same change as BP-048 remainder R2** — land once; do not fork a second datapack client.

2. **`@one/auth`** — `exchangeAuthorizationCode` (and any later token refresh) must set `One-API-Revision` from a package const aligned with `@one/client` (`CLIENT_PREFERRED_API_REVISION`, export one shared number or duplicate the literal `1` until the first bump). Auth family already rejects out-of-window pins (`revision_test.go` Auth cases).

3. **`tools/one-mcp`** — `OneClient.request` / `getToken` / `mcpRpc` must send `One-API-Revision` (option `apiRevision` defaulting to `1`, or `ONE_API_REVISION` env). This is the stdio fallback MCP host; remote Cursor/Claude hosts are configured by the customer JSON in [builder-connect.md](../../builder-connect.md) (already correct). Vendor tree, not product image.

4. **Regression tests that are missing today**
   - `POST /mcp` with `One-API-Revision: 99` → `400` `API_REVISION_UNSUPPORTED` (window `[1,1]` or simulated `[12,14]`).
   - `POST /mcp` omitted header → 2xx JSON-RPC (default `current`).
   - `POST /mcp/r1` rewrite reaches `handleMCP` (optional; header remains preferred).
   - Datapack `OrgClient` httptest asserts the header (may live under `./internal/datapack` or `./cmd/one`).
   - `@one/client` unit: default header is `"1"` (add a tiny test if the package has a runner; otherwise document in client README only — do not invent a new JS test stack).

`sdk/client` **client** package **is** pinning. Do not “fix” it again.

### 2.3 Phase 4 — first real Client adapter (when / how)

**Keep until a wire break is declared.** Do not invent a break to exercise the plumbing. Do not add Metadata/Deploy/Ops adapters “for completeness.” Do not plan `/client/v2`.

#### When (declaration checklist)

A change is a **revision bump** iff a client pinned to `current` today would mis-parse or mis-act after the change **on an existing Client path**. Owner: `api-families`, cited on the PR that changes the wire.

| Change | Bump? |
|---|---|
| New optional field / new route (`POST /search`, `POST /actions/{name}`, upsert-by-external-id) | No |
| New error **code** on a path that previously returned a different code for the same input | Yes |
| Existing field **meaning** (example: `totalSize` page length → match count) | Yes |
| Removing/renaming a JSON key a pin=1 client reads | Yes |
| Status code change for the same success/failure | Yes |
| Internal SQL / AuthZ / performance with identical JSON | No |
| Entire family rewrite | Family major `/client/v2` (ADR-004) — **out of this remainder** |

**Release steps when declaring:**

1. Set `API_REVISION_CURRENT = previous+1` in `.env.example`, Compose, Helm `values.yaml`, App Spec (keep `API_REVISION_MIN` at the oldest still-adapted pin, today `1`).
2. Publish a revision changelog bullet in [release-cicd.md](../../release-cicd.md) (or a short `docs/api-revision-changelog.md` if the bullet list outgrows one line). One release notice **before** any later `min` raise.
3. Bump implementing-client preferred pins (`cliPreferredApiRevision`, `IDE_COMPAT_MANIFEST.preferredApiRevision`, `CLIENT_PREFERRED_API_REVISION`, `tools/one-mcp` default) to the new `current` **only** when those clients speak the new wire. Leave `minApiRevision=1` until the adapter is sunset.
4. Do **not** bump `PRODUCT_VERSION` as a substitute. Product image may roll independently.

#### How (adapter pattern — prove once)

All in-window pins share one route tree. **No** copy-paste mux. Branch only at the response (or request) shape that diverged.

```text
HTTP  → applyAPIRevision stashes pin on context (compat key)
      → existing handleQuery / handleMCP
      → domain Query() unchanged
      → adaptClientQueryJSON(pin, domainResult)  // the only branch
MCP   → same helper (mcp.CallTool already receives ctx)
Hosted loop → pin from agent_runs row, WithRevision(ctx, pin), then mcp.CallTool
```

**Move the context key** from `httpapi` to `internal/compat`:

```go
// internal/compat/context.go (names illustrative)
func WithRevision(ctx context.Context, pin int) context.Context
func RevisionFromContext(ctx context.Context) (pin int, ok bool)
```

`httpapi.applyAPIRevision` uses `compat.WithRevision`. Handlers and `mcp` call `compat.RevisionFromContext`. Default when missing: install `current` (same as omitted header).

**Worked example — first Client break to use this pattern (illustrative, not pre-declared):**

Shipped `POST /client/v1/query` (and MCP `query`) today:

```json
{ "records": [...], "totalSize": <len(page)>, "done": true|false, "nextCursor": "...", "queryPlan": ... }
```

`totalSize` is **page length** (`len(records)` in `handleQuery` and `mcp.runQuery`). Experience-kit / Salesforce-trained clients often treat `totalSize` as **match count**. If product **declares** that pin `N+1` uses match count:

| Pin | `totalSize` | Extra |
|---|---|---|
| `<= N` (legacy, today `1`) | `len(records)` (keep) | unchanged keys |
| `N+1` (`current`) | DataEngine match count (or `-1` + `done=false` if unknown) | optional `pageSize: len(records)` — additive on the new pin only |

Implementation sketch (do not land until declared):

- `internal/compat` or a tiny `internal/httpapi/query_revision.go`: `AdaptQueryResponse(pin int, records, done, next, plan, matchCount) map[string]any`.
- `handleQuery` and `mcp.runQuery` **both** call it. Do not diverge MCP JSON from Client JSON for the same pin.
- Tests: window `[1,2]`; pin `1` still has `totalSize==len(records)`; pin `2` has match-count `totalSize`; omitted header follows `current=2`.
- DataEngine may already know match count; if not, adding a count is a **domain** change owned by `db-backend-perf` **in the same PR** as the adapter. Do not change JSON for pin `1`.

If the first **declared** break is instead `GET /client/v1/events` envelope ([BP-042](../../../backlog/BP-042-change-feed-cdc-consumer.md)): **add a new route** for CDC when possible (additive, no bump). Only bump + adapt if the existing events list shape must change. Prefer additive. Same adapter kit either way.

#### Hosted loop pin (Phase 4 prerequisite, same PR as first adapter)

`agent_runs` has no revision column. Worker `agent.run` reconstructs `authz.Actor` but not the pin.

| Store | Choice |
|---|---|
| Column | `agent_runs.api_revision_pin int` (nullable → treat as `current` for rows created before the migration) |
| Write | `POST /client/v1/agents/runs` copies `compat.RevisionFromContext` (or `current` if missing) |
| Read | `agentloop.Execute` calls `compat.WithRevision(ctx, pin)` before `mcp.CallTool` |
| Approve resume | Same stored pin; do not re-read the approve request’s header to change the run’s wire |

No Client `/v2`. No second tool namespace.

#### Optional `minControlIdeVersion`

Stay **out** of the first adapter PR. Pin window already refuses ancient IDE manifests (`minApiRevision > install.current` → `INSTALL_REVISION_TOO_OLD`). Revisit only if production shows IDEs that send a legal pin but cannot speak it.

### 2.4 Failure modes

| Case | Behavior |
|---|---|
| Pin &lt; `min` | `400` `API_REVISION_UNSUPPORTED`; CTA migrate client / update IDE |
| Pin &gt; `current` | `400`; CTA upgrade install (`/ops/v1`) or lower pin |
| Unparsable header | `400` (same code) |
| Invalid install window | `400` `UNPARSEABLE_REVISION`; CTA fix `API_REVISION_*` |
| Datapack omitted pin after bump | Would silently receive `current` JSON — this is why gap-fix A exists |
| MCP host omitted pin after bump | Same; hosts must send header from `/version` |
| Worker tool call without stored pin | Would adapt as `current` — persist pin before any adapter ships |
| Dual header + `/r{N}/` disagree | Header wins (already shipped) |

### 2.5 IDE lockstep

Do **not** unfreeze Control IDE chrome. Connect gate, session pin, and `apiFetch` header already ship. Phase 4 does **not** add Settings chrome. If an adapter changes Operate-consumed JSON, frozen IDE continues on pin `1` until a later IDE cut bumps `preferredApiRevision` (BP-065 lockstep only if that cut also deletes Go chrome routes).

---

## 3. Concrete agentic build plan

### Phase A — Finish pin-gaps + MCP regression (do now)

- **Owner:** `api-families` for tests + `sdk/client/auth` + `tools/one-mcp`. `deploy-ops` may land datapack in the BP-048 R2 slice instead — **one** implementation.
- **Packages allowed:** `internal/datapack` (`OrgClient` only), `cmd/one/datapack.go`, `sdk/client/auth/**`, `tools/one-mcp/src/one-client.ts`, `internal/httpapi/revision_test.go` (and `mcp_routes_test.go` if MCP cases live there), docs that already mention the header.
- **Packages forbidden:** `tools/control-ide/**` unless Connect header regression is proven broken. No `migrations/`. No family `/v2`. No `PRODUCT_VERSION` handshake. No Phase 4 adapter branch.
- **Files likely to change:** `internal/datapack/apply.go`; `cmd/one/datapack.go`; `sdk/client/auth/src/index.ts`; `tools/one-mcp/src/one-client.ts`; `internal/httpapi/revision_test.go`; maybe `internal/httpapi/mcp_routes_test.go`.
- **Tests:** `go test ./internal/httpapi ./internal/datapack ./internal/compat ./cmd/one`. MCP out-of-window + omitted-header cases. Datapack client sets `One-API-Revision`.
- **Exit criteria:** Every product/vendor HTTP client in the pin table sends the header (or is documented curl). `POST /mcp` rejects pin `99`. Datapack apply cannot omit the pin when `ApiRevisionPin > 0`. BP-025 stays **Mitigated**.
- **Depends on:** Nothing. Overlaps BP-048 R2 for datapack only.

### Phase B — Phase 4 adapter (Keep until declared)

- **Owner:** `api-families`. `db-backend-perf` only if DataEngine must expose match count (or equivalent) **without** changing pin-1 JSON. `worker-jobs` for `agent_runs.api_revision_pin` + loop `WithRevision`.
- **Packages allowed:** `internal/compat` (context key), `internal/httpapi` (thin adapt call sites), `internal/mcp` (same helper), `internal/agentloop`, `internal/worker` (pass-through ctx), `migrations/` (nullable pin column), packaging env defaults for `API_REVISION_CURRENT`.
- **Packages forbidden:** `tools/control-ide/**` (frozen). Family `/v2`. New commercial family. GraphQL. Metadata/Deploy/Ops deep adapters. `PRODUCT_VERSION` as pin.
- **Files likely to change:** `internal/compat/revision.go` + new `context.go`; `internal/httpapi/revision.go`; `handleQuery` (or the declared path); `internal/mcp/gateway.go` `runQuery` (or matching tool); `internal/agentloop/loop.go`; `migrations/00xx_agent_run_api_revision.sql`; `.env.example` + Compose/Helm/App Spec `API_REVISION_CURRENT`; `docs/release-cicd.md` changelog bullet.
- **Tests:** `go test ./internal/httpapi ./internal/mcp ./internal/compat ./internal/agentloop` — pin `min` keeps old JSON; pin `current` gets new JSON; MCP tool matches HTTP for the same pin; worker resume uses stored pin; omitted header follows `current`.
- **Exit criteria:** First declared Client break has two live pins; pin-1 clients (IDE / `@one/client` still on `1` / CLI min) keep working; `API_REVISION_MIN` still `1` until a later sunset PR.
- **Depends on:** A real declared Client wire break (query envelope **or** events **or** whatever the PR names). Phase A pins so datapack/MCP/auth do not silently flip to `current`.

### Phase C — Sunset (later; not this work order)

Raise `API_REVISION_MIN`, delete the pin-1 branch, bump client `minApiRevision`. Requires ≥ one release notice. Out of Phase A/B prompts.

---

## 4. Explicit non-goals

- Using `PRODUCT_VERSION` as Connect / client pin SoR (ADR-025).
- HTTP family `/v2` or replacing `/client/v1` with `/client/v12`.
- Removing flat `/v1` aliases (ADR-004 track).
- GraphQL; SCIM path versioning; MCP-only verbs or a second tool namespace.
- Speculative Metadata / Deploy / Ops per-revision adapters.
- Faking a Client break to “prove” Phase 4.
- `minControlIdeVersion` on `/version` in this remainder.
- Unfreezing Control IDE chrome; new Electron Settings.
- Re-planning Phases 0–3 (discovery, middleware, Connect, CLI handshake, `@one/client` pin, packaging).
- Editing `backlog/README.md` or `docs/architecture/README.md` from this remainder’s implementation prompts.

---

## 5. Agentic implementation prompt(s)

### (A) Gap-fix — missing pins + MCP regression guard

```text
You are the Majesta One api-families agent. Implement BP-025 remainder Phase A only:
explicit One-API-Revision pins on the clients that omit them, plus MCP revision
regression tests. Do not implement Phase 4 adapters. Do not bump API_REVISION_CURRENT.

Read first:
- docs/architecture/agentic-remainders/10-bp-025-api-revision.md (§2.1–2.2, Phase A)
- docs/architecture/agent-api-families.md
- docs/adr/025-api-revision-versioning.md
- docs/architecture/ide-api-version-compatibility-build-plan.md (shipped Phases 0–3 — do not redo)
- backlog/BP-025-ide-api-version-compatibility.md
- docs/builder-connect.md (MCP host header)
- internal/httpapi/revision.go, revision_test.go, mcp_routes.go, middleware.go
- internal/compat/revision.go
- internal/datapack/apply.go OrgClient.doJSON
- cmd/one/datapack.go, org.go, compat.go, config.go
- sdk/client/auth/src/index.ts
- sdk/client/client/src/index.ts (already pins — do not “fix”)
- tools/one-mcp/src/one-client.ts
- docs/architecture/agentic-remainders/02-bp-048-one-cli.md §2.4 (datapack pin is the same bug; land once)

Edit scope:
- internal/datapack (OrgClient header only) + cmd/one/datapack.go
- sdk/client/auth/**
- tools/one-mcp/src/one-client.ts (+ README if it shows headers)
- internal/httpapi/revision_test.go and/or mcp_routes_test.go
- Optional one-line note in sdk/client/auth/README.md

If datapack already sends One-API-Revision (BP-048 R2 landed), skip that file and
assert the test instead.

Implement:
1. OrgClient: field ApiRevisionPin; doJSON sets One-API-Revision when pin > 0.
   cmd/one datapack apply copies pin from resolveOrgAuth for target and source.
2. @one/auth: send One-API-Revision on /auth/v1/token (default const 1, same as
   CLIENT_PREFERRED_API_REVISION). Do not parse PRODUCT_VERSION.
3. tools/one-mcp OneClient: send the header on getToken, request, and mcpRpc
   (option/env default 1).
4. Tests: POST /mcp with One-API-Revision 99 → 400 API_REVISION_UNSUPPORTED;
   omitted header still reaches JSON-RPC (default current). Datapack client
   httptest asserts the header when pin is set.

Tests: go test ./internal/httpapi ./internal/datapack ./internal/compat ./cmd/one
No make test-ide. No product image. No migrations.

Out of scope:
- tools/control-ide/** (Connect already pins)
- Phase 4 adapters, APIRevisionFromContext branches, agent_runs column
- Bumping API_REVISION_CURRENT / min, family /v2, GraphQL
- PRODUCT_VERSION Connect gate
- Metadata/Deploy/Ops adapters
- backlog/README.md, docs/architecture/README.md
- Re-implementing @one/client pin (already ships)

Keep BP-025 status Mitigated. Commit when tests pass.
```

### (B) Phase 4 adapter — when a Client wire break is declared (Keep until then)

```text
You are the Majesta One api-families agent. Implement BP-025 remainder Phase B only:
the first real per-revision Client adapter for a DECLARED wire break. If no
release/PR has declared a Client wire incompatibility, stop and do not invent one.

Read first:
- docs/architecture/agentic-remainders/10-bp-025-api-revision.md (§2.3, Phase B)
- docs/architecture/agent-api-families.md
- docs/adr/025-api-revision-versioning.md
- docs/adr/004-three-api-families.md (family /v2 is out of scope)
- docs/architecture/ide-api-version-compatibility-build-plan.md Phase 4
- docs/architecture/hosted-agent-tool-loop-build-plan.md
- backlog/BP-025-ide-api-version-compatibility.md
- internal/httpapi/revision.go, server.go handleQuery (or the declared path)
- internal/mcp/gateway.go (matching tool JSON)
- internal/compat/revision.go
- internal/agentloop/loop.go
- migrations/0000_kernel.sql agent_runs
- .env.example / deploy/docker-compose.yml / deploy/helm/one/values.yaml /
  deploy/digitalocean/app.yaml API_REVISION_*

Edit scope:
- internal/compat (move revision context key here: WithRevision / RevisionFromContext)
- internal/httpapi (stash via compat; adapt JSON at the declared handler only)
- internal/mcp (same Adapt* helper as HTTP — no MCP-only shape)
- internal/agentloop + worker ctx pass-through
- migrations/: nullable agent_runs.api_revision_pin
- packaging defaults: API_REVISION_CURRENT = previous+1, MIN unchanged (1 until sunset)
- docs/release-cicd.md revision changelog bullet (or docs/api-revision-changelog.md)
- Client preferred-pin consts ONLY if those clients already speak the new wire

Implement:
1. Confirm the declared break matches §2.3 (existing Client path, pinned clients
   would mis-parse). If the change can stay additive (new route/field), do that
   and do not bump.
2. Bump API_REVISION_CURRENT. Keep adapters for every pin in [min, current].
3. One helper keyed by pin; no duplicated mux trees; no /client/v2.
4. Persist pin on agent_runs at POST /agents/runs; WithRevision before mcp.CallTool
   and on approve resume. Null old rows → current.
5. Tests: pin min keeps old JSON; pin current gets new JSON; MCP matches HTTP;
   omitted header follows current; worker uses stored pin.

Worked example if the declared break is query totalSize: pin 1 keeps
totalSize=len(page); current uses match count; optional pageSize on the new pin
only. If the declared break is something else, same kit on that handler.

Tests: go test ./internal/httpapi ./internal/mcp ./internal/compat ./internal/agentloop
       ./internal/config
Domain tests only if DataEngine must expose a new count without changing pin-1 JSON.
No make test-ide. Do not unfreeze Control IDE chrome.

Out of scope:
- tools/control-ide/** (frozen; pin 1 IDE keeps working via adapter)
- Family /v2, GraphQL, PRODUCT_VERSION as pin
- Metadata/Deploy/Ops adapters, minControlIdeVersion
- Raising API_REVISION_MIN / deleting the old branch (Phase C sunset)
- Inventing a break if none was declared
- backlog/README.md, docs/architecture/README.md

When the adapter ships, keep BP-025 Mitigated; note Phase 4 landed in the BP
Design line. Commit when tests pass.
```
