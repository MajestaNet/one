# Customer runtime isolation — remainder tech design + agentic build plan

**Work-order slot:** 8 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md)
**Track:** Finish
**Status of remainder:** Phase 1 shipped (admission lanes + async repo→org Deploy). Phases 2–5 remain.
**Domain agents:** `api-families` · `deploy-ops` · `worker-jobs` · `db-backend-perf` · `authz-security`
**Playbooks:** [agent-worker.md](../agent-worker.md) · [agent-api-families.md](../agent-api-families.md) · [agent-data-architecture.md](../agent-data-architecture.md) · [agent-deploy.md](../agent-deploy.md) · [agent-authz.md](../agent-authz.md)
**Existing plans (do not duplicate):** [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) (design SoT) · [ADR-013](../../adr/013-high-volume-flexible-storage.md) · [ADR-014](../../adr/014-customer-code-automations.md) · [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md)

---

## 1. Remainder inventory

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| Phase 0 spec + locked env names | Docs + BP-033 | — | [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) Phase 0; knob table in that plan |
| API admission lanes (`client` / `metadata` / `deploy` / `auth`) | Split limiters + `ADMISSION_CLIENT_RPM_SHARE`; `/auth/v1` not double-counted; MCP classified by tool | — | `internal/httpapi/admission.go`, `middleware.go`; `internal/config` `AdmissionClientRPMShare` |
| Async org validate / apply | Size-gated 202 + `DEPLOY_BUSY` + worker `deploy.validate` / `deploy.apply`; `GET /deploy/v1/work/{jobId}`; CLI/MCP poll | Phase 2 slot claim; Phase 3 `executionRunId` | `internal/deploy/async.go`; `internal/httpapi/deploy_routes.go`; `internal/worker/process.go`; `internal/mcp/builder.go`; `cmd/one/org.go` |
| Worker execution classes + slot caps | FIFO claim by `run_at`; **no** `jobs.class` | Per-class SKIP LOCKED claim + `JOB_SLOTS_*` | `migrations/0000_kernel.sql` `jobs` (`job_type`, no `class`); `migrations/0006_cache_and_worker_leases.sql` `FOR UPDATE SKIP LOCKED` + `locked_at` / `locked_by`; `internal/worker/claim.go` `ClaimJobs` (pending + `run_at`, no class predicate); `internal/worker/process.go` `ProcessJobs` claims one-at-a-time. **Do not confuse** with AgentSpec harness `job_class` (`query\|customize\|ship\|govern\|operate\|skill`) on `agent_playbooks` — `migrations/0059_agent_job_class.sql` / BP-064 |
| Daily quotas | — | `install_quota_counters` + `QUOTA_*` | No table, no `QUOTA_EXCEEDED` in Go; grep hits only the isolation plan / BP-033 |
| Deno RSS / raised async wall | Sync 5s / async 30s wall; mutation 50 / depth 3 | `DENO_RSS_LIMIT_MB`; optional 120s async opt-in | `internal/automation/deno.go` `DefaultAsyncDeadline = 30s`; `internal/automation/sync_exec.go` `SyncDeadline` / `MaxSyncDepth` / `MaxSyncOps`; no `Setrlimit` / cgroup RSS |
| Hosted agent budgets | Loop shipped (BP-006) | Count tool calls into class quota + `Stats.toolCalls` | `internal/agentloop/loop.go` `processCalls`; `internal/worker/process.go` `case "agent.run"` — no daily/slot quota |
| `ExecutionRun` / `ExecutionLogEntry` | Planned in docs only | Seed + HV + writers + Client read | No `platform_debug` in `internal/seed` / `internal/packages`; `docs/data-model.md` “Planned debug objects”; HV path exists for `messages` (`internal/seed/module_messages.go`, `internal/dataengine/storage.go` guardrails, `migrations/0019_high_volume_records.sql`) |
| `debug.read` capability string | Constant + Admin union | Gate Client reads of debug objects; `DebugViewer` PS | `internal/authz/system_perms.go` `CapDebugRead`; `CanonicalCapabilities` includes it so Admin seed has it (`internal/db/system_admin.go`); `migrations/0045_ide_caps_fls_freeze.sql` unions `debug.read` onto Admin. **No** HTTP `requireCapability(CapDebugRead)` and **no** `DebugViewer` in `internal/seed/seed.go` `capabilityPermissionSetDefs` |
| Guest `ctx.log` | Single `ctx.log(...)` → host `Logger` / in-memory `Logs` | `ctx.log.info/warn/error` → HV lines + cap | `internal/automation/embed/bootstrap.ts` `log: (...a) => emit({ kind: "log", ...})`; `internal/automation/deno.go` case `"log"`; `docs/automation-sdk.md` |
| Retention / purge of log lines | jobs/outbox/audit_log only | `execution_log.purge` + `EXECUTION_LOG_RETENTION_DAYS` | `internal/worker/retention.go`; `internal/config/config.go` `RetentionJobsDays` / `Outbox` / `AuditLog` |
| Phase 4 IDE Monitor | Control IDE chrome **frozen** | Docs-only debug recipe | [BP-034](../../adr/030-install-agent-runtime.md) Status Frozen; do **not** edit `tools/control-ide/**` |
| Phase 5 elastic knobs + OTEL | OTEL exporter exists (BP-008 partial) | Document replica×slot math; optional metrics | `internal/otel`; no queue-depth / throttle / Deno-OOM meters |

