# BP-052 customer inference — remainder tech design + agentic build plan

**Work-order slot:** 4 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-052](../../../backlog/BP-052-customer-inference.md)
**Track:** Finish
**Status of remainder:** Partial (Phases 0–5 + SSE shipped; **DO model IDs / `modelId` alias landed**; reconnect/cancel remain)
**Domain agents:** `api-families` then `deploy-ops` then `worker-jobs` (Go only)
**Playbooks:** [agent-api-families.md](../agent-api-families.md) · [agent-deploy.md](../agent-deploy.md) · [agent-authz.md](../agent-authz.md) · [agent-worker.md](../agent-worker.md)
**Existing plans (do not duplicate):** [inference-build-plan.md](../inference-build-plan.md) · [deploy-cloud-capability-contract.md](../deploy-cloud-capability-contract.md) · [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) (BP-006 — **do not reopen**) · [ADR-010](../../adr/010-customer-agentic-platform.md) · [ADR-030](../../adr/030-install-agent-runtime.md)

---

## 1. Remainder inventory

Acceptance checkboxes in [inference-build-plan.md](../inference-build-plan.md) are **all still unchecked in the plan file**. Code review against this tree (2026-08-23) shows Phases 0–5 plus SSE/approve/Ollama **shipped**. Hosted tool loop is **BP-006 mitigated** — not a BP-052 remainder. Settings → Inference chrome is **frozen** ([ADR-030](../../adr/030-install-agent-runtime.md)).

| Surface | Shipped (cite packages/tests) | Still open | Evidence |
|---|---|---|---|
| Migration + tables | `install_inference_config`, `install_inference_providers`, `agent_run_events` | None | `migrations/0050_inference.sql` |
| Router + OpenAI client + SSRF | `internal/inference` `Resolve`, `Complete`, `Stream`, egress allowlist, `APP_ENV!=production` loopback Ollama; `DOModeModels` + `modelId` alias | SSE reconnect/cancel (see below) | `internal/inference/client.go`, `models.go`, `store.go`; `client_test.go` (`TestModelForMode`, `TestStatusJSONModelIDAlias`, `TestValidateProviderBaseURLDevAllowsOllama`) |
| Metadata BYO CRUD + config | `GET/POST/PATCH/DELETE /metadata/v1/inference/providers`, `GET/PATCH /metadata/v1/inference/config`; `apiKey` write-only → `install_secrets`; never echoed | Delete/source-switch edge cases may still be thin | `internal/httpapi/inference_routes.go`; `internal/httpapi/inference_routes_test.go` |
| Deploy Native DO | `GET\|PUT /deploy/v1/cloud/inference` + `/cloud/digitalocean/inference` alias; `deploy` / `deploy`+admin; `billingNotice` + `prepaid: true` | Response uses `doModelId` not plan `modelId`; `PutDOConfig(enabled=false)` forces `active_source=none` and can wipe BYO; no HTTP tests | `internal/httpapi/deploy_cloud_routes.go` (`handleCloudInferenceGet`/`Put`); `inference.StatusJSON` |
| Client streaming create | `POST /client/v1/agents/runs` with `stream:true` or `Accept: text/event-stream` generates immediately; no `202 awaiting_approval`; no `agent.run` job | Generation is bound to the POST `r.Context()` — disconnect **aborts** the run (`failRun` → `failed`) | `internal/httpapi/client_extras.go`; `agent_run_stream.go` `streamAgentRunLLM`; `agent_run_stream_test.go` `TestStreamCreateSkipsAwaitingApproval` |
| SSE approve vs JSON approve | JSON approve enqueues one `agent.run`; SSE approve in-process, no job | Same disconnect-kills-generation gap | `handleApproveAgentRun`; `TestApproveJSONEnqueuesJob`; `TestApproveSSEDoesNotEnqueueJob` |
| GET reconnect tail | `GET /client/v1/agents/runs/{id}/stream` polls `agent_run_events` via `afterSeq` query | No SSE `id:` field; no `Last-Event-ID`; no heartbeats; 5‑minute hard deadline; no tests; terminal set omits `cancelled` | `handleStreamAgentRun`; `inference.ListRunEventsAfter` |
| Cancel | Client disconnect cancels in-process `Execute` (as a side effect). Worker has no cancel. No `POST .../cancel` | Explicit cancel + cooperative stop + persist-on-cancel | No route. `agentloop` has no `StatusCancelled`. `failRun` uses the (possibly cancelled) request ctx to persist |
| Worker non-stream | `agent.run` → `agentloop.Execute` with same `Resolve` | Keep `SoftSkipInference` (intentional: unconfigured worker **completes** with `inferenceSkipped` instead of failing the job) | `internal/worker/process.go`; `agentloop.Config.SoftSkipInference` |
| Hosted tool loop | `internal/agentloop` executes MCP tools as run actor; `awaiting_tool_approval`; SSE/JSON approve split | **Out of BP-052.** BP-006 mitigated | `hosted-agent-tool-loop-build-plan.md`; `agent_run_loop_test.go` |
| Settings Inference UI | Optional JWT client + `ide.settings.inference` cap + Test chat | **Frozen.** Do not add chrome | `internal/authz/system_perms.go` `CapIDESettingsInference`; IDE tree out of this remainder |
| Unconfigured HTTP error | Stream create/approve emit `INFERENCE_NOT_CONFIGURED` | Copy still points at Settings | `inference.FormatRouteError`; `TestStreamCreateSkipsAwaitingApproval` |
| Docs Phase 6 | Partial: `docs/api-families.md` Agents row mentions SSE; contract matrix has inference verb | Metadata inference rows missing; Deploy table missing `/cloud/inference`; `agent-api-families.md` silent on inference; plan checkboxes unchecked; `customer-agents.md` still leads with Settings | `docs/api-families.md` §2 Agents vs §3/§4 tables; `docs/architecture/README.md` still “Active” |

