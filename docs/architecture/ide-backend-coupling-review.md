# Control IDE coupling on the Go install — review and remediation

**Status:** Review complete; Phase 1 (AuthN neutrality) implemented. Phases 2–4 remain.  
**Backlog:** [BP-065](../../backlog/BP-065-ide-backend-coupling.md)  
**Strategy:** [ADR-030](../adr/030-install-agent-runtime.md) · [agent-runtime-build-plan.md](./agent-runtime-build-plan.md)

The product is the Go install (AuthZ, metadata, Deploy/Ship, AgentSpec, job-class harness, skills, hosted tool loop, MCP). Control IDE under `tools/control-ide` is an **optional JWT client**, not the GA path. It is **not frozen against cleanup**: change the IDE in the same change set when that lets the install drop IDE-shaped AuthN, chrome-only routes, `ide.*` caps, or Electron Apply coaching ([ADR-030](../adr/030-install-agent-runtime.md) §5).

Scope of this document: `cmd/`, `internal/`, `migrations/`, `internal/seed`, and **lockstep** `tools/control-ide/**` edits required to remove that coupling. Do not add Electron-only product capability (license as install gate, in-IDE coding-agent host, Operate as end-user CRM).

---

## Verdict

The Go API is **not** gated on Control IDE. Family scopes (`client` / `metadata` / `deploy` / `ops`) and product system caps (`identity.*`, `authz.manage`, `metadata.build`, `deploy.promote`, `govern.*`) are the real HTTP gates. **No handler** calls `requireCapability` with an `ide.*` string. MCP, `one`, curl, and Client Experiences can call the commercial families without chrome grants.

The coupling that remains is still material (Phase 1 AuthN neutrality is in tree):

1. **AuthN defaults (Phase 1)** — empty password `client_id`, install claim, and token-exchange fallback mint `azp=one.install`. Public apps (including `one.controlIde`) need `offline_access` for refresh; generic install sessions still receive refresh.
2. **`EnsureControlIDE` is optional (Phase 1)** — `SEED_CONTROL_IDE` defaults on with `AUTO_SEED`; operators can skip the managed PKCE app. `clientAccessMode=ide_users` is rejected on write.
3. **Hosted agent generation still coaches Electron Apply** — `BuildAgentMessages` injects a `oneEffects` / `graphCalls` fence whenever instructions omit it; the BP-053 section catalog and `RunCoach` starter still speak Control IDE Operate.
4. **A chrome API island lives on Client** — run-graphs, IDE conversations, principal preferences, principal canvas working-sets. These are first-class kernel tables and `/client/v1` routes, registered only on the family prefix (not `/v1` aliases).

Do not keep kernel tables or mint defaults “because the IDE still calls them.” Prefer **lockstep**: update Control IDE to use generic AuthN / family APIs (or local client state), then **delete** the IDE-only install surface. That is a cleaner backend than a permanent chrome island.

---

## Classification

| Class | Meaning | Action |
|---|---|---|
| **A — IDE-shaped AuthN** | Product mint/seed assumes Control IDE | Neutralize install defaults; IDE sends explicit `client_id` |
| **B — Chrome island** | Client routes/tables only the Electron client uses | Move state to the IDE (or drop); then remove Go routes/tables |
| **C — Chrome flags in kernel AuthZ** | `ide.*` caps seeded, never used as HTTP gates | IDE hides tiles from Role scopes + product caps; drop `ide.*` from seed/catalog |
| **D — Agent coaching for Electron Apply** | Preambles / effects / starter specs teach `graph.*` | Job-class language only; drop `graphCalls` persist if IDE Apply is rewritten or removed |
| **E — Dual-purpose product** | Built for IDE, still valid for MCP / Experiences / CLI | Keep; stop documenting as IDE-only |
| **F — Not coupled** | General product the IDE happens to call | No change from this review |

Do **not** add `requireCapability(CapIDE*)` on family routes. That would invert ADR-030 (chrome would become the product gate).

---

## A — IDE-shaped AuthN

### Findings