**Honest summary:** Phase 1 (admission lanes + async repo→org Deploy) is in product Go. Phase 2–3 (job classes, quotas, ExecutionRun) have not landed. BP-005 lease/SKIP LOCKED **must stay**; later remainders add classes and quotas **on top**.

---

## 2. Detailed design (remainder only)

Locked product decisions, default budget table, and object field lists live in [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md). This section is the **current-tree** contract: what to add, where, and how it fails.

### 2.1 Vocabulary (do not collide)

| Term | Meaning | Existing SoR |
|---|---|---|
| **Execution class** (`jobs.class`) | Worker admission bucket: `automation` \| `agent` \| `deploy` \| `default` | This BP (not yet in SQL) |
| **Harness job class** | AgentSpec floor: `query` \| `customize` \| `ship` \| `govern` \| `operate` \| `skill` | BP-064 / `agent_playbooks.job_class` / `internal/agentharness` |

Never reuse harness job-class strings as `jobs.class` values. Map `job_type` → execution class in Go (single function, tested).

### 2.2 Admission lanes (Phase 1)

**Today:** `Handler()` in `internal/httpapi/middleware.go` applies one `RATE_LIMIT_PER_MINUTE` bucket keyed by client IP (with private-proxy XFF rules). `/auth/v1/token` uses `AUTH_TOKEN_RATE_LIMIT_PER_MINUTE` already. Family prefixes are registered in `internal/httpapi/server.go` + `*_routes.go`.

**Remainder:** classify each request **before** the global limiter:

| Lane | Path prefix (after API revision strip) | Budget |
|---|---|---|
| `client` | `/client/v1`, `/v1` aliases that are Client | `ceil(RATE_LIMIT_PER_MINUTE * ADMISSION_CLIENT_RPM_SHARE)` (default `0.7`) |
| `metadata` | `/metadata/v1` | share of remainder |
| `deploy` | `/deploy/v1` | share of remainder |
| `auth` | `/auth/v1` | keep existing `authTokenLimiter` (do not double-count) |
| `ops` | `/ops/v1` | remainder (same pool as metadata/deploy unless a later ADR splits it) |
| probes | `/healthz`, `/readyz`, `/version` | unlimited (already skipped) |

**`/mcp`:** classify by tool family when the JSON-RPC body names a tool (`org_validate` / `org_deploy` / `pack` / `org_retrieve` → `deploy`; Metadata upserts → `metadata`; Client tools → `client`). If body parse fails, charge **`deploy`** (conservative: cannot starve Client). Do not invent an MCP-only lane.

**429 shape:** keep `error: RATE_LIMITED` for RPM; add `details.lane`. Use `writeErrDetails` (`internal/httpapi/server.go`).

**Config:** add `AdmissionClientRPMShare float64` to `internal/config/config.go` (env `ADMISSION_CLIENT_RPM_SHARE`, default `0.7`, clamp `(0,1]`). Document in `docs/tech-stack.md`, `docs/security.md`, `.env.example`.

### 2.3 Async Deploy by default (Phase 1)

**Repo→org only.** Peer/inbound promote stays removed (`handlePromotions` already rejects `artifact`). No change to `multi-env-deploy.md` topology.

**Expensive work (must not run on the API goroutine when over the size gate):**

| HTTP | Engine today | Remainder |
|---|---|---|
| `POST /deploy/v1/packages/validate-local` | `DeployEngine.ValidateLocal` sync (`handlePackageValidateLocal`) | Size gate → enqueue `deploy.validate`, **202** + `jobId` (and `bundleId` if already stored) |
| `POST /deploy/v1/bundles/{id}/validate` | `ValidateBundle` sync | Same job type; 202 when over gate |
| `POST /deploy/v1/promotions` | `PromoteBundle` validates+applies in-request | Insert `deploy_promotions` `pending`, enqueue `deploy.apply`, 202; poll `GET /deploy/v1/promotions/{id}` (already exists) |
| `POST /deploy/v1/tests/runs` | Sync unless `async: true` | Default **async** when step count exceeds gate; 429 `DEPLOY_BUSY` when deploy class saturated **and** queue depth ≥ `DEPLOY_QUEUE_MAX` |