**Plan acceptance vs code (do not re-plan shipped rows):**

| Checkbox in inference-build-plan | Code verdict |
|---|---|
| Settings → Inference tool + `ide.settings.inference` | Shipped (optional client). Frozen — remainder does not touch it |
| BYO CRUD stores key; never echoes plaintext | Shipped in handlers. **Add HTTP tests** (remainder) |
| Deploy `PUT .../cloud/inference` enables DO + billing notice | Shipped (`StatusJSON` always includes `billingNotice`/`prepaid`). **Add `modelId` + HTTP tests** |
| `POST /agents/runs` `stream:true` emits tokens when configured | Shipped (`agent_run_loop_test.go` with mock BYO) |
| Streaming create `approved:false` is not `202 awaiting_approval` | Shipped + tested |
| SSE approve in-process, no job | Shipped + tested |
| Settings Test chat | Shipped IDE. Frozen |
| Unconfigured → `INFERENCE_NOT_CONFIGURED` | Shipped on HTTP stream. Worker soft-skips (Keep) |
| Worker uses same router | Shipped |
| Provisional Dev/Standard/Pro IDs in one constants file | **Open** — map lives in `client.go`, not a dedicated file; Standard ID is stale vs current DO catalog |

---

## 2. Detailed design (remainder only)

Cite [ADR-010](../../adr/010-customer-agentic-platform.md) (AgentSpec / MCP / no fourth family) and [ADR-030](../../adr/030-install-agent-runtime.md) (install is SoR; Control IDE optional JWT client). Do not add `/inference/chat/completions`. Do not unfreeze Settings chrome. Do not invent a live DO model catalog ([inference-build-plan.md](../inference-build-plan.md) choice **2C**).

### 2.1 Model ID constants (retune)

**Today:** `var DOModeModels` in `internal/inference/client.go`:

| Mode | Current ID | Current DO Serverless catalog (docs.digitalocean.com/products/inference/details/models, 2026-08) |
|---|---|---|
| `dev` | `openai-gpt-oss-20b` | Still listed (serverless) — **keep** |
| `standard` | `llama3.3-70b-instruct` | **Not listed.** Serverless Meta option is `llama-4-maverick`. Llama 3.1 8B (`llama3-8b-instruct`) is dedicated-only |
| `pro` | `openai-gpt-oss-120b` | Still listed (serverless) — **keep** |

