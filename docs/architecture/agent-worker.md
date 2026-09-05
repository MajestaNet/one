# Agent playbook: Worker / jobs

For agents changing async jobs, outbox delivery, webhook POSTs, or the worker claim loop. Follow this before writing code.

## Where to look

| Concern | Path |
|---|---|
| Worker entry | `cmd/worker` |
| Claim / lease | `internal/worker/claim.go`, `claim_test.go` |
| Process handlers | `internal/worker/process.go`, `process_test.go` |
| Poll loop | `internal/worker/loop.go` |
| Kernel tables | `migrations/` (`jobs`, `outbox_events`, `webhook_deliveries`, lease columns) |
| Enqueue from API | Client/Metadata handlers that insert jobs/outbox rows; agent runs in `internal/httpapi/client_extras.go`; platform actions stay in-request when `syncSafe` ([platform-actions-build-plan.md](./platform-actions-build-plan.md)) |
| System notification intents | `internal/db/outbox.go` — `install.claimed`, `principal.created`, `principal.password_changed` ([BP-038](../../backlog/BP-038-no-product-mailer-byo-alerts.md)); **no product mailer** |
| Backlog | SKIP LOCKED leases (mitigated), [`BP-006`](../../backlog/BP-006-agent-guardrails.md) (hosted loop — [plan](./hosted-agent-tool-loop-build-plan.md)), [`BP-064`](../../backlog/BP-064-install-agent-runtime.md), [`BP-033`](../../backlog/BP-033-customer-runtime-isolation.md) (job classes / quotas / ExecutionRun), [`BP-038`](../../backlog/BP-038-no-product-mailer-byo-alerts.md), [`BP-041`](../../backlog/BP-041-record-external-id-upsert-bulk.md) (`ingest.process` — [plan](./external-id-upsert-bulk-build-plan.md)), [`BP-043`](../../backlog/BP-043-cross-object-search-api.md) (`search.reindex` — [plan](./cross-object-search-build-plan.md)), [`BP-061`](../../backlog/BP-061-platform-actions.md) (guest `invokeAction`) |

## What ships today

```text
Postgres-backed jobs + outbox (no separate Node job runner)
Claim: UPDATE … WHERE id IN (SELECT … FOR UPDATE SKIP LOCKED)
Leases: locked_at / locked_by; expired lease reclaim (~5 minutes)
Webhook idempotency: webhook_deliveries unique ledger
At-least-once delivery with handler idempotency responsibility
```

API **and** worker still call `EnsureKernel` on boot (`cmd/api`, `cmd/worker`). Concurrent first migrate is unsafe when a kernel SQL file is not idempotent — wait for API `/readyz` before starting the worker ([customer-rollout-gap-log.md](../customer-rollout-gap-log.md) G-MIGRATE-RACE).

## What to do (change types)

### A. Change claim / concurrency

1. Preserve `FOR UPDATE SKIP LOCKED` (or equivalent lease) — do not regress to unlocked polls (BP-005).
2. Cover concurrency in `claim_test.go`.
3. Tune poll interval / batch size in `loop.go` only with measurements.

### B. Add a job / outbox type

1. Define the row shape in kernel migrations if the schema changes.
2. Enqueue from the owning API family (usually Client or Metadata).
3. Implement process branch in `process.go` with idempotent side effects.
4. Add focused tests; run `go test ./internal/worker/...`.

### C. Webhook delivery

1. Keep the unique delivery ledger for idempotent POSTs.
2. Do not reintroduce Graphile Worker / Node sidecars (ADR-005 / BP-012).

### D. Agent runs (async)

1. Run execution may be job-backed; the **shared** hosted loop lives in `internal/agentloop` — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md). Guardrails and principal identity are AuthZ/Client concerns — see [agent-authz.md](./agent-authz.md) and [agent-api-families.md](./agent-api-families.md).
2. Reconstruct the run `Actor` from `agent_runs.actor_id`; **fail** if missing. Never fall back to `DEFAULT_OWNER_ID`. Audit `actor_id` on tool outcomes stays that principal (BP-006 / BP-013).
3. JSON `POST .../approve` enqueues `agent.run` with `resume` (do not start a blank generation). SSE approve continues in-process and **must not** also enqueue. When parking at `awaiting_tool_approval`, release the job lease — resume is a new job.

### E. Job classes / quotas / customer debug (BP-033)

1. Follow [customer-runtime-isolation-build-plan.md](./customer-runtime-isolation-build-plan.md) — per-class slots, daily quotas, `ExecutionRun` / `ExecutionLogEntry` writers.
2. Do not let Deploy/automation/agent classes starve the default claim path for Client-critical work without documented admission rules.

### F. Platform action host RPC (`invokeAction`)

1. Follow [platform-actions-build-plan.md](./platform-actions-build-plan.md) Phase 3 — one frozen SDK method; dispatch to `internal/actions.Invoke` as run-as.
2. Sync guests may call only `syncSafe` actions (share the write tx). Do not let Deno call Client HTTP.
3. Do not add per-verb `ctx` methods.

## Explicit non-goals (until docs say otherwise)

- External queue products as the default (SQS/etc.) without an ADR
- Reintroducing Node/Graphile worker paths
- Processing managed metadata migrations in the worker (those ride API boot / migrate). Today `cmd/worker` still calls `EnsureKernel` — do not add a *third* migrator; prefer serializing on API `/readyz` until that call is removed.

## Checklist before merging a worker PR

- [ ] Claim still uses SKIP LOCKED (or documented equivalent)
- [ ] Handlers are idempotent under at-least-once delivery
- [ ] Tests cover claim and/or new process branch
- [ ] No new runtime language or sidecar
- [ ] Worker follow-ups updated if tuning/retry behavior changed