| Location | Behavior | Class |
|---|---|---|
| `authz.ControlIDEAzp` = `one.controlIde` (`internal/authz/client_access.go`) | Canonical Connected App apiName for the desktop client | A |
| `handleAuthPasswordGrant` (`internal/httpapi/install_claim_routes.go`) | Empty `client_id` → `ControlIDEAzp` | A |
| `POST /auth/v1/install/claim` | Always `actor.Azp = ControlIDEAzp` | A |
| `POST /auth/v1/token/exchange` | Starts as `ControlIDEAzp`; remaps only if OIDC aud maps to another Connected App | A |
| `ShouldIssueRefresh` (`internal/authz/refresh_token.go`) | **Always true** when `azp == one.controlIde` (no `offline_access`). Other public apps need `offline_access` | A |
| `clientAccessMode=ide_users` (`internal/edge/policy.go`, `AllowClientAccess` / `AllowBearerAzp`) | Mint: deny `client_credentials`; azp empty or `one.controlIde`. Bearer: azp must be Control IDE or bootstrap | A — **blocks MCP/CLI/Experiences if enabled** |
| `clientAccessMode=registered_clients` | Bearer requires non-empty azp; mint does **not** check a Connected App registry | E (incomplete, not IDE-only) |
| `integration.EnsureControlIDE` (`internal/integration/service.go`), called from `seed.Seed` | Idempotent managed public app: PKCE, callbacks `127.0.0.1:5173`, `localhost:5173`, `one-control://oauth/callback`; service principal `control-ide@one.local` + role `MetadataDeveloper` | A |
| `callbackAllowed` (`internal/httpapi/auth_login_routes.go`) | Prefix-allows `one-control://` when any stored URL uses that scheme | A |
| `devCORSOrigin` (`internal/httpapi/middleware.go`) | Non-prod reflects loopback `Origin` (Vite `:5173`). Production: no CORS | E (useful for any local SPA) |

**Not found on the Go API:** IDE license / JWS / Stripe entitlement (BP-062 remains Frozen and unimplemented). Do not add it as an install gate.

### Risk

Password curl and first-admin claim mint tokens that look like Control IDE: they always get refresh tokens, and `ide_users` would accept them as the desktop client. Operators who set `clientAccessMode=ide_users` (documented as a hardening option) would lock out builder MCP and Client Experiences.

### Remediation (Phase 1 — AuthN neutrality + IDE lockstep)

1. **Stop inventing Control IDE azp on the install.** Empty password `client_id` and install claim use an install-generic azp (for example `lattice.install`) or require `client_id`. Token exchange stays on the mapped Connected App; default generic, not `one.controlIde`.
2. **Control IDE always sends `client_id=one.controlIde`** (password, PKCE, refresh). Update Connect / claim / silent refresh in `tools/control-ide` in the same PR as the Go default change.
3. **Same refresh rule for all public apps.** Either require `offline_access` for Control IDE too (IDE requests it), or issue refresh to any password/PKCE public client. Do not special-case one apiName.
4. **Remove `ide_users`.** It is an install lock that fights MCP/Experiences. Prefer `open` (default) or a finished `registered_clients` that checks the Connected App table. Update any IDE copy that recommends `ide_users`.
5. **`EnsureControlIDE` is optional seed**, not every-install kernel. IDE first-run can create/ensure the Connected App via Metadata when missing, or document a one-shot seed flag. Default seed may stay on until the IDE path is proven.
6. **Do not gate claim, MCP, or Client CRUD on IDE entitlement.**

Invasive (avoid): making `ide_users` the default; requiring `azp=one.controlIde` on `/me`; wiring mTLS as product AuthZ.

---

## B — Chrome island (Client routes + kernel tables)

Registered only when `prefix == "/client/v1"` in `registerClientExtras` / the named `register*` functions. **Not** aliased on flat `/v1`. Auth: `scope: client` only.

| Method + path | Handler file | Storage | Purpose |
|---|---|---|---|
| `GET /client/v1/run-graphs/home` | `rungraph_routes.go` | `principal_run_graphs` (`migrations/0053_principal_run_graphs.sql`) | Get-or-create personal Operate/Run graph |
| `GET\|PUT\|PATCH /client/v1/run-graphs/{graphKey}` | same | same | Principal graph document (`one.runGraph/v1` in `internal/rungraph`) |
| `POST /client/v1/run-graphs/resolve` | same | hydrate via DataEngine | Card hydrate for graph nodes (AuthZ + FLS); not a second query API |
| `GET\|POST /client/v1/agents/conversations` | `agentconversation_routes.go` | `agent_conversations` (`migrations/0052_agent_conversations.sql`) | IDE chat threads (`mode` default `operate`) |
| `GET\|PATCH /client/v1/agents/conversations/{id}` | same | same | Thread + rename |
| `POST .../conversations/{id}/messages` | same | `agent_conversation_messages` | Append turns; optional `runId` |
| `GET\|PUT\|DELETE /client/v1/preferences[/{kind}]` | same file | `principal_preferences` | Opaque blobs (tests use kind `ide.settings`) |
| `GET\|PUT\|DELETE /client/v1/canvases[/{id}]` | `canvasdocument_routes.go` | `principal_canvas_documents` (`migrations/0040_principal_canvas_documents.sql`) | Principal **working-set** documents — **not** org ToolSpec |