**Size gate (locked remainder knobs):**

| Env | Default | Meaning |
|---|---|---|
| `DEPLOY_SYNC_MAX_FILES` | `50` | Metadata+src file count in the pack |
| `DEPLOY_SYNC_MAX_BYTES` | `2097152` | Artifact JSON/zip bytes |
| `DEPLOY_QUEUE_MAX` | `8` | Pending+running `class=deploy` jobs; extra mutates → `429 DEPLOY_BUSY` |
| `JOB_SLOTS_DEPLOY` | `1` | Concurrent running deploy-class jobs (enforced in Phase 2; Phase 1 may count `jobs` by `job_type IN (...)` until the column exists) |

Tiny packs **under** the gate stay **200/201** with the current result body so `one org validate` and MCP tight loops remain snappy.

**202 body (until Phase 3 objects exist):**

```json
{
  "accepted": true,
  "status": "queued",
  "jobId": "<uuid>",
  "bundleId": "<uuid>",
  "poll": "/deploy/v1/work/{jobId}"
}
```

Add `GET /deploy/v1/work/{jobId}` (Deploy scope): returns `{ jobId, jobType, status, lastError, result }` **only** for `deploy.validate` / `deploy.apply` / `customer.test.run` created on this install. Do not expose `automation.run` / `agent.run` through this route.

**`DEPLOY_BUSY`:** `429` with `error: DEPLOY_BUSY`, message that org validate/deploy/test is at `JOB_SLOTS_DEPLOY` and queue depth limit. Never wait on Client threads.

**CLI/MCP lockstep (required so shipped DX does not break):** `cmd/one/org.go` must treat HTTP 202 as “poll `poll` until status completed/failed” (timeout ≥ worker lease). `internal/mcp/builder.go` `orgValidate` / `orgDeploy` must return the 202 handle (jobId + poll) rather than blocking the MCP HTTP request on a full apply. Do **not** add Control IDE chrome.

**Worker handlers (Phase 1 may land stubs that run the existing engine):** `internal/worker/process.go` new cases `deploy.validate` and `deploy.apply` calling `DeployEngine.ValidateLocal` / `ValidateBundle` / `PromoteBundle` (or a extracted apply-from-pending-promotion helper in `internal/deploy/service.go`). `customer.test.run` already exists. Idempotent: if promotion/validation already terminal, complete the job.

### 2.4 Execution classes, slots, quotas (Phase 2)

**Schema (next kernel migration after `0059_agent_job_class` — `0060_…`):**

1. `jobs.class text NOT NULL DEFAULT 'default'` with `CHECK (class IN ('automation','agent','deploy','default'))`.
2. Index `(status, class, run_at)` for class-aware claims (keep `jobs_claim_idx` / SKIP LOCKED semantics).
3. Kernel table `install_quota_counters (day date NOT NULL, class text NOT NULL, count bigint NOT NULL DEFAULT 0, PRIMARY KEY (day, class))`.

**`job_type` → class (locked):**

| `job_type` | `class` |
|---|---|
| `automation.run` | `automation` |
| `agent.run` | `agent` |
| `deploy.validate`, `deploy.apply`, `customer.test.run` | `deploy` |
| `ingest.process`, `search.reindex`, `sharing.recalc`, `projection.build`, `hv.partition.roll`, `retention.purge`, `execution_log.purge` | `default` |

Enqueue sites must set `class` (or a helper `worker.Enqueue(ctx, pool, jobType, payload)` derives it). Today’s `INSERT INTO jobs (job_type, payload)` call sites: `internal/dataengine/sync_automations.go`, `internal/httpapi/client_automation_runs.go`, `internal/httpapi/client_extras.go`, `internal/mcp/gateway.go`, `internal/deploy/service.go`, `internal/dataengine/ingest.go`, `internal/metadata/write.go`, `internal/db/sharing.go`, `internal/mcp/builder.go`.

**Claim (do not regress BP-005):**

