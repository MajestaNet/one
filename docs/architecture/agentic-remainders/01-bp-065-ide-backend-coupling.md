# Neutralize Control IDE coupling on the Go install — remainder tech design + agentic build plan

**Work-order slot:** 1 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-065](../../../backlog/BP-065-ide-backend-coupling.md)
**Track:** Finish
**Status of remainder:** Partial (Phase 1 AuthN neutrality landed; Phases 2–4 remain)
**Domain agents:** `authz-security` / `api-families` / `control-ide` (lockstep) / `worker-jobs` (Phase 2 persist only)
**Playbooks:** [agent-authz.md](../agent-authz.md) · [agent-api-families.md](../agent-api-families.md) · [agent-control-ide.md](../agent-control-ide.md) · [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) · [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md)
**Existing plans (do not duplicate):** [ide-backend-coupling-review.md](../ide-backend-coupling-review.md) (Phase 0 inventory — this doc is remainder-only) · [ADR-030](../../adr/030-install-agent-runtime.md) §5 · [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) (keep opaque RT contract; drop the Control IDE refresh special-case) · [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) freeze vs finish (chrome routes are remove-with-lockstep, not frozen)

---

## 1. Remainder inventory

Phase 0 (review + BP-065 + ADR-030 §5) is **shipped**. **Phase 1 AuthN neutrality has landed.** Phases 2–4 remain. Inventory is against the current tree (2026-08-23).

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| Family HTTP **not** gated on `ide.*` | Yes — zero `requireCapability(authz.CapIDE*)` call sites. Gates are family scopes + product caps (`identity.*`, `authz.manage`, `metadata.build`, `deploy.promote`, `govern.*`) | Do **not** add `ide.*` HTTP gates | Repo-wide grep; [system-capabilities.md](../system-capabilities.md) |
| MCP / hosted loop ignore `graph.*` | Yes — `HostedLoopV1Catalog` is Client read/write + invoke; `ToolCallsFromEffects` skips `graph.` prefix; `mcp.ListTools()` has no `graph.*` | Do **not** add `graph.*` to MCP. Drop persist of `graphCalls` after Phase 2 IDE Apply rewrite | `internal/agentharness/hosted.go`; `internal/inference/tools.go`; `internal/mcp/gateway.go`; `internal/agentloop/loop.go` |
| Job-class harness catalog | Yes — install-neutral preambles in `jobCatalog` (`JobCatalogVersion = "1"`). Starters set `jobClass` so Bind uses this catalog when present | Section catalog (`CatalogVersion = "5"`) still Control-IDE-shaped; `harness.operate.mutate` still hints `boardHandoff`; `ChromeHints` / `graph.*` tool names remain | `internal/agentharness/jobclass.go`; `internal/agentharness/catalog.go`; `internal/agentharness/apply.go` |
| Dual-purpose Client/Metadata (runs, ToolSpec, playbooks, MCP, `/version`) | Keep (class E) | Drop IDE-only **input keys** (`conversationId`, `contextExcerpts`, `activeTool`) once the IDE stops sending them (Phase 3) | `internal/httpapi/client_extras.go` `handleCreateAgentRun`; `internal/inference/effects.go` |
| Device enroll | Keep (class F / generic identity). Header `X-One-Device-Id` is not mTLS | Do not expand BP-022 mTLS | `internal/httpapi/device_routes.go`; `migrations/0022_system_caps_access_mode.sql` |
| `devCORSOrigin` loopback reflect | Keep (class E — any local SPA) | No change | `internal/httpapi/middleware.go`; `internal/httpapi/cors_test.go` |
| AuthN defaults mint `azp=one.controlIde` | **Phase 1** | Empty password `client_id`, install claim, token-exchange fallback use `one.install` | `internal/authz/client_access.go` `InstallAzp`; `internal/httpapi/install_claim_routes.go`; `internal/httpapi/auth_routes.go` `handleAuthTokenExchange` |
| `ShouldIssueRefresh` Control IDE shortcut | **Phase 1** | Public apps (including `one.controlIde`) need `offline_access`; `one.install` password/token_exchange still gets refresh | `internal/authz/refresh_token.go`; [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) helper |
| `clientAccessMode=ide_users` | **Phase 1** | Validate rejects writes; stored rows Effective → `open` + warn; Allow* branches removed | `internal/edge/policy.go`; `internal/authz/client_access.go`; `docs/security.md` |
| `EnsureControlIDE` on every `AUTO_SEED` | **Phase 1** | `SEED_CONTROL_IDE` (default on with AUTO_SEED); skip Ensure when off; `offline_access` on AllowedScopesHint | `internal/config/config.go`; `internal/seed/seed.go`; `internal/integration/service.go` |
| `callbackAllowed` prefix for `one-control://` | Keep in Phase 1 (IDE PKCE still needs it) | Optional later exact-match only | `internal/httpapi/auth_login_routes.go` L436–449 |
| Control IDE Connect `client_id` | **Phase 1** | PKCE + silent refresh send `client_id=one.controlIde` and `scope=offline_access`. **Claim JSON has no `client_id`** (azp is `one.install`). `exchangeOneIdToken` sends no `client_id` (default azp `one.install`). IDE has **no password-grant path** (hosted `/auth/v1/login` password form sends `client_id` only when the query string has it) | `tools/control-ide/src/renderer/oauthPkce.ts`; `refreshSession.ts`; `govern/ConnectSection.tsx` `claimInstall`; `internal/httpapi/auth_login_page.go` |
| Hosted `oneEffects` / `graphCalls` coaching | **Open** | `BuildAgentMessages` appends a Control IDE fence whenever instructions lack `` ```oneEffects ``. Tests **require** that inject | `internal/inference/effects.go` L25–37; `internal/inference/client_test.go`; `internal/inference/effects_test.go` |
| Section catalog Electron Apply | **Open** | Every section preamble starts “You are operating inside Majesta One Control IDE …”. `harness.run.tools` lists `graph.pin` / `graph.publishSubgraph` / `oneEffects`. `ChromeHints` / `ContextPackHints` include `runGraph`, `boardHandoff`, `ide.settings` | `internal/agentharness/catalog.go`; returned on `GET /metadata/v1/agents/harnesses` (`handleListAgentHarnesses`) |
| Starter `RunCoach` | **Open** | Graph.* through “the IDE bridge”; `graph.publishSubgraph`; `tool.create` via IDE. Package version `1.3.4`. Clone never overwrites existing customer rows | `internal/seed/module_agents_starter.go`; `internal/seed/agents_starter_test.go` |
| `EnrichAgentOutput` persist of `graphCalls` | **Open** (hosted loop does not Apply) | Persist on `agent_runs.output` “for optional clients”. Tests assert persist + non-execution | `internal/inference/effects.go` `EnrichAgentOutput`; `internal/agentloop/loop.go`; `internal/agentloop/loop_test.go`; `internal/agentloop/loop_integration_test.go` |
| Chrome Client island — **run-graphs** | **Open** — heavily used by IDE | `GET/PUT/PATCH /client/v1/run-graphs/{graphKey}`, `GET .../home`, `POST .../resolve`. Table `principal_run_graphs`. Scope `client` only. Registered only when `prefix == "/client/v1"` | `internal/httpapi/rungraph_routes.go`; `internal/rungraph/*`; `migrations/0053_principal_run_graphs.sql`; IDE `tools/control-ide/src/renderer/run/graph/api.ts` |
| Chrome Client island — **conversations** | **Open** — used by IDE chat | `GET/POST /client/v1/agents/conversations`, messages append. Tables `agent_conversations`, `agent_conversation_messages` | `internal/httpapi/agentconversation_routes.go`; `migrations/0052_agent_conversations.sql`; IDE `tools/control-ide/src/renderer/agents/conversations.ts`, `App.tsx` |
| Chrome Client island — **preferences** | **Open on Go; unused by current IDE** | `GET\|PUT\|DELETE /client/v1/preferences[/{kind}]` → `principal_preferences`. Tests use kind `ide.settings`. **No** `tools/control-ide` caller of `/client/v1/preferences` | `internal/httpapi/agentconversation_routes.go`; `internal/httpapi/agentconversation_test.go` |
| Chrome Client island — **principal canvases** | **Open on Go; unused by current IDE** | `GET\|PUT\|DELETE /client/v1/canvases[/{id}]` → `principal_canvas_documents`. Operate canvas chrome already removed from the IDE (ADR-021 / ToolSpec) | `internal/httpapi/canvasdocument_routes.go`; `migrations/0040_principal_canvas_documents.sql`; `internal/httpapi/canvasdocument_test.go` |
| `ide.*` catalog (29 caps) | **Open** as chrome flags, not HTTP gates | Seeded on Admin (`CanonicalCapabilities` / `EnsureSystemAdminPermissionSet`) and product packs (`capabilityPermissionSetDefs`). Access JWT does **not** carry `ide.*`. IDE fail-closed after `/me` in `scopes.ts` | `internal/authz/system_perms.go` `CapIDE*` / `AllIDECapabilities()`; `internal/seed/seed.go`; `internal/db/system_admin.go`; `migrations/0045_ide_caps_fls_freeze.sql`, `0047_ide_run_build_tools_caps.sql`; `tools/control-ide/src/renderer/scopes.ts` |
| Route registration file layout | Shipped (surprising vs module map) | `registerClientExtras` (including chrome `register*`) lives in `internal/httpapi/metadata_routes.go` L125–141, not `client_extras.go`. Wired from `server.go` | `internal/httpapi/metadata_routes.go`; `internal/httpapi/server.go` |

**Inventory surprises vs [ide-backend-coupling-review.md](../ide-backend-coupling-review.md):**

1. Review example azp `lattice.install` conflicts with [glossary.md](../../glossary.md) (“do not reintroduce Lattice”). Remainder locks **`one.install`**.
2. Control IDE sends `client_id=one.controlIde` and `scope=offline_access` on PKCE authorize, code exchange, and refresh. Claim and token-exchange still omit `client_id` (generic `one.install`).
3. Public-app refresh requires `offline_access` (no `ControlIDEAzp` shortcut). Control IDE PKCE requests that scope; generic `one.install` password/claim/token_exchange still receive refresh.
4. Preferences + principal-canvas HTTP are a chrome island with **no current IDE consumers**. Phase 3 can unregister those two without a local-store rewrite.
5. Starters already have `jobClass`, so hosted `RunCoach` uses the job-class preamble (install-neutral) **plus** customer graph.* instructions **plus** `BuildAgentMessages` auto-inject (the inject key is the fence tag `` ```oneEffects ``, which `RunCoach` text does not contain).

---

## 2. Detailed design (remainder only)

Cite [ADR-030](../../adr/030-install-agent-runtime.md) §5: prefer changing `tools/control-ide` when that lets Go delete routes, caps, or coaching. Do not invent a parallel stack. Do not add Electron-only product chrome. Do not add a fourth API family.

### 2.1 AuthN neutrality (Phase 1)

**Canonical Connected App for the optional desktop client stays `one.controlIde`** (`authz.ControlIDEAzp` / `integration.APINameControlIDE`). That string remains the apiName of the managed public PKCE app. What changes: the **install must stop inventing that azp** when the caller did not send it.

#### Generic install azp

Lock: `authz.InstallAzp = "one.install"`.

| Mint path | Today | After Phase 1 |
|---|---|---|
| `POST /auth/v1/token` `grant_type=password`, empty `client_id` | `azp=one.controlIde` | `azp=one.install` |
| `POST /auth/v1/install/claim` | always `azp=one.controlIde` | `azp=one.install` (ignore a body `client_id` if present — claim is first-admin, not an OAuth client) |
| `POST /auth/v1/token/exchange` | start `one.controlIde`; remap if OIDC aud maps to a Connected App | start `one.install`; remap if OIDC aud maps (unchanged remap) |
| `grant_type=authorization_code` | azp = code’s client | unchanged |
| `grant_type=password` with `client_id=one.controlIde` | Control IDE | unchanged |
| `grant_type=refresh_token` | preserves stored azp | unchanged (families minted as Control IDE keep that azp until re-login) |
| Bootstrap `API_KEYS` | `one.bootstrap` | unchanged |

Do **not** require `client_id` on password grant (curl first-admin / hosted login without query param must keep working). Do **not** require the managed Control IDE app to exist for claim.

JWT `azp` is a string claim, not a FK. `one.install` need not be a Connected App row.

#### Refresh rule (no apiName shortcut)

Replace `ShouldIssueRefresh` Control IDE branch with one rule for all public apps:

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

`one.install` is the **install-generic** session client, not a Control IDE special-case. Confidential apps never get refresh (secret is the long-lived credential). Client Experiences stay opt-in via `offline_access` ([refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md)).

Control IDE must therefore:

1. Keep sending `client_id=one.controlIde` on PKCE authorize, code exchange, and refresh (already true).
2. Send `scope=offline_access` on the **authorization_code** (and any future password) token request so `ShouldIssueRefresh` stays true for `one.controlIde`.
3. Claim continues to receive refresh because claim mints `azp=one.install`.
4. Default PKCE `oauthClientId` stays `CONTROL_IDE_INTEGRATION`; do not leave the field blank in a way that would mint `one.install` for a desktop session.

Amend in the **same** Phase 1 change set (docs, not a new ADR): [ADR-006](../../adr/006-jwt-auth.md) “Control IDE always receives a refresh token”; [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) helper; `docs/security.md` `ide_users` bullet.

Existing refresh families with `azp=one.controlIde` keep rotating until Sign in. No `apiRevision` bump (mint-side default; pinned IDEs that send `client_id` are unchanged).

#### Remove `ide_users`

`ide_users` is an install lock that fights MCP, `one`, and Client Experiences.

| Layer | Behavior |
|---|---|
| `edge.Validate` | Reject `clientAccessMode=ide_users` with a clear error (400 on `PUT /metadata/v1/install/exposure`) |
| `EffectiveClientAccessMode` | If a stored row still has `ide_users`, treat as `open` and log a warning (do not 500 the API) |
| `AllowClientAccess` / `AllowBearerAzp` | Delete the `ClientAccessIDEUsers` branches |
| Constant | Remove `ClientAccessIDEUsers` after call sites and tests are gone, or keep as deprecated unused |
| IDE copy | `ConnectSection.tsx` client-credentials subtitle that names `ide_users` — delete that clause |
| Operator docs | `docs/security.md`, `docs/auth-adapters.md`, `docs/architecture/scim-provisioning.md` — `open` (default) or `registered_clients` |

Do **not** finish `registered_clients` registry checking as a BP-065 extra (class E incomplete). Default stays `open`.

#### Optional `EnsureControlIDE`

Today `seed.Seed` always calls `EnsureControlIDE` when `AUTO_SEED` is on. Lock:

- New env `SEED_CONTROL_IDE` (default **on** when `AUTO_SEED=1`, so existing installs and IDE first-run stay green).
- When `SEED_CONTROL_IDE=0` (or `AUTO_SEED=0`), skip `EnsureControlIDE`. Claim, MCP, password, and Client CRUD must still work.
- Add `offline_access` to the seeded app’s `AllowedScopesHint` (`openid`, `email`, `profile`, `offline_access`) so the public-app hint matches the refresh rule. `validatePublicScopes` already allows `offline_access`.
- Keep `callbackAllowed` `one-control://` prefix in Phase 1.
- Do **not** have the IDE create the Connected App via Metadata on first-run in Phase 1 (extra surface). Document: operators who skip seed and still want PKCE can enable the managed app later or set the flag.

`control-ide@one.local` + `MetadataDeveloper` is IDE-shaped service principal seed. When the flag is off, that principal is not created. Do not delete existing rows on upgrade (idempotent skip).

#### AuthZ / failure modes (Phase 1)

- Unknown `client_id` on password grant: still accept as azp string (today’s behavior) unless `registered_clients` later grows a registry check — out of Phase 1.
- Token exchange with no Connected App map: `one.install`, not Control IDE.
- `AllowBearerAzp(registered_clients)` still requires non-empty azp; `one.install` counts as non-empty.
- Do not gate claim, MCP, or Client CRUD on IDE entitlement (BP-062 stays Frozen).
- Do not require `azp=one.controlIde` on `/me`.

### 2.2 Coaching (Phase 2)

Hosted loop and MCP **already** do not execute `graph.*`. Coaching still spends tokens on Electron Apply.

#### Drop auto-inject

`BuildAgentMessages` must **not** append the Control IDE `oneEffects` / `graphCalls` / `proposal` / `boardHandoff` fence. Keep:

- Native `tools` / `tool_calls` (BP-006).
- Fence fallback **only** for MCP-named `toolCalls` inside `` ```oneEffects `` if the **customer/AgentSpec** text already taught that envelope (parser stays; inject goes).
- Serialization of `input.conversation` history (generic).
- `contextExcerpts` / `activeTool` formatting may remain until Phase 3 drops those IDE-only input keys — they are not azp-gated coaching.

Do not azp-gate a second coaching dialect.

#### Section catalog vs job-class catalog

`Apply` uses job-class catalog when `jobClass` is set; otherwise section catalog (`internal/agentharness/apply.go`). `primarySection` XOR fill stays ([agent-runtime-build-plan.md](../agent-runtime-build-plan.md)); do not break YAML.

Phase 2 rewrite of the **section catalog** (`catalog.go`):

- Preambles in install / job-class language (same voice as `jobclass.go`), not “inside Majesta One Control IDE”.
- Delete `ChromeHints` from `Definition` **or** leave the JSON field empty and stop populating it (prefer delete + bump `CatalogVersion` so harness version mismatch warnings fire honestly).
- Remove `graph.pin`, `graph.publishSubgraph`, `oneEffects` from preambles and `ContextPackHints` (`runGraph`, `boardHandoff`, `ide.settings` → install-neutral hints: `selection`, `activeEnv`, `capsSummary`, …).
- Bump `CatalogVersion` (today `"5"` → `"6"`) and each section `Definition.Version`.

Job-class catalog: drop `boardHandoff` from `harness.operate.mutate` `ContextPackHints`. Bump `JobCatalogVersion` (`"1"` → `"2"`). Settings job class may keep binding `harness.settings.install` until a later catalog merge — rewrite that section preamble only.

`GET /metadata/v1/agents/harnesses` returns both catalogs; chrome fields disappearing is an additive-compat shrink of optional JSON.

#### `RunCoach` starter

Rewrite instructions to query / describe / platform actions / skills — **no** `graph.*`, **no** “IDE bridge”, **no** `tool.create` via Electron. Keep Curator/Doer language only if it still means “stage Client-shaped proposals for human approve on `/agents/runs`”, not graph Apply.

- Bump `AgentsStarterPackageVersion` (`1.3.4` → `1.3.5` or next).
- Clone still **does not overwrite** existing customer `api_name` rows. Existing cloned `RunCoach` playbooks stay IDE-shaped until the customer edits them. Tests that assert `graph.publishSubgraph` on the **template** must flip to the new text.
- Do not mention vendor paths (`internal/`, `BP-*`) in managed starter instructions (ADR-010).

#### IDE Apply, then drop persist

Order inside Phase 2 (same change set if tests stay green; otherwise 2a then 2b):

1. **2a — IDE:** stop Apply of `graphCalls` from `agent_runs.output`. Either (a) drive the graph from Client describe/query + local graph mutations the user (or local parser) performs, or (b) parse model **prose / local fence** in the renderer only — never require Go persist. Touch: `run/runToolEffects.ts`, `run/agentEffectsFromSummary.ts`, `run/graph/agentGraphTools.ts`, `run/graph/proposalStaging.ts`, `operate/handoff.ts`.
2. **2b — Go:** stop `EnrichAgentOutput` from promoting `graphCalls` / `proposal` / `boardHandoff` onto `agent_runs.output`. Keep promoting MCP `toolCalls` if that is how fence fallback works. Update `internal/agentloop` tests: `graphCalls` must **not** be required on output; still must **not** execute.

Hosted loop continues to ignore `graph.*` if a customer AgentSpec still emits them.

### 2.3 Remove chrome Client island (Phase 3)

Auth: these routes are `scope: client` only, registered only on `/client/v1` (not `/v1` aliases) from `registerClientExtras` in `metadata_routes.go`.

**Default: delete from the install after the in-repo IDE no longer calls them.** Do not keep kernel tables “for compat.”

| Surface | IDE today | Remainder action |
|---|---|---|
| Run-graphs | Heavy (`run/graph/api.ts`, Operate home) | Move document to **IDE-local encrypted store** (sibling of `session.bin`, CIDE-10 crypto in `src/main/sessionStore.ts` — do **not** stuff graphs into the JWT session blob). Hydrate cards via existing Client describe/query (or keep a **thin** resolve helper only if still needed — prefer Client query in the IDE). Then unregister Go routes |
| Conversations | Used (`agents/conversations.ts`, `App.tsx`) | Move thread list + messages to the same local store (or drop persist across restarts). Stop sending `conversationId` on `POST /agents/runs`. Then unregister Go routes |
| Preferences | **No IDE caller** | Unregister immediately in Phase 3 |
| Principal canvases | **No IDE caller** (Operate canvas removed) | Unregister immediately. Do **not** confuse with Metadata ToolSpec `/metadata/v1/tools` (class E — keep) |

**If a surface is still needed as product** (any client, not just Electron): redesign as a generic principal document or reuse `agent_runs` — **not** `one.runGraph/v1` + `graph.*`. That needs an explicit product justification in a later ADR. This remainder does **not** promote the island to GA.

Lockstep: `control-ide` + `api-families` in one change set. After unregister, those URLs **404**.

**Later migrate (Phase 3b, same BP, separate implement slice):** drop tables `principal_run_graphs`, `agent_conversations`, `agent_conversation_messages`, `principal_preferences`, `principal_canvas_documents` once no supported IDE build reads them. Next kernel tag after current `0059_agent_job_class` (confirm journal idx at implement time). Keep `internal/rungraph` only until routes die; then delete the package if nothing else imports it.

**IDE-only run input keys:** once conversations are local, stop sending `conversationId`, `contextExcerpts`, `activeTool` from `App.tsx`. Go may ignore unknown JSON (already). Optionally stop formatting those keys in `BuildAgentMessages` in the same PR.

No MCP projection of these routes or `graph.*`. No fourth API family. Device enroll stays.

### 2.4 Drop `ide.*` (Phase 4)

Go HTTP must continue to **not** `requireCapability(CapIDE*)`.

IDE fail-closed after `/me` switches from `ide.*` to **Role family scopes + product caps**. Admin (`isAdmin`) still sees all tiles.

| Tile / mode | Gate after drop |
|---|---|
| Operate (mode) | Role scope `client` |
| Graph & Tools | `client` |
| Query | `client` |
| Monitor | `debug.read` **or** `debug.trace` |
| Explorer | `client` |
| Build (mode) | Role scope `metadata` |
| Objects / Packages / Agents / Tools / Repo | `metadata.build` |
| Deploy Pipeline | `deploy` + `deploy.promote` |
| Govern (mode) | any of `identity.users`, `identity.integrations`, `authz.manage`, `govern.network`, `govern.agents` |
| Users | `identity.users` |
| Integrations | `identity.integrations` |
| Experiences | `metadata.build` (Experience apps are Metadata) |
| Install auth | `govern.network` |
| Permissions | `authz.manage` |
| Settings (mode) | `client` (signed-in) |
| Account / Environments (Connect) | `client` |
| Hosting | `deploy.promote` |
| Inference | `metadata.build` **or** `deploy.promote` (install inference SoR is Metadata/Deploy HTTP) |

`tools/control-ide/src/renderer/scopes.ts`: replace `MODE_IDE_CAP` / `TOOL_IDE_CAP` maps; keep `hasScope` exact-match; fail-closed after `/me` when caps/scopes are known. Permissions panel checkboxes for `ide.*` go away (or read-only legacy if a PS still lists them).

**Seed / catalog:**

1. Stop merging `*IDECapabilities()` into `capabilityPermissionSetDefs()` and stop appending `AllIDECapabilities()` onto `CanonicalCapabilities`.
2. `EnsureSystemAdminPermissionSet` then seeds product caps + debug only (`ON CONFLICT DO UPDATE` will **strip** `ide.*` from managed Admin on next seed — acceptable; chrome flags are not HTTP gates).
3. `ValidateSystemPermissions` / `KnownCapabilities`: **keep accepting** `ide.*` on PATCH for one migrate window so customer PSs that still list them do not 400. Then a later migrate (or Phase 4b) removes them from `KnownCapabilities` and strips leftover strings from `permission_sets.system_permissions`.
4. Do not add new `ide.*` strings. SQL `0045` / `0047` stay historical; no new ide-cap unions.

`GET /client/v1/me` `systemPermissions` will stop listing `ide.*` for Admin after re-seed. IDE must not treat missing `ide.operate` as “hide Operate” — it uses the table above.

### 2.5 Cross-cutting contracts

- **API families:** chrome routes stay Client until deleted. Do not move them to Metadata or a fourth family ([ADR-004](../../adr/004-three-api-families.md)).
- **MCP:** adapter only; never `graph.*` ([hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) “Out of hosted loop v1”).
- **License:** BP-062 Frozen — not an install gate.
- **Electron LLM / in-IDE coding-agent host:** forbidden.
- **Lockstep fence:** each implement phase may edit `tools/control-ide/**` **only** to consume the cleaner API or local state ([agent-control-ide.md](../agent-control-ide.md)).

---

## 3. Concrete agentic build plan

Implement **Phase 1 first**. Phases 2–4 are gated: do not start the next phase until the previous phase’s exit criteria are green **and** the user/parent says continue.

### Phase 1 — AuthN neutrality + IDE Connect lockstep

- **Owner domain agents:** `authz-security` then `control-ide` (same change set). Cite [agent-authz.md](../agent-authz.md) + [agent-control-ide.md](../agent-control-ide.md).
- **Packages allowed:** `internal/authz`, `internal/edge`, `internal/httpapi` (token/claim/refresh/exposure/login page tests), `internal/integration` (`EnsureControlIDE` hint + seed skip), `internal/seed`, `internal/config`, `internal/db` tests as needed, `tools/control-ide/**` (Connect / PKCE / refresh / claim / copy only), AuthN docs listed in §2.1.
- **Packages forbidden:** `internal/inference`, `internal/agentharness` coaching, chrome route deletion, `migrations/` (no kernel DDL in Phase 1), new Electron chrome, BP-062, MCP catalog.
- **Files likely to change:**
  - `internal/authz/client_access.go` (`InstallAzp`; keep `ControlIDEAzp` as the desktop apiName)
  - `internal/authz/refresh_token.go`, `refresh_token_test.go`
  - `internal/authz/client_access_test.go`
  - `internal/httpapi/install_claim_routes.go`, `install_claim_routes_test.go`, `password_routes_test.go`
  - `internal/httpapi/auth_routes.go` (token exchange default)
  - `internal/httpapi/auth_refresh.go` (no behavior change beyond helper)
  - `internal/edge/policy.go`, `policy_test.go`
  - `internal/httpapi/exposure_routes.go` (via `Validate`)
  - `internal/integration/service.go` (`AllowedScopesHint` + still-idempotent Ensure)
  - `internal/seed/seed.go`, `internal/config/config.go` (`.env.example` if env vars are documented there)
  - `tools/control-ide/src/renderer/oauthPkce.ts` (`scope=offline_access` on login/authorize/token)
  - `tools/control-ide/src/renderer/refreshSession.ts` (keep `client_id`)
  - `tools/control-ide/src/renderer/govern/ConnectSection.tsx` (claim unchanged body OK; drop `ide_users` copy; PKCE default client id)
  - `tools/control-ide/src/renderer/oauthPkce.test.ts`, `api.test.ts`, `panels/ConnectPanel.test.tsx`
  - Docs: ADR-006 refresh bullet; refresh-token-session-build-plan helper; `docs/security.md`; `docs/auth-adapters.md`; `docs/architecture/scim-provisioning.md`
- **Tests to add or extend:**
  - `go test ./internal/authz/...` — `ShouldIssueRefresh`: Control IDE **without** `offline_access` is false; **with** it + public is true; `one.install` password/claim-equivalent true; `ide_users` branches gone
  - `go test ./internal/httpapi/...` — password grant without `client_id` mints `azp=one.install` and still returns refresh; password with `client_id=one.controlIde` + `scope=offline_access` mints Control IDE + refresh; claim `azp=one.install` + refresh, **no** requirement that `one.controlIde` Connected App exists; token exchange default `one.install`; `PUT .../exposure` `ide_users` → 400
  - `go test ./internal/edge/...` — Validate rejects `ide_users`; Effective maps stored `ide_users` → `open`
  - `go test ./internal/seed/...` or config test — `SEED_CONTROL_IDE=0` skips Ensure (if wired through seed opts)
  - `make test-ide` — PKCE URL/token body includes `client_id=one.controlIde` and `offline_access`; Connect copy no longer recommends `ide_users`
- **Exit criteria:**
  - Password grant without `client_id` does not mint `azp=one.controlIde`
  - Control IDE still signs in (explicit `client_id=one.controlIde` + `offline_access`)
  - Install claim works without Control IDE Connected App and does not mint Control IDE azp
  - `clientAccessMode=ide_users` rejected as unsupported (PUT) / treated as `open` if already stored
  - `GET /mcp/tools` still has no `graph.*` (regression; do not change MCP)
  - No chrome routes deleted
- **Dependencies:** BP-063 refresh machinery (already shipped). Does **not** wait on BP-062, BP-022, BP-054.

### Phase 2 — Coaching + IDE Apply (gated on Phase 1)

- **Owner domain agents:** `api-families` then `control-ide` then `worker-jobs` (persist tests only). Cite [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) + [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) + [agent-control-ide.md](../agent-control-ide.md).
- **Packages allowed:** `internal/inference`, `internal/agentharness`, `internal/seed` (`module_agents_starter.go`), `internal/agentloop` (persist assertions), `internal/httpapi` harness list tests if JSON shape changes, `tools/control-ide/src/renderer/run/**`, `operate/handoff.ts`, related Vitest.
- **Packages forbidden:** chrome route deletion (Phase 3); `ide.*` catalog drop (Phase 4); MCP `graph.*`; new Electron product chrome; AuthN re-litigation.
- **Files likely to change:**
  - `internal/inference/effects.go`, `effects_test.go`, `client_test.go`, `tools.go` (comments)
  - `internal/agentharness/catalog.go`, `catalog_test.go`, `jobclass.go`, `apply_test.go`
  - `internal/seed/module_agents_starter.go`, `agents_starter_test.go`
  - `internal/agentloop/loop.go` (comment), `loop_test.go`, `loop_integration_test.go`
  - IDE Apply: `run/runToolEffects.ts`, `run/agentEffectsFromSummary.ts`, `run/graph/agentGraphTools.ts`, `run/graph/proposalStaging.ts`
- **Tests:**
  - `go test ./internal/inference/...` — `BuildAgentMessages("Be brief.", …)` does **not** contain `graphCalls` / Control IDE fence; still builds conversation history
  - `go test ./internal/agentharness/...` — section preambles have no “Control IDE” / `graph.pin`; job-class operate has no `boardHandoff` hint; `primarySection` XOR still works; unset `jobClass` still binds section catalog (rewritten)
  - `go test ./internal/seed/...` — `RunCoach` template has no `graph.publishSubgraph` / IDE bridge
  - `go test ./internal/agentloop/...` — `graph.*` still not executed; persist of `graphCalls` **not** required after 2b
  - Hosted run with `jobClass=query` or `operate` does not instruct `graph.pin` unless customer AgentSpec text does
  - `make test-ide` — Apply tests updated; graph still works from local/Client paths
- **Exit criteria:** hosted `/agents/runs` for MCP builders is not coached to Electron Apply; `GET /mcp/tools` still has no `graph.*`; `primarySection` alias remains.
- **Dependencies:** Phase 1 green. BP-006 loop already mitigated — do not re-litigate executor.

### Phase 3 — Remove chrome Client routes (gated on Phase 2)

- **Owner domain agents:** `control-ide` + `api-families` (one change set). Cite [agent-api-families.md](../agent-api-families.md) + [agent-control-ide.md](../agent-control-ide.md). `db-backend-perf` only for Phase 3b table drop.
- **Packages allowed:** `internal/httpapi` (`metadata_routes.go` `registerClientExtras`, `rungraph_routes.go`, `agentconversation_routes.go`, `canvasdocument_routes.go`), `internal/rungraph` (delete or unwire), `tools/control-ide/src/main/` (encrypted local blob), `src/renderer/run/graph/`, `src/renderer/agents/conversations.ts`, `App.tsx`. Phase 3b: `migrations/` + journal.
- **Packages forbidden:** `ide.*` drop (Phase 4); MCP projection of deleted routes; fourth family; Metadata ToolSpec deletion; device enroll deletion.
- **Files likely to change:**
  - Unregister calls in `registerClientExtras` (`metadata_routes.go` L137–140)
  - Delete or empty `rungraph_routes.go`, `agentconversation_routes.go` (prefs), `canvasdocument_routes.go`
  - Tests: `rungraph_test.go`, `rungraph_resolve_test.go`, `agentconversation_test.go`, `canvasdocument_test.go` → 404 contracts
  - IDE local store + graph/conversation adapters
- **Tests:**
  - `go test ./internal/httpapi/...` — `GET /client/v1/run-graphs/home`, conversations, preferences, principal canvases → 404; `GET /client/v1/tools`, `POST /client/v1/agents/runs`, Metadata `/tools` still 2xx
  - `make test-ide` — Operate graph + chat work **without** those URLs
- **Exit criteria:** IDE Operate/chat works without `/client/v1/run-graphs` or `/agents/conversations` (local or dropped). Preferences/canvases 404. No MCP `graph.*`.
- **Dependencies:** Phase 2 (so Go no longer persists `graphCalls` that the IDE would have Applied from output). Frozen Operate UX BPs (BP-018–021, BP-054, BP-055 remainders) stay frozen — this phase **removes** install coupling, it does not add graph features.

### Phase 4 — Drop `ide.*` (gated on Phase 3)

- **Owner domain agents:** `authz-security` + `control-ide`. Cite [agent-authz.md](../agent-authz.md) + [agent-control-ide.md](../agent-control-ide.md).
- **Packages allowed:** `internal/authz/system_perms.go` (+ tests), `internal/seed/seed.go`, `internal/db/system_admin.go`, `internal/httpapi` `/me` tests if any, `tools/control-ide/src/renderer/scopes.ts` (+ `scopes.test.ts`, Permissions panel tests), [system-capabilities.md](../system-capabilities.md).
- **Packages forbidden:** `requireCapability(CapIDE*)` on family routes; BP-062 seat-count-by-`ide.*`; new `ide.*` strings; chrome route revival.
- **Files likely to change:** listed above; `internal/seed/seed_capabilities_test.go`; `internal/authz/system_perms_test.go`.
- **Tests:**
  - `go test ./internal/authz/...` `./internal/seed/...` `./internal/db/...` — Admin canonical set has no `ide.*`; `ValidateSystemPermissions` still accepts leftover `ide.*` during the window
  - `make test-ide` — tiles hide from Role scopes + product caps; missing `ide.operate` does not hide Operate if `client` scope is present
- **Exit criteria:** new Admin seed has no `ide.*`; IDE tiles gated as §2.4; zero `requireCapability(CapIDE*)`.
- **Dependencies:** Phase 3 (so we are not teaching `ide.run.tools` as the reason graphs exist on the server).

### Close-out (docs, not a fifth product phase)

When all four phases land: set BP-065 status to Mitigated; update `backlog/README.md` (implement agent does this — not this remainder docs PR). Point ADR-030 / runtime plan at mitigated.

---

## 4. Explicit non-goals

- `requireCapability(CapIDE*)` on family routes
- BP-062 license JWS / Stripe entitlement as an install gate
- Adding `graph.*` / `graph.publishSubgraph` to MCP or hosted loop catalog
- New Electron-only product chrome (license onboarding, private update CDN, in-IDE coding-agent host, Operate as end-user CRM)
- A fourth API family or GraphQL
- Keeping chrome routes indefinitely “because the in-repo IDE still calls them”
- Breaking `primarySection` YAML (alias stays until a later jobClass-only migrate)
- ToolSpec Metadata/Client catalog removal (`/metadata/v1/tools`, `/client/v1/tools`)
- Finishing `registered_clients` Connected App registry enforcement (class E; separate if ever)
- ALB mTLS / BP-022 device remainders
- Replacing Client Experiences with a second admin SPA
- Dropping `callbackAllowed` `one-control://` in Phase 1
- Requiring `client_id` on password grant
- Calendar estimates

---

## 5. Agentic implementation prompt(s)

Paste after this docs PR merges. **Phase 1 is in tree.** Implement **Phase 2 first**. Later prompts are gated.

### Phase 1 — Keep (do not re-implement)

AuthN neutrality landed: `azp=one.install` defaults, public-app `offline_access` for refresh, `ide_users` rejected, `SEED_CONTROL_IDE`, IDE `client_id=one.controlIde`. Tests: `internal/httpapi/authn_neutrality_test.go`, Control IDE Connect/oauthPkce.

### Phase 2

```text
Implement Majesta One BP-065 Phase 2 (coaching + IDE Apply). Phase 1 AuthN neutrality is already in tree.

Read first:
- docs/architecture/agentic-remainders/01-bp-065-ide-backend-coupling.md (§2.2, §3 Phase 2)
- docs/architecture/ide-backend-coupling-review.md (class D)
- docs/architecture/agent-runtime-build-plan.md (freeze vs finish; job-class SoR)
- docs/architecture/hosted-agent-tool-loop-build-plan.md (do not add graph.* to MCP; loop already ignores graphCalls)
- docs/architecture/agent-control-ide.md
- docs/adr/030-install-agent-runtime.md
- backlog/BP-065-ide-backend-coupling.md

Scope (edit):
- Drop BuildAgentMessages auto-inject of the Control IDE oneEffects/graphCalls fence. Keep MCP toolCalls fence parsing; do not azp-gate a second dialect.
- Rewrite section catalog preambles to install/job-class language; delete ChromeHints / graph.* tool names / runGraph|boardHandoff|ide.settings hints. Bump CatalogVersion. Drop boardHandoff from harness.operate.mutate hints; bump JobCatalogVersion.
- Keep primarySection XOR / alias; do not require a Metadata migrate.
- Rewrite seed RunCoach to query / platform actions / skills — no graph.*, no IDE bridge. Bump agents_starter version. Do not overwrite existing customer clones.
- IDE: stop Apply of graphCalls from agent_runs.output (local parse or Client-only). Then drop EnrichAgentOutput persist of graphCalls/proposal/boardHandoff. Hosted loop still must not execute graph.*.

Tests:
- go test ./internal/inference/... ./internal/agentharness/... ./internal/seed/... ./internal/agentloop/...
- Hosted run jobClass=query|operate does not instruct graph.pin unless the AgentSpec customer text does
- GET /mcp/tools still has no graph.*
- make test-ide for Apply/graph parser changes

Out of scope:
- Deleting chrome Client routes (Phase 3)
- Dropping ide.* (Phase 4)
- New Electron chrome; graph.* on MCP; AuthN re-opens; BP-062
- Re-implementing Phase 1 azp / ide_users / SEED_CONTROL_IDE
```

### Phase 3

```text
Implement Majesta One BP-065 Phase 3 (remove chrome Client island) with Control IDE lockstep. Only if Phase 2 is green and the user said continue.

Read first:
- docs/architecture/agentic-remainders/01-bp-065-ide-backend-coupling.md (§2.3, §3 Phase 3)
- docs/architecture/ide-backend-coupling-review.md (class B)
- docs/architecture/agent-api-families.md
- docs/architecture/agent-control-ide.md
- docs/adr/030-install-agent-runtime.md §5
- backlog/BP-065-ide-backend-coupling.md

Scope (edit):
- Prefer changing tools/control-ide so Go can delete routes. Move run-graph + conversation state to IDE-local encrypted storage (CIDE-10 sibling of session.bin — not the JWT blob). Unregister /client/v1/run-graphs*, /agents/conversations*, /preferences*, principal /canvases* from registerClientExtras (internal/httpapi/metadata_routes.go). Preferences and principal canvases have no current IDE callers — delete without a store rewrite.
- Keep Metadata ToolSpec /client/v1/tools, POST /agents/runs, device enroll, activity-feed.
- Stop sending conversationId / contextExcerpts / activeTool once local.
- No MCP projection. No fourth API family. Do not drop tables in the same PR unless routes + IDE are already 404-clean; table drop is Phase 3b (new numbered migration after 0059).

Tests:
- go test ./internal/httpapi/... — those URLs 404; tools/runs/Metadata tools still work
- make test-ide — Operate graph + chat without run-graphs/conversations HTTP

Out of scope:
- ide.* catalog drop (Phase 4)
- graph.* on MCP; new Electron chrome; BP-062; mTLS; ToolSpec removal
- Re-adding Operate canvas
```

### Phase 4

```text
Implement Majesta One BP-065 Phase 4 (drop ide.* from product AuthZ) with Control IDE lockstep. Only if Phase 3 is green and the user said continue.

Read first:
- docs/architecture/agentic-remainders/01-bp-065-ide-backend-coupling.md (§2.4, §3 Phase 4 tile map)
- docs/architecture/ide-backend-coupling-review.md (class C)
- docs/architecture/agent-authz.md
- docs/architecture/agent-control-ide.md
- docs/architecture/system-capabilities.md
- backlog/BP-065-ide-backend-coupling.md
- docs/adr/030-install-agent-runtime.md

Scope (edit):
- IDE tiles fail-closed on Role scopes + product caps (see remainder §2.4). Update tools/control-ide scopes.ts + tests. Do not requireCapability(CapIDE*) on family routes.
- Stop seeding ide.* onto Admin CanonicalCapabilities and capabilityPermissionSetDefs. EnsureSystemAdminPermissionSet follows CanonicalCapabilities.
- Keep accepting leftover ide.* strings on PATCH for one migrate window (KnownCapabilities); do not add new ide.* strings.
- Update system-capabilities.md: ide.* are legacy accepted-on-write, not chrome SoR.

Tests:
- go test ./internal/authz/... ./internal/seed/... ./internal/db/...
- New Admin seed has no ide.*; make test-ide tile gating without ide.*

Out of scope:
- HTTP gates on ide.*; BP-062 seat counts; reviving chrome routes; graph.* on MCP; license as install gate
```