Device enroll (`GET /devices`, `POST /devices/enroll`, `POST /devices/{id}/revoke` in `device_routes.go`, table from `migrations/0022_system_caps_access_mode.sql`) is **generic identity** with IDE-era comments (BP-022 mTLS remainders are not product work). Header `X-Majesta One-Device-Id` is not a TLS client cert. Do not treat enroll as IDE-only AuthZ. Do not expand mTLS in product Go unless a later identity task owns it.

### Risk

New agents will “complete” graphs, conversations, or preferences as if they were product GA. Hosted loop already correctly **does not Apply** `graph.*`. The island is unused by MCP/CLI today. Leaving the routes “forever for the IDE” keeps the install coupled.

### Remediation (Phase 3 — remove from the install, lockstep IDE)

Prefer a cleaner kernel over a permanent chrome island.

1. **Default:** move run-graph / conversation / preference / principal-canvas state into Control IDE (session / encrypted local store). Then **unregister** the Go routes and stop writing new kernel rows. Drop tables in a later migrate once no supported IDE build reads them.
2. **If a surface is still needed as product** (any client, not just Electron), redesign it as a generic principal document or reuse `agent_runs` — not `one.runGraph/v1` + `graph.*`. That needs an explicit product justification, not “the panel already exists.”
3. **No MCP projection** of `graph.*` or these routes.
4. Do **not** create a fourth API family for chrome.
5. Cross-plane: `control-ide` + `api-families` in one change set. IDE Vitest (`make test-ide`) plus Go route tests.

---

## C — `ide.*` capabilities (chrome flags, not HTTP gates)

29 caps in `internal/authz/system_perms.go` (`CapIDE*` through `AllIDECapabilities()`). Included in `CanonicalCapabilities` / Admin seed (`EnsureSystemAdminPermissionSet`). Product packs in `seed.capabilityPermissionSetDefs()` merge matching chrome onto Operate / Build / Deploy / Govern.

SQL: `migrations/0045_ide_caps_fls_freeze.sql`, `migrations/0047_ide_run_build_tools_caps.sql`. Settings/inference caps land on re-seed, not a dedicated SQL union.

`GET /client/v1/me` returns `systemPermissions` (effective union). Access JWT does **not** carry `ide.*`. Fail-closed after `/me` is an **IDE client** contract (`docs/architecture/system-capabilities.md`, `authz-ide-fls-build-plan.md`) — **not** enforced in Go.

Repo-wide: **zero** `requireCapability(authz.CapIDE*)` call sites.

### Risk

The catalog reads as if Control IDE modes were a product AuthZ surface. Seat-count-by-`ide.*` (BP-062) would over-count every Admin/Operate assignee. MCP/CLI correctly ignore the flags.

### Remediation (Phase 4 — drop `ide.*` from product AuthZ)

1. **IDE fail-closed uses product caps + Role scopes**, not `ide.*`. Map tiles in the client: Operate → `client`; Build Objects → `metadata` + `metadata.build`; Govern Users → `identity.users`; Ship → `deploy` + `deploy.promote`; Settings Hosting → `deploy.promote`; etc. Update `tools/control-ide` `/me` chrome checks in the same change set.
2. Stop seeding `ide.*` onto Admin and product packs. Accept remaining strings on PATCH for one migrate window, then remove from `KnownCapabilities`.
3. Do not add new `ide.*` strings. Do not add `requireCapability(CapIDE*)` on family routes.

---

## D — Agent coaching for Electron Apply