- Keep `UPDATE … WHERE id IN (SELECT … FOR UPDATE SKIP LOCKED)` and `locked_at` / `locked_by` / lease reclaim / heartbeat (`claim.go`, `process.go` `runClaimedJobWithLease`).
- Add `ClaimJobsOfClass(ctx, pool, workerID, class, limit, maxRunning)` that refuses to claim when `count(*) FILTER (status='running' AND class=…)` ≥ `JOB_SLOTS_*` **inside the same statement** (subquery or CTE), so two workers cannot both oversubscribe.
- `ProcessJobs` order per tick: **`default` first** (Client-critical ingest/search/sharing), then `automation`, `agent`, `deploy` up to their slot caps. Deploy/automation/agent must not starve `default`.
- Install-wide slots (not per-worker): `JOB_SLOTS_AUTOMATION` default 4, `JOB_SLOTS_AGENT` default 2, `JOB_SLOTS_DEPLOY` default 1. Document `min(env, replicas*k)` in Phase 5; v1 may treat env as the hard cap.

**Daily quotas:** `INSERT INTO install_quota_counters (day, class, count) VALUES (CURRENT_DATE, $class, 1) ON CONFLICT (day, class) DO UPDATE SET count = install_quota_counters.count + 1 RETURNING count`. If `count > QUOTA_*` → fail job with `QUOTA_EXCEEDED` (do not run Deno/loop). `default` class has **no** daily quota. UTC day is fine (install-local).

**Deno remainder:** keep sync 5s / 50 mutations / depth 3. Raise **async** wall only via `AUTOMATION_ASYNC_DEADLINE_SECONDS` (default **30** until operators opt in to 120 — isolation plan). Apply RSS soft limit: `syscall.Setrlimit(RLIMIT_AS)` / `RLIMIT_DATA` when available; on failure log and continue with wall timeout. Stream guest logs into a byte buffer capped by `EXECUTION_LOG_BYTES_PER_RUN` (truncate + `truncated=true` on the run once Phase 3 exists).

**Agent remainder:** hosted loop already runs as `agent.run`. Increment automation-style daily `agent` counter at claim (or first tool call). Record `toolCalls` into ExecutionRun `Stats` in Phase 3. Do not block BP-006 behavior on missing ExecutionRun — quotas still apply.

### 2.5 Debug objects (Phase 3)

**Package:** always-on managed module `platform_debug` (`Optional: false`), same install path as `agents_starter`: `packages.Register` in `internal/seed/module_platform_debug.go`, `InstallPlatformDebug` from `internal/seed/seed.go` after `InstallCore`. `packages.IsManagedPackageName` picks it up automatically — Deploy must reject customer packs that try to ship it.

**Objects (field lists already locked in the isolation plan):**

- `ExecutionRun` — `storage_mode=flexible`; indexed `JobId`, `Kind`, `Status`, `CorrelationId`, `ActorId`.
- `ExecutionLogEntry` — `storage_mode=high_volume`; indexed lookup `ExecutionRunId`; `Seq`; DataEngine HV guardrails already require Id / time bound / indexed filter (`internal/dataengine/storage.go`). Client queries **must** predicate `ExecutionRunId` and/or `CreatedAt` range.

Ensure HV LIST partition via `db.EnsureHighVolumePartition` on enable (same as Message). RANGE roll is existing `hv.partition.roll`.

**AuthZ:**

- Writers: **platform only**. Customer `POST/PATCH/DELETE /client/v1/sobjects/ExecutionRun|ExecutionLogEntry` → `403` (object create/update/delete denied for all non-platform actors). Implement a small `internal/execution` writer used by worker/automation/deploy that inserts via DataEngine **without** customer CRUD (system writer / modify-all internal path). Do not let guests SQL the tables.
- Readers: Client GET/query requires **object read** **and** `debug.read` (`CapDebugRead`). Admin already has the cap. Seed optional PS **`DebugViewer`** (`debug.read` only) in `capabilityPermissionSetDefs`.
- `debug.trace` / TraceFlag stay BP-034 backend remainder — out of this phase unless a later prompt says otherwise.

**Writers (at-least-once safe):** upsert ExecutionRun by `JobId` (unique projection / indexed). Append log lines with monotonic `Seq` per run. Idempotent job retries must not duplicate the run row; extra log lines are acceptable if Seq is unique (`ExecutionRunId+Seq`).

Wire:

| Producer | Kind | Files |
|---|---|---|
| `automation.run` + sync guest | `automation` | `internal/worker/automation_run.go`, `internal/dataengine/sync_automations.go`, `internal/automation/deno.go` Logger |
| `agent.run` | `agent` | `internal/worker/process.go` + `internal/agentloop` (tool call count in `Stats`) |
| `deploy.validate` / `deploy.apply` / `customer.test.run` | `deploy_validate` / `deploy_apply` / `customer_test` | `internal/worker/process.go`, `internal/deploy` |
| Guest SDK | log lines | `embed/bootstrap.ts` + `one_automation.ts`: add `log.info` / `log.warn` / `log.error`; keep `log(...)` as info alias (`docs/automation-sdk.md`) |