**Design:**

1. Move the map + `ModelForMode` / `ValidateMode` into `internal/inference/models.go` (the “one constants file” the plan already named). `client.go` keeps HTTP client code only.
2. Retune Standard → `llama-4-maverick`. Do **not** add catalog fetch, picker UI, or per-request model override.
3. `StatusJSON` already exposes `doModeModels`. Keep that map as the public retune surface for CLI/MCP/optional IDE.
4. Add JSON alias **`modelId`** (plan contract) next to existing `doModelId`. Both equal `ModelForMode(*cfg.DOMode)` when `doEnabled`. Deploy PUT/GET must include `billingNotice`, `prepaid: true`, `modelId`, `doModelId`, `doModeModels`.
5. Comment in `models.go`: swap IDs here only; no migration (resolved at `Resolve` time, not stored).

AuthZ unchanged: Metadata `metadata` / `metadata.build`; Deploy `deploy` / admin on PUT.

### 2.2 Source-switch + delete polish (Metadata / Deploy)

`active_source` is a singleton. Two bugs remain:

**A. `PutDOConfig(enabled=false)`** (`store.go`) always writes `active_source=none`, even when `default_provider_api_name` is set (operator had flipped to BYO via PATCH, or wants to disable DO without destroying BYO).

Locked behavior:

| Call | `do_enabled` | `active_source` |
|---|---|---|
| `PUT` `{enabled:true, mode}` | `true` | `digitalocean` (requires token, as today) |
| `PUT` `{enabled:false}` | `false` | If current source is `digitalocean`: `byo` when `default_provider_api_name` is non-null, else `none`. If current source is already `byo`/`none`: **leave `active_source` unchanged** |
| `PATCH` `{activeSource:byo, defaultProviderApiName}` | unchanged | `byo` |
| `PATCH` `{activeSource:none}` | unchanged | `none` |
| `PATCH` `{activeSource:digitalocean}` | reject (today) — keep | — |

**B. `DELETE /inference/providers/{apiName}`** when that row is `default_provider_api_name`: FK `ON DELETE SET NULL` leaves `active_source=byo` with a null default → `Resolve` returns `ErrNotConfigured`. After delete, if the deleted name was the default, set `active_source=none` and null the default in the same transaction.

**C. `FormatRouteError`:** replace Settings copy with install SoR:

- `INFERENCE_NOT_CONFIGURED` → `Configure install inference: Metadata /metadata/v1/inference/providers + /inference/config (BYO), or Deploy PUT /deploy/v1/cloud/inference (Native DigitalOcean).`

Do not mention Control IDE Settings in API error strings.

### 2.3 Reconnect

**Problem:** `streamAgentRunLLM` runs `agentloop.Execute(r.Context(), …)`. If the POST SSE client drops, Go cancels that context, `generate`/`Stream` abort, `failRun` marks `failed` (and may fail to persist if the ctx is already done). `GET .../stream` cannot resume a live generation; it only tails rows already in `agent_run_events`.

**Locked behavior:**

```text
POST /agents/runs (stream)  ──► register run ctx (process-local) ──► Execute(runCtx)
                                      │                                    │
                                      ├─ write SSE to this HTTP conn        ├─ persist every event (seq)
                                      └─ HTTP disconnect: stop writing SSE  └─ keep generating until
                                         (do not cancel runCtx)                cancel / complete / fail

GET  /agents/runs/{id}/stream?afterSeq=N | Last-Event-ID: N
                                      └── replay seq>N, then tail until terminal
POST /agents/runs/{id}/cancel
                                      └── cancel runCtx + CAS status=cancelled
```