Hosted loop and MCP **do not** execute `graph.*`. `HostedLoopV1Catalog` is Client read/write + `invoke_action` / `invoke_skill`. `ToolCallsFromEffects` skips names with prefix `graph.`. `agentloop` persists `graphCalls` / `proposal` / `boardHandoff` on `agent_runs.output` and never Applies them.

Coaching still teaches Electron Apply:

| Location | What the model is told |
|---|---|
| `inference.BuildAgentMessages` (`internal/inference/effects.go`) | If instructions lack ` ```oneEffects `, append a Control IDE fence with `graphCalls`, `proposal`, `boardHandoff` |
| BP-053 section catalog (`internal/agentharness/catalog.go`) | Every preamble starts “You are operating inside Majesta One Control IDE …”. `harness.run.tools` lists `graph.pin`, `graph.publishSubgraph`, `oneEffects`. `ChromeHints` / `ContextPackHints` (`runGraph`, `boardHandoff`, `ide.settings`) |
| Job-class catalog (`internal/agentharness/jobclass.go`) | Install-neutral preambles (good). `harness.operate.mutate` still hints `boardHandoff`. `settings` job class still binds `harness.settings.install` (section catalog) |
| `seed` `RunCoach` (`internal/seed/module_agents_starter.go`) | Graph.* through “the IDE bridge”; `graph.publishSubgraph`; `tool.create` via IDE |
| Apply path | When `jobClass` is **unset**, Bind uses the section catalog — so legacy AgentSpecs still get IDE preambles |

`primarySection` (`operate|run|build|ship|govern|settings`) remains a **compatibility alias** for job class (BP-064 / BP-053). That alias is not chrome-only; YAML and Metadata still use it. Keep the XOR fill. Do not require a breaking Metadata migration.

### Risk

Hosted `/agents/runs` for MCP builders still spends tokens on Electron Apply. Starter `RunCoach` is the default operate-class coach and is IDE-shaped.

### Remediation (Phase 2 — coaching + IDE Apply)

1. **Drop auto-inject** of the Control IDE `oneEffects` fence in `BuildAgentMessages`. Hosted loop already parses `toolCalls`. Do not azp-gate a second coaching dialect on the install.
2. Rewrite the **section catalog** preambles to install/job-class language (or stop applying that catalog once every starter + customer spec has `jobClass`). Delete `ChromeHints` / `graph.*` tool names from Go.
3. Rewrite `RunCoach` to query / platform actions / skills — no `graph.*`. Bump `agents_starter`. Existing cloned customer playbooks stay until the customer edits them.
4. **IDE Apply:** either (a) stop Apply of `graphCalls` and drive the graph from Client query/describe only, or (b) keep Apply purely client-side from model prose the IDE parses locally. Then **drop** `EnrichAgentOutput` persist of `graphCalls` / `proposal` / `boardHandoff` on `agent_runs.output`.
5. Hosted loop and MCP still never execute `graph.*`.

---

## E — Dual-purpose product (keep)

| Surface | Why it stays |
|---|---|
| `GET\|POST\|PATCH\|DELETE /metadata/v1/tools` (legacy `/canvases` alias) | ToolSpec is org metadata (ADR-021). Experiences may consume later; not dead chrome |
| `GET /client/v1/tools[/{apiName}]` | AuthZ-filtered runtime catalog (`canOpen`) |
| `POST/GET /client/v1/agents/runs` (+ stream, approve) | Hosted loop (BP-006). Drop IDE-only input keys (`conversationId`, `contextExcerpts`, `activeTool`) once the IDE stops sending them |
| `GET /client/v1/agents/playbooks` | Product catalog (`jobClass` SoR, `primarySection` alias) |
| `GET /metadata/v1/agents/harnesses` | Returns section catalog **and** `jobClasses` |
| Inference Metadata/Deploy routes | Install SoR (BP-052). Settings UI is an optional client |
| `POST /mcp`, `GET /mcp/tools` | Builder adapter (ADR-010 / 030) |
| `GET /version`, `One-API-Revision` / `/r{N}/` | All clients (BP-025 named “ide-api-version”; ADR-025 is the pin) |
| `GET /client/v1/activity-feed` | General Client composition |
| `GET/POST /deploy/v1/peers` | Topology for CLI/docs; IDE env switcher was a consumer |
| `/metadata/v1/experiences` | Client Experience apps (ADR-019) — not ToolSpec |
| PKCE + Connected Apps | Generic OAuth. Only the **seeded** `one.controlIde` row is IDE-specific |

Legacy `/metadata/v1/canvases` → **deprecate-with-compat** toward `/tools`. `allowedCanvasSpecs` JSON key is the same array as `allowedToolSpecs`.

---

## F — Explicitly not IDE-specific

Record CRUD, query, search, composite, bulk, ingest, platform actions, automations invoke, audit, principals, Roles/PS, SCIM, sharing, Ops rolls, Deploy promote/pack, worker/outbox. `debug.trace` is catalogued next to Operate Monitor in docs; it is still a product debug cap (BP-033/034), not an `ide.*` gate.

---

## What MCP must never grow

Already locked in [agent-runtime-build-plan.md](./agent-runtime-build-plan.md):

- `graph.*` / `graph.publishSubgraph`
- Run-graph / conversation / preference / principal-canvas HTTP
- Ops mutate

Do not add a second tool namespace for Electron Apply.

---

## Phased remediation (no calendar)

Lockstep is allowed and preferred. Each implement phase may edit `tools/control-ide/**` **only** to consume the cleaner API (or local state).

| Phase | Track | Edits | Behavior change? |
|---|---|---|---|
| **0** | This document + BP-065 | `docs/`, `backlog/`, ADR-030 §5 | Policy only |
| **1** | AuthN neutrality | Go token/claim/refresh + IDE Connect (`client_id`, `offline_access`) | Yes — azp and refresh |
| **2** | Coaching | `internal/inference`, `internal/agentharness`, `internal/seed`, IDE Apply parser | Yes — prompts; drop `graphCalls` persist after IDE stops Apply-from-output |
| **3** | Chrome island **removal** | Unregister Client chrome routes; IDE local store (or drop features); later migrate drop tables | Yes — those URLs 404 |
| **4** | Drop `ide.*` | Seed/catalog + IDE tile gating from Role/product caps | Yes — `/me` no longer lists `ide.*` |

Phase 1–2 fight ADR-030 in production today. Phase 3–4 are the cleanup that is now in scope because the IDE can move.

### Acceptance (when implementing)

- Password grant without `client_id` does not mint `azp=one.controlIde`
- Control IDE still signs in (explicit `client_id=one.controlIde`)
- Install claim works without Control IDE and does not require the managed app
- `clientAccessMode=ide_users` is removed or rejected as unsupported
- Hosted run with `jobClass=query` (or `operate`) does not instruct `graph.pin` unless the AgentSpec customer text does
- `GET /mcp/tools` still has no `graph.*`
- After Phase 3: IDE Operate/chat still works without `/client/v1/run-graphs` or `/agents/conversations` (local or dropped)
- After Phase 4: IDE tiles hide from Role scopes + product caps; new Admin seed has no `ide.*`

### Non-goals

- Gating HTTP on `ide.*`
- IDE license JWS as an install gate (BP-062)
- ALB mTLS in product Go (BP-022)
- A fourth API family or GraphQL
- Replacing Client Experiences with a second admin SPA
- Adding an in-IDE coding-agent host
- Keeping chrome routes indefinitely “for compat” when the IDE in-repo can be updated

---

## Playbook pointers

| Concern | Playbook |
|---|---|
| Token azp / `ide_users` / `ide.*` catalog | [agent-authz.md](./agent-authz.md) |
| Chrome routes vs family ownership | [agent-api-families.md](./agent-api-families.md) |
| Harness / MCP / hosted loop | [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) · [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) |
| Control IDE lockstep (consume cleaner APIs / local state) | [agent-control-ide.md](./agent-control-ide.md) |

---

## Inventory method

Read ADR-030, API-family and AuthZ playbooks, then verified register functions in `internal/httpapi` (`routes`, `registerClientFamily`, `registerClientExtras`, `registerMetadataWrites`, `registerRunGraphRoutes`, `registerAgentConversationRoutes`, `registerCanvasDocumentRoutes`, `registerDeviceRoutes`) plus `internal/authz/system_perms.go`, `client_access.go`, `refresh_token.go`, `internal/integration/service.go` `EnsureControlIDE`, `internal/agentharness`, `internal/inference/effects.go`, `internal/agentloop`, `internal/rungraph`, `internal/seed/module_agents_starter.go`, and kernel SQL `0040`, `0045`, `0047`, `0052`, `0053`.