**Retention:** job type `execution_log.purge` (class `default`) from `internal/worker/retention.go` / `loop.go` `maybeRunRetention`. Hard-delete HV rows older than `EXECUTION_LOG_RETENTION_DAYS` (default 14) with `CreatedAt` bound (ADR-013). Prefer DETACH of aged RANGE partitions when the Message HV roll already created them; otherwise batched `DELETE` with time predicate.

**202 + objects:** Phase 3 202 bodies add `executionRunId`. Keep `jobId` for compatibility.

### 2.6 Phase 4 / 5 (docs; IDE frozen)

**Phase 4:** docs-only UX. Update [customer-developer-workflow.md](../../customer-developer-workflow.md) with a debug recipe (`debug.read`, query ExecutionRun, page ExecutionLogEntry with `ExecutionRunId` + time). Update [automation-sdk.md](../../automation-sdk.md) for `ctx.log.*`. **Do not** edit `tools/control-ide`. Operate Monitor IDE is [BP-034](../../adr/030-install-agent-runtime.md) **Frozen**.

**Phase 5:** document how App Platform/Helm worker replica count interacts with slot defaults; optional admin-only Metadata/Deploy exposure of quota knobs (install-local, not multi-tenant metering). Pair OTEL meters (queue depth, `DEPLOY_BUSY` count, `QUOTA_EXCEEDED`, Deno OOM) with BP-008 — do not replace ExecutionLogEntry with operator spans.

### 2.7 Failure modes

| Condition | Code | Behavior |
|---|---|---|
| Client RPM exhausted | `429 RATE_LIMITED` | Deploy/Metadata still have their lanes; Client does not borrow them |
| Deploy/Metadata RPM exhausted | `429 RATE_LIMITED` | Client lane unaffected |
| Deploy slots + queue full | `429 DEPLOY_BUSY` | No extra `deploy.*` jobs |
| Daily automation/agent cap | job `failed` / `QUOTA_EXCEEDED`; ExecutionRun `throttled` when objects exist | Guest does not start |
| Deno wall / RSS | job failed; run `failed` + error entry | Sync path still rolls back the write tx (ADR-014) |
| Lease lost mid-run | existing heartbeat fencing | At-least-once; writers upsert by JobId |
| HV query without selective filter | existing DataEngine validation error | Do not raise `HighVolumeLocatorMaxRows` to “fix” debug scans |
| Customer invents ExecutionRun | `403` on create | Platform writer only |

---

## 3. Concrete agentic build plan

### Phase 1 — Admission lanes + async Deploy

- **Owner:** `api-families` then `deploy-ops` (CLI poll: `deploy-ops` may touch `cmd/one`)
- **Packages allowed:** `internal/httpapi`, `internal/config`, `internal/deploy`, `internal/worker` (new `deploy.*` process branches only), `internal/mcp` (return 202 handles), `cmd/one` (poll 202), `docs/tech-stack.md`, `docs/security.md`, `.env.example`
- **Packages forbidden:** `tools/control-ide/**`; `internal/seed` objects (Phase 3); do not change SKIP LOCKED claim SQL except to enqueue new job types
- **Files likely to change:** `internal/httpapi/middleware.go`, `middleware_test.go`, `deploy_routes.go`, `validate_local_test.go`, `server.go` (`writeErrDetails` callers); `internal/config/config.go`, `config_test.go`; `internal/deploy/service.go`, `validate_local.go`, `validate.go`, `apply.go`; `internal/worker/process.go`, `process_test.go`; `internal/mcp/builder.go`, `gateway.go`; `cmd/one/org.go`; `.env.example`
- **Tests:** `go test ./internal/httpapi/... ./internal/config/... ./internal/deploy/... ./internal/worker/... ./internal/mcp/...` plus `go test ./cmd/one/...` if org poll is added. Prefer `internal/testutil` for HTTP+DB. Cover: Client lane still allows when Deploy lane is saturated; tiny validate stays 200; large validate 202; `DEPLOY_BUSY` at queue cap; claim still SKIP LOCKED (`claim_test.go` must stay green).
- **Exit criteria:** Under concurrent Deploy validate, Client CRUD is not charged to the Deploy RPM lane; expensive validate/apply returns 202 and completes on the worker; `one org validate` still prints a report (via sync tiny path or poll).
- **Depends on:** BP-005 mitigated (leases). BP-032/BP-048 are consumers. Does **not** wait for Phase 3 objects (`jobId` is enough).

