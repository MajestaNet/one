# Control IDE demo-client fidelity uplift

**Status:** Active plan (explicit unfreeze of **demo-client honesty**, not frozen Electron chrome).  
**Backlog:** [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md)  
**Playbook:** [agent-control-ide.md](./agent-control-ide.md)  
**Domain agent:** `control-ide` (IDE-only workstreams). Spawn `api-families` / `authz-security` / `worker-jobs` only for the **API gaps** table — do not invent chrome-only kernel routes.  
**Depends on:** shipped family HTTP ([api-families.md](../api-families.md)), hosted loop ([hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) / [BP-006](../../backlog/BP-006-agent-guardrails.md)), install coupling cleanup ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)).  
**Does not reopen:** frozen Control IDE chrome (license, update CDN, Operate-as-CRM, BoardHandoff, Hosting admin UI, four-tile IA, collection-node remainders). See [ADR-030 freeze table](./agent-runtime-build-plan.md#freeze-vs-finish).

Audit date: 2026-08-25 against the current tree (`internal/httpapi` + `tools/control-ide/src/renderer`).

---

## Thesis

Control IDE is an **optional JWT demo client** of the Go install ([ADR-030](../adr/030-install-agent-runtime.md) §5). The install already implements Client / Metadata / Deploy / Ops / Auth / SCIM / MCP plus a hosted `/agents/runs` tool loop. The IDE already calls a real subset of those families — and it also **stubs, auto-greens, and documents APIs that do not exist**.

This plan makes the demo **honest**: every panel either exercises a live family route with real success/failure, or it is labeled unavailable / removed. It does **not** turn Electron into the product, the Ship GUI of record, or an end-user CRM.

```text
Go install (product)
  family HTTP + hosted loop + MCP + one CLI
        │
        ▼
Control IDE (optional demo)
  same JWT, same routes, no parallel AuthZ, no fake green
```

---

## Locked decisions

| Decision | Choice |
|---|---|
| Product | Go install. IDE is a thin Bearer client. |
| Honesty | No silent stub success. Offline seed is allowed only behind an explicit **Offline** badge and never on a connected session. |
| Hosted loop | The install **executes** MCP tools ([BP-006](../../backlog/BP-006-agent-guardrails.md) mitigated). The IDE must **consume** `awaiting_tool_approval` + `POST .../approve`. It must not apply writes locally and pretend the loop ran. |
| Chrome island | Do **not** add consumers of `/client/v1/run-graphs`, conversations, preferences, or principal canvases. Those are [BP-065](../../backlog/BP-065-ide-backend-coupling.md) Phase 3 delete-with-lockstep. Existing graph/chat may keep calling them until Phase 3; new work uses family APIs. |
| `ide.*` caps | Do not add new `ide.*` strings. New tile gates use family scopes + product caps (`metadata.build`, `identity.users`, `deploy.promote`, `authz.manage`, `debug.read`, …) — BP-065 Phase 4. |
| New panels | Allowed only when they bind a **shipped** family route the demo cannot otherwise reach. No new Operate graph chrome, license, update CDN, or in-IDE coding-agent host. |
| API gaps | If the IDE needs a list/get/patch the Go mux does not register, hand off to `api-families` — do not fake the UI. |
| Monitor | Do not invent `/client/v1/debug/*` in the IDE. That route is **not registered**. Honest empty-state until [BP-033](../../backlog/BP-033-customer-runtime-isolation.md) Phase 3 lands. |
| Ship of record | `one` CLI + Deploy HTTP ([BP-048](../../backlog/BP-048-one-cli.md)). IDE Deploy is a twin: fix lying status, do not add peer-to-peer promote. |
| End-user CRM | Client Experience ([ADR-019](../adr/019-client-experience-oss-kits.md)). Do not deepen Operate record UX ([BP-018](../adr/030-install-agent-runtime.md) stays Frozen). |

---

## Audit summary (what is real vs what lies)

### Already a real JWT client (keep; bugfix only)

| Surface | Evidence | Family routes |
|---|---|---|
| Connect / claim / PKCE / refresh | `govern/ConnectSection.tsx`, `oauthPkce.ts`, `refreshSession.ts` | `/auth/v1/token`, `/authorize`, `/install/claim`, `/revoke` |
| Session + revision pin | `api.ts`, `compat.ts` | `GET /client/v1/me`, `GET /version` |
| Operate graph persist + hydrate | `run/graph/` | `/client/v1/run-graphs/*`, `/resolve`, `/search`, `/activity-feed` |
| List View / Object Home | `run/RunObjectHomePanel.tsx` | describe, query, sobjects CRUD, composite (≤25) |
| Published ToolSpecs | `run/RunToolPanel.tsx`, `panels/ToolsPanel.tsx` | `/client/v1/tools`, `/metadata/v1/tools` |
| Agent catalog + SSE generate | `App.tsx`, `agents/runs.ts` | `/client/v1/agents/playbooks`, `POST/GET .../runs`, SSE |
| Object Manager create | `panels/ObjectManagerPanel.tsx` | `POST/PATCH /metadata/v1/objects`, `POST /fields`, `GET /field-types` |
| Packages enable/disable | `panels/PackagesPanel.tsx` | `/metadata/v1/packages` |
| AgentSpec wizard | `panels/AgentsPanel.tsx` | `/metadata/v1/agents/playbooks`, `GET .../harnesses` |
| Repo init/export | `panels/RepoPanel.tsx` | `/deploy/v1/packages/initialize-repo`, `/export` |
| Deploy validate → promote | `panels/DeployPanel.tsx` | `validate-local`, `tests/runs`, `promotions` |
| Users / integrations / install auth | Govern panels | principals, integrations, `/metadata/v1/install/auth` |
| Outbound connectors + OAuth | `OutboundConnectorsPanel.tsx` | `/metadata/v1/connectors`, `/auth/v1/connectors/.../authorize` |
| Roles + permission-set object CRUD | `panels/PermissionsPanel.tsx` | `/client/v1/roles`, `/metadata/v1/permissions/sets` |
| Hosting cloud verbs | `govern/cloud.ts` | `/deploy/v1/cloud/*` |
| Inference BYO + test chat | `panels/InferencePanel.tsx` | `/metadata/v1/inference/*`, SSE run |
| Query + Explorer | `operate/QueryPanel.tsx`, `ExplorerPanel.tsx` | `/client/v1/describe`, `/query`, packages |
| Device enroll | Connect | `POST /client/v1/devices/enroll` |

### Class A — lies (must fix first)

These tell a human the install did work it did not do, or call APIs that are not registered.

| Lie | Where | Reality | Fix (workstream) |
|---|---|---|---|
| “There is no hosted tool loop” + auto-approve parked generate | `agents/runs.ts` L163–177 (`approved: true` retry); comment L168 | Hosted loop **shipped**. Distinct park is `awaiting_tool_approval`. | **WS1** |
| Approve applies IDE `graph.*` / tool docs **without** `POST .../approve` | `workspace/types.ts` `pendingToolApply`; `App.tsx` approve path | Loop ignores `graph.*`. Writes park on the install. | **WS1** + BP-065 Phase 2 |
| Customer tests always **Passed** on HTTP 200 | `DeployPanel.tsx` `runTests` | Report body can contain failures / async `work/{jobId}`. | **WS0** |
| Monitor prefers `/client/v1/debug/trace-flags` and `/debug/logs` | `operate/MonitorPanel.tsx` | **No such routes** in `internal/httpapi`. TraceFlag / ExecutionRun objects are BP-033 Phase 3 (not seeded). | **WS0** |
| Offline CRM seed presented as a board | `operate/CrmPanel.tsx` `SEED` / `SEED_FIELDS` | Fake Acme/Northwind rows. Panel is not on the Operate rail but still deep-linkable. | **WS0** |
| Offline agent dock + stub chat replies | `workspace/types.ts` `SEED_AGENT_CHATS` `demoStub`; `App.tsx` “Stub · … received your note” | Fine when disconnected **if** labeled Seed. Must never mix with live playbooks. | **WS0** |
| Automations panel exists but is **off the Build rail** (`@deprecated`) | `workspace/types.ts` `MODE_WORKSPACE_TOOLS.build`; `AutomationsPanel.tsx` | Metadata automations CRUD is real. | **WS2** |
| Experiences is a read-only table that tells you to curl | `ExperiencesPanel.tsx` | Metadata `CRUD /experiences` is registered. | **WS3** |
| Agent tools hard-coded to 3 tokens | `AgentsPanel.tsx` `TOOL_OPTIONS` | Harness tokens include `search`, `actions.invoke`, `skills.invoke`, … | **WS2** |
| Inference/Operate comments still describe pre-loop park | `STREAM_PARKED_HINT` | Pre-LLM `awaiting_approval` vs tool-write `awaiting_tool_approval` are different. | **WS1** |

### Class B — thin (upgrade in place; no new chrome thesis)

| Surface | What works | Gap | Workstream |
|---|---|---|---|
| Object Manager | Create object + field; PATCH labels | No field PATCH/DELETE; no object DELETE; no unique/indexed/features; no projection rebuild | **WS2** |
| Permission sets | Object C/R/U/D + `ide.*` chips | **No `fieldPermissions` UI** though Metadata stores FLS | **WS3** |
| Sharing | Client CRUD is sharing-honest | **No** `/metadata/v1/sharing/*` admin UI (OWD + rules are shipped) | **WS3** |
| Users | CRUD, freeze, password, credentials, role/PS | No directory-tags / data-roles | **WS3** |
| Account settings | Read-only session summary | No `POST /me/password`; caps truncated to 12; unused `/preferences` (do **not** add — chrome island) | **WS3** |
| Deploy | validate-local + promotions to **connected** org | No `/deploy/v1/work/{id}` poll; no test assertion parse | **WS5** |
| Query | JSON query + 50-row cap | No upsert/ingest/actions from this panel (those belong WS4, not Query chrome) | **WS4** |
| List View bulk | Composite PATCH ≤25 | No ingest jobs, no external-id upsert | **WS4** |
| Outbound connectors | 3 catalog templates | Fine for demo; do not build a connector IDE | — |
| Tools (Build) | JSON layout/nodes | No visual builder (keep — not a product chrome expansion) | — |

### Class C — missing demo of shipped backend (add thin tools)

| Backend (registered) | IDE today | Add? |
|---|---|---|
| `GET/POST /client/v1/actions`, `lead.convert`, `quote.accept` | **Zero** renderer calls | Yes — Operate Object Home / Lead sheet **or** a Build “Actions” inspect tool. Not a Sales workspace ([BP-019](../adr/030-install-agent-runtime.md) stays Frozen). |
| Ingest jobs `/client/v1/jobs/ingest*` | None | Yes — Build inspect: CSV/JSON batch for one object. |
| Upsert `/sobjects/{object}/upsert` + external-id paths | None | Yes — Object Home “Upsert” when describe has unique external id. |
| `GET /client/v1/events`, ack | None | Optional thin Build Events list. |
| `GET /client/v1/audit` (admin) | None | Govern read-only table. |
| Metadata webhooks list/create | None | Build Webhooks tool. |
| Metadata validation-rules **POST only** | None | Create from Object Manager **if** list can come from object payload; otherwise API handoff. |
| `POST /metadata/v1/projections/{object}/build` | None | Button on Object Manager detail. |
| `/ops/v1/upgrades*` | **Zero** | Settings Hosting **read** of available upgrades when `scope:ops`; mutate is optional confirm. |
| `/mcp` + `GET /mcp/tools` | Copy only | Settings “Builder connect” **read-only** catalog + copy `POST /mcp` URL. Do **not** host MCP inside Electron. |
| Callable automations `POST /client/v1/automations/{apiName}/runs` | Tool chips only (`run/automations.ts`) | Expose from Automations panel. |
| Directory tags / data-roles | None | Govern. |
| Devices list/revoke | Enroll only | Connect / Account: list + revoke. |

### Class D — do not add in this uplift

| Capability | Why not |
|---|---|
| SCIM `/scim/v2` admin UI | IdP connector talks to SCIM; humans use Users + Install auth. |
| In-IDE MCP client / coding-agent host | ADR-030 forbidden. |
| License JWS / Stripe / update CDN | BP-062 / BP-015 Frozen. |
| New Operate graph features, collection remainders, reporting | BP-059 / BP-021 Frozen; BP-065 wants **less** graph coupling. |
| Peer-to-peer Deploy promote UI | Supported path is switch-org + same SHA ([customer-dx-build-plan.md](./customer-dx-build-plan.md)). |
| Files, merge, CDC | Backend [BP-045](../../backlog/BP-045-files-content-storage.md) / [BP-046](../../backlog/BP-046-record-merge-dedupe.md) / [BP-042](../../backlog/BP-042-change-feed-cdc-consumer.md) not demo-ready. |
| Principal preferences / canvases | Chrome island with **no** current IDE caller — unregister in BP-065 Phase 3, do not wire. |
| Fourth API family / `requireCapability(CapIDE*)` | Inverts ADR-030. |

### Class E — backend not ready (honest empty-state only)

| Planned | Status | IDE rule |
|---|---|---|
| `ExecutionRun` / `ExecutionLogEntry` + `/client/v1/debug/*` | BP-033 Phase 3 **Open** — not in seed, not in mux | Monitor stays “not available on this install”. Do not keep a fake debug client. |
| `GET/PATCH/DELETE /validation-rules` | Mux is **POST create only** | Do not draw a full rules manager until `api-families` adds list/get. |
| `DELETE /automations/{apiName}`, webhook patch/delete | Not registered | Disable those buttons or hand off. |

---

## Workstreams (execute in order)

Each workstream is one or more PRs. Do not batch WS1 with new panels. Verify with `make test-ide` (and `make test-ide-integration` when the workstream claims a live route).

### WS0 — Honesty pass (no new panels)

**Goal:** A connected session never shows fake records, fake test green, or APIs that 404 by design.

| Change | Files | Acceptance |
|---|---|---|
| Parse test-run body / job status; `passed` only when the suite passed | `panels/DeployPanel.tsx` | Vitest: HTTP 200 + `{ status: "failed" }` → Failed; poll `/deploy/v1/work/{id}` when returned |
| Monitor: stop calling unregistered `/client/v1/debug/*` first | `operate/MonitorPanel.tsx` | Default to “debug objects not on this install” (existing `missing` empty state). Optional: `GET /metadata/v1/objects/ExecutionLogEntry` probe. Remove dedicated-debug happy path until Go registers it. |
| CRM offline seed: connected session must not use `SEED`; offline must badge **Offline stub** and refuse mutate | `operate/CrmPanel.tsx` | Tests: connected fetch error ≠ seed rows |
| Seed agents: keep disconnected catalog; never stub-reply when `session.token` is set | `App.tsx`, `workspace/types.ts` | Connected + missing playbooks → empty dock, not Seed chat that pretends to run |
| Restore Automations on the Build rail (it is implemented) | `workspace/types.ts` `MODE_WORKSPACE_TOOLS.build`, `scopes.ts` tests | Rail includes `automations` when `metadata` + `metadata.build` |
| Copy: delete “no hosted tool loop” | `agents/runs.ts` | Comment matches BP-006 |

**Out of scope:** new Deploy UX, Monitor SSE, CRM form depth.

**Agent prompt:**

```text
Implement Control IDE BP-066 WS0 (honesty pass) under tools/control-ide only.

Read: docs/architecture/ide-demo-client-uplift-build-plan.md (WS0),
docs/architecture/agent-control-ide.md.

Do: DeployPanel test-run status parsing; Monitor stop calling unregistered
/client/v1/debug/*; CrmPanel no seed when connected; no stub chat when JWT
present; put automations back on MODE_WORKSPACE_TOOLS.build; fix hosted-loop
comment in agents/runs.ts.

Verify: cd tools/control-ide && npm test
Do not edit cmd/, internal/, migrations/.
```

---

### WS1 — Consume the hosted agent tool loop

**Goal:** Chat is a viewer of `/client/v1/agents/runs`, not an Electron executor.

The install already: streams generation, executes MCP read tools, parks writes as `awaiting_tool_approval`, continues on `POST /client/v1/agents/runs/{id}/approve` (JSON worker or SSE). Hosted loop **does not Apply** `graph.*` ([hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md)).

| Change | Detail |
|---|---|
| Status model | Treat `awaiting_tool_approval` as write-park. Keep `awaiting_approval` as pre-generation park (legacy). Do **not** collapse them. |
| Create | `stream: true`, `approved: false` is fine. **Do not** retry the create with `approved: true` to skip the park. If JSON park without SSE, show Approve — do not throw `STREAM_PARKED_HINT` as the only path. |
| Approve | Always `POST .../approve` (prefer SSE). After approve, render subsequent `token` / tool-result events. |
| `pendingToolApply` | Restrict to **client-local** effects the loop will never execute (`graph.*` until BP-065 Phase 2 removes them). Never use it for `create_record` / `update_record` / `invoke_action` / `invoke_skill`. |
| Transcript | Show MCP tool names + args/results from run output (`toolCalls`, `toolsExecuted`, SSE events) — not only `toolsPlanned` allowlist tokens. |
| Inference test chat | Keep `approved: true` **only** for the Settings probe (`oneInferenceTest`) so operators can ping a model without a playbook. Document that this path is generation-only. |

**Files:** `agents/runs.ts`, `workspace/messageModel.ts`, `workspace/StreamMessageBubble.tsx`, `App.tsx` `approveInAgentChat` / `finalizeAgentRun`, `run/runToolEffects.ts`.

**Tests:** unit for status mapping; component for Approve → POST approve; integration against a running API if `ONE_JWT` is set (create run that plans a write, assert park, approve, assert terminal).

**Cross-plane:** none required. BP-065 Phase 2 (drop `oneEffects` inject) can land after WS1 is green — then delete remaining `graph.*` Apply.

**Agent prompt:**

```text
Implement Control IDE BP-066 WS1 (hosted loop consumer) under tools/control-ide.

Read: docs/architecture/ide-demo-client-uplift-build-plan.md (WS1),
docs/architecture/hosted-agent-tool-loop-build-plan.md,
docs/architecture/agent-control-ide.md.

Wire awaiting_tool_approval vs awaiting_approval. Stop auto-retry create with
approved:true. Approve must POST /client/v1/agents/runs/{id}/approve.
pendingToolApply only for graph.* local chrome (not Client writes).
Render executed MCP tool names from run output.

Verify: make test-ide
Do not edit Go. Do not add graph.* to MCP.
```

---

### WS2 — Build metadata completeness

**Goal:** Object Manager / Automations / Agents / Webhooks demonstrate Metadata the install already has.

| Panel | Work |
|---|---|
| Object Manager | PATCH field label/required/picklist; DELETE custom field; DELETE custom object (respect 403 managed); unique/indexed checkboxes; **Rebuild projections** → `POST /metadata/v1/projections/{object}/build` + poll GET |
| Automations | On Build rail (WS0). PATCH `active`. **Run now** → `POST /client/v1/automations/{apiName}/runs` + poll `GET .../runs/{id}`. Hide Delete until mux has DELETE. |
| Agents | Replace `TOOL_OPTIONS` with harness tokens from `GET /metadata/v1/agents/harnesses` (union of job-class floors). `allowedSkills` picker from `GET /client/v1/automations` + `GET /client/v1/actions`. |
| Webhooks | New Build tool: `GET/POST /metadata/v1/webhooks`. No PATCH/DELETE until Go adds them. |
| Validation | If object describe/list includes rules, show them. Create via `POST /metadata/v1/validation-rules`. Full manager blocked on API gap. |
| Packages | Detail from `GET /metadata/v1/packages/{name}` (objects even when disabled — Explorer already uses this). |

**Do not:** visual ToolSpec builder; YAML MetadataPanel push-to-org (repo mirror stays best-effort).

**API handoff (optional same-milestone):** `api-families` adds `GET/PATCH/DELETE /metadata/v1/validation-rules`, `DELETE /automations/{apiName}`, webhook item PATCH/DELETE. IDE enables buttons only after those land.

---

### WS3 — Govern AuthZ completeness

**Goal:** Identity admin can exercise shipped AuthZ without curl.

| Panel | Work |
|---|---|
| Permission sets | `fieldPermissions` matrix (object × field × readable/editable) from describe + existing Metadata body (`fieldPermissions` in `metadata_routes.go`) |
| Sharing | New Govern tool: `/metadata/v1/sharing/settings`, enable, objects OWD, rules CRUD (`sharing_routes.go`) |
| Directory | Tags CRUD + assign (`/client/v1/directory-tags`); data-roles (`/client/v1/data-roles`) — can share the Users tool as tabs |
| Experiences | Create/PATCH/DELETE via `/metadata/v1/experiences` (today list-only) |
| Devices | `GET /client/v1/devices` + revoke; enroll already in Connect |
| Account | `POST /client/v1/me/password`; show **product** caps (not first 12 `ide.*`). Do **not** call `/client/v1/preferences`. |

Gate new tools on `authz.manage` / `identity.users` / `identity.manage` — not `ide.govern.*`.

---

### WS4 — Client data operations (demo, not CRM product)

**Goal:** Prove platform actions, ingest, and upsert exist. **Not** a Sales/Service workspace.

| Tool | Binding |
|---|---|
| Object Home / List View | If `GET /client/v1/actions` returns verbs whose `input` matches current object, show **Run action** (e.g. `lead.convert` on Lead). POST body from a small generated form. |
| Object Home | Upsert when the object has an external-id unique field: `POST /sobjects/{object}/upsert` |
| Build **Ingest** inspect tool | Create job → PUT batch → PATCH close → GET results (`/client/v1/jobs/ingest*`) |
| Build **Events** (optional) | `GET /events`, ack unpublished |
| Govern **Audit** (admin) | `GET /client/v1/audit` last-N table |

Keep List View composite bulk as-is. Do not add reporting (BP-021 Frozen).

---

### WS5 — Deploy / Ops honesty

**Goal:** Pipeline chips match worker reality.

| Change | Route |
|---|---|
| After validate-local / promotions, if body has `jobId` / `workId`, poll `GET /deploy/v1/work/{jobId}` until terminal | Already registered |
| Tests: treat `status`, `failed`, `assertions` honestly | `/deploy/v1/tests/runs` |
| Settings Hosting: if session has `ops` (or admin), `GET /ops/v1/upgrades/available` read-only | No mutate in v1 of this WS unless the operator confirms and engine is configured (503 → honest empty) |

Do **not** add DigitalOcean-only chrome ([BP-027](../adr/030-install-agent-runtime.md) Frozen). Existing `DigitalOceanCloudSection` stays as the cloud adapter UI.

Do **not** add promote-to-peer. Env switcher + repeat validate/deploy is the supported multi-env path.

---

### WS6 — Lockstep with BP-065 (do not fight the install)

This workstream is **constraints** plus small IDE edits that land with Go PRs. It is not extra chrome.

| BP-065 phase | IDE rule during this uplift |
|---|---|
| 2 Coaching | After Go drops `oneEffects` inject: IDE Apply of `graph.*` is optional/local or removed. WS1 already stopped treating it as the loop. |
| 3 Chrome island | Prefer **not** to add conversation/graph features. If Phase 3 moves transcripts local, chat persistence becomes Electron store — out of WS1–5. Preferences/canvases: never start using them. |
| 4 `ide.*` | Retarget `scopes.ts` fail-closed to product caps + family scopes; then Go can drop `ide.*` seed. |

New WS2–WS5 tools **must** gate on product caps from day one so Phase 4 does not rewrite them.

---

## Suggested PR slicing

| PR | Content | Unblocks |
|---|---|---|
| 1 | WS0 honesty | Stops demo lies immediately |
| 2 | WS1 hosted loop | Makes agents match the product |
| 3 | WS2 Object Manager + Automations rail + harness tokens | Builder demo |
| 4 | WS3 FLS + sharing + experiences CRUD | Admin demo |
| 5 | WS4 actions + ingest | Headless Client demo in the IDE |
| 6 | WS5 deploy work poll + ops read | Ship twin honesty |
| 7 | WS2 webhooks / projections (can merge with 3) | — |
| * | BP-065 Phases 2–4 | Separate Finish track; this plan must not regress it |

---

## API gaps (handoff, not IDE fiction)

Spawn `api-families` only if a WS is blocked. Cite [agent-api-families.md](./agent-api-families.md).

| Gap | Today | Needed for |
|---|---|---|
| Validation rules list/get/patch/delete | POST create only | Full Object Manager rules |
| Automation DELETE | Missing | Automations panel |
| Webhook item PATCH/DELETE | List/create only | Webhooks tool |
| `/client/v1/debug/*` + ExecutionRun seed | Not registered; BP-033 Phase 3 Open | Real Monitor (keep Frozen chrome; this is **Go**) |

Do **not** register debug routes “for the IDE.” If they ship, they are product Client routes for any `debug.read` principal (MCP/CLI included).

---

## Verification

| Layer | Command |
|---|---|
| Unit / component | `make test-ide` / `cd tools/control-ide && npm test` |
| Live contracts | API up + `ONE_JWT` / `ONE_API_KEY`: `make test-ide-integration` |
| WS1 loop | Integration: create run → park `awaiting_tool_approval` → approve → completed; assert **no** local sobjects write from `pendingToolApply` for that tool |
| WS0 deploy | Fixture report `{ ok: false }` does not set tests Passed |
| Go | Only if an API-gap PR is in the same change set: `go test ./internal/httpapi/...` |

Do not run product `make ci` as the primary check for IDE-only PRs.

---

## Explicit non-goals

- Hosting Control IDE as browser SaaS; embedding it in product images
- License onboarding, private update CDN, premium chrome
- Operate as end-user CRM (layouts, sales pipeline, reporting)
- In-IDE coding agent, Electron LLM, MCP session host
- Deepening `/client/v1/run-graphs` or conversations as product GA
- `requireCapability(ide.*)` on family HTTP
- Replacing `one` as Ship of record