1. **Run context registry** on `httpapi.Server` (map `runID → context.CancelFunc`, mutex). `Execute` uses `context.WithCancel(context.WithoutCancel(parent))` plus a wall-clock cap (reuse existing 5–15 min generation budget — prefer **15 minutes** for the execute ctx; the HTTP GET stream may be shorter/longer independently).
2. **HTTP disconnect does not cancel generation.** The POST handler still writes SSE while the client is connected; on `r.Context().Done()` it **returns** without calling the registry cancel. Events already persisted remain the reconnect source of truth.
3. **`writeSSE` emits `id: {seq}`** using `AppendRunEvent`’s returned seq (today the seq is discarded). Payload JSON unchanged (`event: token|run|done|error|…`).
4. **`GET .../stream` cursor:** `afterSeq` query **or** `Last-Event-ID` header (integer). `Last-Event-ID` wins if both set and valid. Invalid values → `afterSeq=0`.
5. **Heartbeats:** SSE comment line `: ping\n\n` every 15s while waiting so proxies do not idle-drop. Do not persist pings.
6. **Deadline:** reset the GET-stream idle timer on each persisted event or ping. Cap total GET-stream at 15 minutes; then emit `error` `{code:"STREAM_TIMEOUT"}` and close (**do not** fail the run). Client reconnects with last seq.
7. **Terminal statuses** for GET-stream: `completed`, `failed`, `dry_run_complete`, `awaiting_tool_approval`, **`cancelled`**. `awaiting_tool_approval` already stops the tail (correct — client must POST approve).
8. **Orphan `running`:** if GET-stream sees `status=running`, no registry entry on this process, and no new `agent_run_events` for 30s, emit `error` `{code:"RUN_ORPHANED"}` and CAS the row to `failed` with that message (covers API restart mid-stream). Worker-owned runs stay `running` until the job finishes — detect worker ownership if a live `jobs` row `agent.run` exists for that `runId`; do **not** orphan those.
9. **`ListRunEventsAfter` `LIMIT 500`:** keep; the poll loop already pages. No schema change.
10. **`persistRun` / `AppendRunEvent`:** use `context.WithoutCancel` (or `context.Background()` with a 5s timeout) so a cancelled HTTP ctx cannot skip the terminal UPDATE. This is required for both cancel and disconnect.

AuthZ: `GET .../stream` stays `client` (today). No new capability.

Failure modes:

| Event | Run status | Client |
|---|---|---|
| POST SSE drop | stays `running` until complete/cancel | `GET .../stream?afterSeq=` last id |
| Explicit cancel | `cancelled` + `done` event `{status:"cancelled"}` | both POST and GET see `done` |
| Upstream LLM error | `failed` + `error` event (today) | unchanged |
| API process death | `running` until GET-stream orphan sweep or worker | `RUN_ORPHANED` or worker completion |

Do not add a second inference family. Do not replay tokens by re-calling the LLM.

### 2.4 Cancel

**New route:** `POST /client/v1/agents/runs/{id}/cancel`  
**AuthZ:** `client` scope (same as create/stream). **Not** `govern.agents` (that gates parked-write **approve**, not generation stop). Actor need not be admin. (Runs are not principal-ACL’d on GET today; do not invent ACL here.)

**Body:** empty JSON `{}` optional.  
**Responses:**

- `200` `{id, status:"cancelled"}` when CAS succeeds from `queued` \| `running`.
- `409` `{code:"NOT_CANCELLABLE"}` when status is already terminal or `awaiting_approval` / `awaiting_tool_approval` (those use approve / leave parked). Cancelling a parked write is **out of scope** (BP-006).
- `404` unknown id.

**Effects:**

1. CAS `UPDATE agent_runs SET status='cancelled', completed_at=now() WHERE id=$1 AND status IN ('queued','running')`.
2. Registry `Cancel()` if present (in-process stream).
3. Persist `error` event `{code:"CANCELLED"}` and `done` `{id, status:"cancelled"}` via detached ctx.
4. `agentloop.Execute` / `inference.Client.Stream`: check `ctx.Err()` on each chunk; on cancel, **do not** call `failRun`. If CAS already wrote `cancelled`, return `context.Canceled` and skip overwrite. If generate observes cancel before CAS (race), still CAS to `cancelled`, never `failed`.
5. Worker `agent.run`: between tool rounds and inside `Stream` via ctx, stop. Also `SELECT status` before each `generate` call; if `cancelled`, return without `failRun`. JSON-queued jobs that have not started: CAS in the handler is enough (job later no-ops if status is not `queued`/`running` — add that guard at the top of the worker case).