### Phase 2 — Execution classes + quotas + Deno RSS

- **Owner:** `worker-jobs`
- **Packages allowed:** `internal/worker`, `internal/automation`, `internal/config`, `migrations/` (+ `migrations/meta/_journal.json`), enqueue call sites listed in §2.4, `cmd/worker` if config wiring is needed
- **Packages forbidden:** `tools/control-ide/**`; do not replace Postgres jobs with an external queue; do not alter `agent_playbooks.job_class`
- **Files likely to change:** `migrations/0060_job_execution_class_quotas.sql` (name may shift if 0060 is taken — next free number); `internal/worker/claim.go`, `claim_test.go`, `process.go`, `process_test.go`, `loop.go`, `automation_run.go`; `internal/automation/deno.go`; `internal/config/config.go`; enqueue `INSERT INTO jobs` sites
- **Tests:** `go test ./internal/worker/... ./internal/automation/... ./internal/config/...`. Must include: two workers cannot exceed `JOB_SLOTS_AUTOMATION`; `default` still claims while deploy class is saturated; daily cap fails with `QUOTA_EXCEEDED`; SKIP LOCKED + lease reclaim + heartbeat still pass (`claim_test.go`).
- **Exit criteria:** A runaway `automation.run` loop hits concurrent + daily caps without exhausting the API DB pool; Client-critical `default` jobs still claim.
- **Depends on:** Phase 1 job types `deploy.validate` / `deploy.apply` (or map existing types). ExecutionRun `throttled` status can wait for Phase 3 (`last_error` is enough in v1).

### Phase 3 — ExecutionRun / ExecutionLogEntry + writers

- **Owner:** `db-backend-perf` then `authz-security` then `api-families` / `worker-jobs` for writers
- **Packages allowed:** `internal/seed`, `internal/packages`, `internal/dataengine`, `internal/db`, `internal/authz`, `internal/httpapi` (capability gate on Client read; 403 on customer write), `internal/automation`, `internal/worker`, `internal/deploy`, `internal/agentloop` (stats only), `migrations/` if HV/partition helpers need a nudge, `docs/data-model.md`, `docs/modules/platform-debug.md`, `docs/modules/README.md`, `docs/automation-sdk.md`, `docs/architecture/system-capabilities.md`
- **Packages forbidden:** `tools/control-ide/**`; do not use `audit_log` as the developer stream; do not add a fourth API family
- **Files likely to change:** `internal/seed/module_platform_debug.go` (new), `seed.go`, `modules.go`; `internal/seed/seed.go` `capabilityPermissionSetDefs` (`DebugViewer`); `internal/httpapi/server.go` sobject handlers (deny CUD; require `debug.read` on GET/query); new `internal/execution` writer; `internal/worker/automation_run.go`, `process.go`, `retention.go`; `internal/automation/embed/bootstrap.ts`, `one_automation.ts`, `deno.go`; `internal/dataengine` only if HV predicates need an indexed-lookup hint
- **Tests:** `go test ./internal/seed/... ./internal/packages/... ./internal/dataengine/... ./internal/authz/... ./internal/httpapi/... ./internal/worker/... ./internal/automation/... ./internal/execution/...`. Cover: HV query without `ExecutionRunId`/time rejected; customer POST ExecutionRun 403; `debug.read` principal can GET; guest `ctx.log.error` persists a line; purge deletes aged HV rows; job retry does not duplicate ExecutionRun for the same `JobId`.
- **Exit criteria:** A failing automation is reproducible from Client: open `ExecutionRun`, page `ExecutionLogEntry`, see exception text — no SSH / operator `slog`.
- **Depends on:** Phase 2 class column useful for Kind mapping but not a hard schema dep (Kind comes from job_type). BP-034 Monitor **must not** block this phase.

### Phase 4 — Docs-only debug UX (IDE frozen)

- **Owner:** `worker-jobs` or docs-capable `api-families` (not `control-ide`)
- **Packages allowed:** `docs/customer-developer-workflow.md`, `docs/automation-sdk.md`, maybe `docs/customer-connect.md`
- **Packages forbidden:** `tools/control-ide/**` — BP-034 Phase 4 Monitor IDE is Frozen
- **Files likely to change:** docs listed above
- **Tests:** none beyond link checks
- **Exit criteria:** A builder following customer-developer-workflow can query debug objects over Client API
- **Depends on:** Phase 3

### Phase 5 — Elastic knobs + OTEL hooks

