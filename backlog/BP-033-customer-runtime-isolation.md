# BP-033: Customer runtime isolation, execution budgets, and debug objects

- **Severity:** High
- **Status:** Partially mitigated (Phase 1 admission + async Deploy landed; Phases 2–3 remain)
- **Area:** `internal/httpapi` (admission), `internal/worker` (job classes / quotas), `internal/automation`, `internal/deploy`, `internal/seed` / `internal/packages` (ExecutionRun + ExecutionLogEntry), `migrations/` (quota counters)
- **Design:** [customer-runtime-isolation-build-plan.md](../docs/architecture/customer-runtime-isolation-build-plan.md)
- **Remainder (agentic):** [08-bp-033-runtime-isolation.md](../docs/architecture/agentic-remainders/08-bp-033-runtime-isolation.md) (Phases 1–3 + docs-only 4; IDE Monitor frozen)

## Problem

A single Majesta One install is both the **live CRM** and the **Build/Ship/automation/agent** runtime. Without isolation:

1. Repo→org validate / deploy / customer tests can run synchronously on the API process and shared DB pool, starving Client traffic while the org is still “operational.”
2. A buggy automation or agent can enqueue unbounded work; today’s Deno wall/depth/mutation caps help per-run but there is no install-level concurrency/daily budget, and no RSS limit story — yet the product still wants **more headroom than typical incumbent platform** governors.
3. Customers cannot effectively debug their own code: failures land in operator `slog`, `jobs.last_error`, or stub `agent_runs` — not in queryable, retained, AuthZ’d developer objects.

## Why it matters

B2B buyers expect production to stay up during deploys and expect execution-trace–class visibility for automations/agents — without accepting typical incumbent platform heap/CPU ceilings as Majesta One’s product limit.

**DX dependency:** Productized CLI/IDE tight loops ([BP-048](./BP-048-one-cli.md), [BP-032](./BP-032-customer-dx-validate-deploy.md)) assume validate/deploy cannot starve live Client traffic. Treat this BP as a **production DX safety prerequisite** — not optional polish after CLI auth/init.

## Direction

Follow the build plan:

1. **Admission lanes** — reserve most RPM/concurrency for Client; budget Deploy/Metadata.
2. **Async Deploy by default** for expensive org validate/deploy/test; `JOB_SLOTS_DEPLOY=1` with queue/`DEPLOY_BUSY` (repo→org only — no peer promote).
3. **Worker job classes** + daily quotas + raised-but-finite Deno/agent budgets (env-overridable with install size).
4. **Two managed objects:** `ExecutionRun` (flexible) + `ExecutionLogEntry` (`high_volume`) with `debug.read`, SDK `ctx.log.*`, retention purge.
5. Keep **BP-008 OTEL** as the operator plane; do not overload `audit_log`.

## Mitigation

| Slice | Status |
|---|---|
| Build plan + this BP | Done (this change) |
| Phase 1 — API admission + async Deploy | **Done** (lanes + expanded-artifact size-gated 202 / `DEPLOY_BUSY` + `deploy.validate`/`deploy.apply`; uploaded archives are capped at 128 MiB expanded / 10,000 files; no `jobs.class` yet) |
| Phase 2 — Job classes + quotas + Deno RSS soft limit | Open |
| Phase 3 — ExecutionRun / ExecutionLogEntry + writers | Open |
| Phase 4 — IDE/docs debug UX | Open |
| Phase 5 — Elastic knobs + OTEL hooks | Open |

## Explicit non-goals

- Multi-tenant SaaS fair-share across customers
- Removing sandbox import/network bans (ADR-014)
- Legacy vendor debug-log file compatibility
- Replacing BP-008 operator observability

## Related

- [BP-034](../docs/adr/030-install-agent-runtime.md) — Operate Monitor IDE consumes TraceFlag + ExecutionLogEntry (Phase 4 of tools plan)
- [BP-032](./BP-032-customer-dx-validate-deploy.md) · [BP-048](./BP-048-one-cli.md) — repo→org DX / CLI productization (protected by this isolation work)
- [customer-runtime-isolation-build-plan.md](../docs/architecture/customer-runtime-isolation-build-plan.md)
- [08-bp-033-runtime-isolation.md](../docs/architecture/agentic-remainders/08-bp-033-runtime-isolation.md) — remainder + paste-ready Phase 2–3 prompts (Phase 1 is in tree)
- [BP-005](../docs/architecture/agent-worker.md) — leases (mitigated); this BP adds classes/quotas on top
- [BP-008](./BP-008-production-packaging.md) — OTEL operator telemetry
- [BP-009](./BP-009-no-in-kernel-language.md) · [ADR-014](../docs/adr/014-customer-code-automations.md)
- [BP-006](./BP-006-agent-guardrails.md) — hosted agent loop still consumes these budgets
- [BP-032](./BP-032-customer-dx-validate-deploy.md) — DX repo→org validate/deploy UX must use budgeted Deploy path
- [BP-048](./BP-048-one-cli.md) — CLI/IDE productization; production DX loops depend on this isolation
- [multi-env-deploy.md](../docs/multi-env-deploy.md) — peer registry is topology only; no inbound promote
- [ADR-013](../docs/adr/013-high-volume-flexible-storage.md) — HV log lines