Add `agentloop.StatusCancelled = "cancelled"`. No migration (`agent_runs.status` is unconstrained text).

### 2.5 API remainders (tests + contract)

There is **no** HTTP test for Metadata inference or Deploy cloud inference. Add `internal/httpapi/inference_routes_test.go` (DB harness via `internal/testutil`, `LockInferenceConfig`):

| Case | Expect |
|---|---|
| `POST /metadata/v1/inference/providers` with `apiKey` | `201`, `hasSecret:true`, body has **no** `apiKey`; secret row `inference.{apiName}` |
| `GET /metadata/v1/inference/providers` | no ciphertext / no `apiKey` |
| `PATCH .../config` `{activeSource:digitalocean}` | `400` |
| `PATCH .../config` `{activeSource:byo, defaultProviderApiName}` | `200`, `activeSource=byo` |
| `PUT /deploy/v1/cloud/inference` without token | `400 DO_TOKEN_MISSING` |
| `PUT` with token + `{enabled:true, mode:"standard"}` | `200`, `prepaid:true`, `billingNotice` non-empty, `modelId` = Standard constant, `doModeModels.standard` same |
| `PUT {enabled:false}` after BYO default exists | does **not** clear `activeSource=byo` |
| `DELETE` default provider | `activeSource=none` |
| Non-`metadata.build` key mutating providers | `403` |
| Non-admin `PUT` cloud inference | `403` |

SSE reconnect/cancel tests in `agent_run_stream_test.go` (extend, do not replace existing approve tests).

### 2.6 Docs (same Finish track, no chrome)

Tick shipped checkboxes in `inference-build-plan.md`; point remainders at **this** file. Add Metadata inference + Deploy `/cloud/inference` rows to `docs/api-families.md`. Mention inference routes in `agent-api-families.md` §B. Rewrite the Settings-first sentence in `docs/customer-agents.md` to Metadata/Deploy SoR + optional IDE. `docs/architecture/README.md` inference row: Phases 0–5 shipped; remainder this file.

---

## 3. Concrete agentic build plan

### Phase 1 — Model IDs + Metadata/Deploy contract polish

- **Owner:** `api-families` (+ `deploy-ops` for PUT semantics only)
- **Packages allowed:** `internal/inference/**`, `internal/httpapi/inference_routes.go`, `internal/httpapi/deploy_cloud_routes.go`, `internal/httpapi/inference_routes_test.go` (new), `internal/inference/client_test.go`
- **Packages forbidden:** `tools/control-ide/**`, `internal/agentloop/**` (except if `FormatRouteError` tests need none), `internal/mcp/**`, hosted-loop files
- **Files likely to change:** `internal/inference/models.go` (new), `internal/inference/client.go` (move map out), `internal/inference/store.go` (`PutDOConfig`, delete-default), `internal/inference/events.go` (`modelId` alias + error copy), `internal/httpapi/inference_routes.go` (delete-default transaction), `internal/httpapi/inference_routes_test.go`
- **Tests:** `go test ./internal/inference ./internal/httpapi -count=1` (needs `DATABASE_URL` for HTTP). Extend `TestModelForMode` for `llama-4-maverick`.
- **Exit criteria:** Standard resolves to `llama-4-maverick`; PUT/GET JSON includes `modelId`; disabling DO does not wipe BYO; deleting the default provider sets `none`; `INFERENCE_NOT_CONFIGURED` copy has no “Settings”; HTTP tests above green.
- **Dependencies:** none. Not BP-006.

### Phase 2 — Reconnect + cancel (Go)