- **Owner:** `deploy-ops` (+ `worker-jobs` for meters)
- **Packages allowed:** `docs/`, optional `internal/otel`, `internal/worker` metric points, optional Deploy/Metadata admin exposure of quota env (install-local)
- **Packages forbidden:** multi-tenant SaaS metering; Control IDE Hosting UI (BP-027 frozen)
- **Exit criteria:** Docs state how raising worker replicas interacts with `JOB_SLOTS_*`; throttle counts are visible on the operator OTEL plane (BP-008) without mixing into ExecutionLogEntry
- **Depends on:** Phases 1–2; BP-008 partial is enough for no-op exporter

---

## 4. Explicit non-goals

- Multi-tenant fair-share across customers (ADR-001)
- Unfreezing Control IDE Operate Monitor / Build-Ship ExecutionRun chrome (BP-034 Frozen; ADR-030)
- Removing Deno import/network bans (ADR-014)
- External queue (SQS/etc.) as the default (would need a new ADR)
- Replacing BP-008 operator OTEL with ExecutionLogEntry
- Using `audit_log` as the developer debug stream
- Legacy vendor debug-log file format
- Peer/inbound promote
- Changing AgentSpec harness `job_class` (BP-064)
- Guaranteeing sync Deploy for multi-thousand-file packs
- Raising HV unbounded scans / bypassing ADR-013 guardrails
- Reintroducing Graphile Worker / Node sidecars (ADR-005 / BP-012)
- Regressing `FOR UPDATE SKIP LOCKED` leases (BP-005)

---

## 5. Agentic implementation prompt(s)

Phase 4 IDE is **frozen** (BP-034). Do not ship Control IDE Monitor/Build chrome in these slices. Docs-only Phase 4 and elastic Phase 5 are follow-ons after Phase 3 — not in the prompts below.

### Prompt A — Phase 1 admission + async Deploy

```text
Implement Majesta One BP-033 Phase 1 only (API admission lanes + async repo→org Deploy). Do not implement job-class slots/quotas, ExecutionRun objects, or Control IDE UI.

Read first:
- docs/architecture/agentic-remainders/08-bp-033-runtime-isolation.md (§2.2–2.3, Phase 1)
- docs/architecture/customer-runtime-isolation-build-plan.md (Phase 1; do not duplicate Phase 2–3)
- docs/architecture/agent-api-families.md
- docs/architecture/agent-deploy.md
- docs/architecture/agent-worker.md (BP-005 leases — do not regress)
- backlog/BP-033-customer-runtime-isolation.md

Edit scope (allowed):
- internal/httpapi (middleware.go lane limiter; deploy_routes.go 202/DEPLOY_BUSY; GET /deploy/v1/work/{jobId}; tests)
- internal/config (ADMISSION_CLIENT_RPM_SHARE, DEPLOY_SYNC_MAX_FILES/BYTES, DEPLOY_QUEUE_MAX, JOB_SLOTS_DEPLOY)
- internal/deploy (size gate; enqueue deploy.validate / deploy.apply; keep PromoteBundle poll via existing GET promotions/{id})
- internal/worker (process.go cases for deploy.validate / deploy.apply only; preserve SKIP LOCKED claim)
- internal/mcp/builder.go (org_validate / org_deploy return 202 handles; do not block MCP HTTP on full apply)
- cmd/one/org.go (poll 202 until validate/deploy report is ready)
- docs/tech-stack.md, docs/security.md, .env.example (new env keys)

Tests:
- go test ./internal/httpapi/... ./internal/config/... ./internal/deploy/... ./internal/worker/... ./internal/mcp/...
- go test ./cmd/one/... if you change org validate/deploy
- Claim tests in ./internal/worker must stay green (FOR UPDATE SKIP LOCKED + leases)

Out of scope:
- jobs.class column, install_quota_counters, JOB_SLOTS_AUTOMATION/AGENT enforcement
- ExecutionRun / ExecutionLogEntry / platform_debug seed
- tools/control-ide/**
- Peer/inbound promote
- Changing agent_playbooks.job_class (BP-064 harness)
- Phase 4 IDE Monitor (BP-034 frozen)
```

### Prompt B — Phase 2 job classes + quotas

```text
Implement Majesta One BP-033 Phase 2 only (worker execution classes, slot caps, daily quotas, Deno RSS soft limit). Do not seed ExecutionRun. Do not edit Control IDE. Do not regress BP-005 SKIP LOCKED leases.

Read first:
- docs/architecture/agentic-remainders/08-bp-033-runtime-isolation.md (§2.1, §2.4, Phase 2)
- docs/architecture/customer-runtime-isolation-build-plan.md (Phase 2 + default budget table)
- docs/architecture/agent-worker.md
- backlog/BP-033-customer-runtime-isolation.md

Edit scope (allowed):
- migrations/ next free SQL + meta/_journal.json: jobs.class CHECK (automation|agent|deploy|default) DEFAULT 'default'; index (status, class, run_at); install_quota_counters (day, class, count)
- internal/worker: ClaimJobsOfClass with running-count cap in the same SKIP LOCKED statement; ProcessJobs claims default first, then automation/agent/deploy; Enqueue helper; process.go QUOTA_EXCEEDED; tests in claim_test.go / process_test.go
- All INSERT INTO jobs sites: set class via helper (dataengine, httpapi, deploy, mcp, metadata, db/sharing)
- internal/automation/deno.go: optional AUTOMATION_ASYNC_DEADLINE_SECONDS (default 30); RSS soft limit when OS allows
- internal/config: JOB_SLOTS_AUTOMATION/AGENT/DEPLOY, QUOTA_AUTOMATION_RUNS_PER_DAY, QUOTA_AGENT_RUNS_PER_DAY, DENO_RSS_LIMIT_MB, EXECUTION_LOG_BYTES_PER_RUN (buffer cap even without objects)
- cmd/worker if ProcessOptions needs the new knobs
- .env.example + docs/tech-stack.md / docs/security.md env table

Tests:
- go test ./internal/worker/... ./internal/automation/... ./internal/config/...
- Two workers cannot exceed JOB_SLOTS_AUTOMATION
- default class still claims while deploy class is at JOB_SLOTS_DEPLOY
- Daily cap fails jobs with QUOTA_EXCEEDED
- Existing lease reclaim / ClaimJobByID / heartbeat tests pass

Out of scope:
- ExecutionRun / ExecutionLogEntry / debug.read HTTP
- tools/control-ide/**
- External queues
- Renaming or reusing AgentSpec harness job_class (query|customize|ship|govern|operate|skill)
- Phase 1 admission if not already merged — if lanes are missing, stop and say Phase 1 is a prerequisite rather than reinventing rate limits
- Phase 4 IDE (frozen)
```

### Prompt C — Phase 3 ExecutionRun / ExecutionLogEntry

```text
Implement Majesta One BP-033 Phase 3 only (managed debug objects + platform writers + Client read). Control IDE Monitor is frozen (BP-034) — docs in this slice only as needed for object/SDK contracts. Do not add Electron panels.

Read first:
- docs/architecture/agentic-remainders/08-bp-033-runtime-isolation.md (§2.5, Phase 3)
- docs/architecture/customer-runtime-isolation-build-plan.md (data model + Phase 3)
- docs/architecture/agent-data-architecture.md
- docs/adr/013-high-volume-flexible-storage.md
- docs/adr/014-customer-code-automations.md
- docs/architecture/agent-authz.md
- docs/architecture/system-capabilities.md
- backlog/BP-033-customer-runtime-isolation.md

Edit scope (allowed):
- internal/seed: new always-on platform_debug module (Optional: false) with ExecutionRun (flexible) + ExecutionLogEntry (high_volume); InstallPlatformDebug from seed.go like agents_starter; DebugViewer PS with debug.read
- docs/modules/platform-debug.md + docs/modules/README.md always-on table; docs/data-model.md (move off “Planned”)
- internal/execution (new): platform writer upsert-by-JobId, append log lines with Seq, byte cap EXECUTION_LOG_BYTES_PER_RUN
- internal/httpapi sobjects: deny customer CUD on those apiNames; GET/query requires CapDebugRead + object read
- internal/worker + internal/automation + internal/deploy + internal/agentloop: write runs/logs; guest ctx.log.info/warn/error (keep ctx.log as info); execution_log.purge retention
- internal/authz only if DebugViewer / debug.read enforcement needs a helper
- docs/automation-sdk.md for log levels
- 202 Deploy/automation responses may add executionRunId (keep jobId)

Tests:
- go test ./internal/seed/... ./internal/packages/... ./internal/httpapi/... ./internal/worker/... ./internal/automation/... ./internal/execution/... ./internal/dataengine/... ./internal/authz/...
- HV query without ExecutionRunId or CreatedAt bound is rejected
- Customer POST ExecutionRun → 403
- Principal with DebugViewer can query lines for a run
- Job retry does not duplicate ExecutionRun for the same JobId
- Purge removes HV rows older than EXECUTION_LOG_RETENTION_DAYS

Out of scope:
- tools/control-ide/** and BP-034 TraceFlag / Monitor poll API (unless already specified elsewhere — do not build IDE chrome)
- Changing admission lanes or SKIP LOCKED claim (Phase 1–2)
- audit_log as debug stream; OTEL operator console
- npm in Deno; removing sandbox bans
- Peer promote
```