- **Owner:** `api-families` then `worker-jobs`
- **Packages allowed:** `internal/httpapi/agent_run_stream.go`, `internal/httpapi/client_extras.go`, `internal/httpapi/metadata_routes.go` (register cancel), `internal/httpapi/agent_run_stream_test.go`, `internal/inference/events.go` (seq in SSE helper if moved), `internal/inference/client.go` (`Stream` `ctx.Err()`), `internal/agentloop/loop.go`, `internal/agentloop/persist.go`, `internal/worker/process.go`
- **Packages forbidden:** `tools/control-ide/**`. Do not change hosted catalog, `mcp.CallTool`, or `awaiting_tool_approval` semantics.
- **Files likely to change:** as above; small `Server` registry field in `internal/httpapi/server.go`
- **Tests to add:**
  - GET `/stream?afterSeq=` replays persisted events, honors `Last-Event-ID`
  - SSE frames include `id: <seq>`
  - POST stream: cancel HTTP client **without** `POST /cancel` → run stays `running` (or reaches `completed` if mock LLM is instant); GET `/stream` still tails
  - `POST .../cancel` on `running` → `cancelled`, no `failed`, worker job count 0 for stream creates
  - Cancel of `awaiting_tool_approval` → `409 NOT_CANCELLABLE`
  - Worker: job with status already `cancelled` is a no-op
  - `failRun`/`persistRun` still writes when the HTTP ctx is cancelled (unit-level if easier)
- **Exit criteria:** disconnect ≠ fail; cancel is explicit; reconnect by seq; worker respects cancel; existing approve tests still pass (`TestApproveJSONEnqueuesJob`, `TestApproveSSEDoesNotEnqueueJob`, `TestStreamCreateSkipsAwaitingApproval`).
- **Dependencies:** Phase 1 optional (can parallel). **Not** BP-006. Parked-write cancel stays BP-006.

### Phase 3 — Docs (no product code)

- **Owner:** `api-families` (docs only)
- **Packages allowed:** `docs/architecture/inference-build-plan.md` (checkboxes + pointer here), `docs/api-families.md`, `docs/architecture/agent-api-families.md`, `docs/customer-agents.md`, `docs/architecture/README.md`, `backlog/BP-052-customer-inference.md` (status)
- **Forbidden:** `backlog/README.md` unless the closer also de-risks the table in a later PR; `tools/control-ide/**`
- **Exit criteria:** shipped checkboxes marked; remainder bullets point here; Metadata/Deploy inference listed in api-families; customer-agents leads with install APIs.
- **Dependencies:** after Phase 1–2 land, or same PR if those phases are done.

---

## 4. Explicit non-goals

- Hosted tool loop, native `tools` / `tool_calls` admission, `awaiting_tool_approval` (BP-006 mitigated — [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md))
- New Settings → Inference chrome, Test chat UI, or Electron SSE client changes (frozen; ADR-030)
- Live DO model catalog / picker; per-run model override; multi-provider failover
- Dedicated GPU Inference; product-side inference billing
- `/inference/chat/completions` proxy family
- Cancelling parked writes (`awaiting_approval` / `awaiting_tool_approval`)
- Principal ACL on agent runs
- Control IDE reconnect/cancel UX (clients may use GET `/stream` + POST `/cancel` later; not this remainder)

---

## 5. Agentic implementation prompt(s)

### Prompt A — Phase 1 (model IDs + Metadata/Deploy contract) — **Keep (in tree)**

Do not paste this prompt. `DOModeModels`, `modelId` alias, and `inference_routes_test.go` shipped. Next executable remainder is **Prompt B (SSE reconnect + cancel)**.

### Prompt A — Phase 1 (model IDs) — historical

```text
You are a Majesta One Go domain agent (`api-families` + `deploy-ops`). Implement BP-052 remainder Phase 1 only: model ID constants + Metadata/Deploy contract polish. Do not implement reconnect/cancel. Do not edit tools/control-ide. Do not reopen BP-006 hosted tool loop.

Read first:
- docs/architecture/agentic-remainders/04-bp-052-customer-inference.md (§2.1, §2.2, §3 Phase 1)
- docs/architecture/inference-build-plan.md (locked decisions 2C; do not duplicate shipped phases)
- docs/architecture/deploy-cloud-capability-contract.md (inference verb)
- docs/architecture/agent-api-families.md, docs/architecture/agent-deploy.md
- ADR-010, ADR-030
- backlog/BP-052-customer-inference.md
- internal/inference/client.go (DOModeModels), store.go (PutDOConfig, DeleteProvider), events.go (StatusJSON, FormatRouteError)
- internal/httpapi/inference_routes.go, deploy_cloud_routes.go

Edit scope (only):
- internal/inference/** (add models.go; move DOModeModels / ModelForMode / ValidateMode / BillingNotice if it keeps the constants file the single retune point)
- internal/httpapi/inference_routes.go (delete-default → active_source=none in same path)
- internal/httpapi/deploy_cloud_routes.go only if PUT must call updated PutDOConfig (prefer keep handler thin)
- tests: internal/inference/client_test.go; new internal/httpapi/inference_routes_test.go using internal/testutil (LockInferenceConfig, RequireDatabase, NewTestServer)

Do:
1. Retune Standard to llama-4-maverick; keep Dev=openai-gpt-oss-20b, Pro=openai-gpt-oss-120b.
2. StatusJSON: add modelId alias equal to doModelId; keep doModeModels.
3. PutDOConfig(enabled=false): do not wipe BYO active_source (see remainder §2.2 table).
4. DELETE default BYO provider: set active_source=none.
5. FormatRouteError INFERENCE_NOT_CONFIGURED: Metadata/Deploy paths, no “Settings”.
6. HTTP tests listed in remainder §2.5.

Out of scope: tools/control-ide/**, agentloop cancel/reconnect, POST /cancel, GET /stream Last-Event-ID, live DO catalog, /inference/chat/completions, backlog/README.md.

Tests: go test ./internal/inference ./internal/httpapi -count=1
Exit: Phase 1 exit criteria in the remainder doc.
```

### Prompt B — Phase 2 (reconnect + cancel, Go only)

```text
You are a Majesta One Go domain agent (`api-families` then `worker-jobs`). Implement BP-052 remainder Phase 2 only: agent-run SSE reconnect + explicit cancel. Do not add Control IDE chrome. Do not change hosted tool-loop admission, MCP catalog, or awaiting_tool_approval (BP-006).

Read first:
- docs/architecture/agentic-remainders/04-bp-052-customer-inference.md (§2.3, §2.4, §3 Phase 2)
- docs/architecture/inference-build-plan.md (Client SSE contract; GET .../stream reconnect)
- docs/architecture/hosted-agent-tool-loop-build-plan.md (do not reopen; reuse Execute)
- docs/architecture/agent-api-families.md, docs/architecture/agent-worker.md, ADR-030
- internal/httpapi/agent_run_stream.go, client_extras.go (create/approve), metadata_routes.go (run routes)
- internal/agentloop/loop.go, persist.go
- internal/inference/client.go Stream(); events.go AppendRunEvent
- internal/worker/process.go agent.run case
- existing tests: internal/httpapi/agent_run_stream_test.go (must keep all current asserts)

Edit scope:
- internal/httpapi/agent_run_stream.go, client_extras.go, metadata_routes.go (register POST .../cancel), server.go (run-cancel registry field)
- internal/httpapi/agent_run_stream_test.go (extend)
- internal/inference/client.go (ctx.Err() in Stream loop), events.go if writeSSE/seq helper lives here
- internal/agentloop/loop.go (StatusCancelled; do not failRun on cancel), persist.go (WithoutCancel persist)
- internal/worker/process.go (no-op if run already cancelled; honor ctx)

Do (remainder §2.3–2.4):
1. Detach Execute from POST r.Context(); disconnect must not fail the run.
2. SSE id: {seq} from AppendRunEvent; GET /stream honors afterSeq and Last-Event-ID; pings every 15s; idle deadline resets; include cancelled as terminal.
3. POST /client/v1/agents/runs/{id}/cancel, scope client, CAS queued|running → cancelled; 409 NOT_CANCELLABLE for parked/terminal.
4. Orphan running only when no in-process registry AND no live jobs.agent.run row.
5. Preserve: stream create skips awaiting_approval; SSE approve does not enqueue; JSON approve enqueues.

Out of scope: tools/control-ide/**, Metadata/Deploy model-ID retune (Phase 1), BP-006 tool execution changes, cancel of awaiting_tool_approval, new API family, backlog/README.md.

Tests: go test ./internal/httpapi ./internal/agentloop ./internal/inference ./internal/worker -count=1
Exit: Phase 2 exit criteria in the remainder doc. Existing approve/stream tests still pass.
```
