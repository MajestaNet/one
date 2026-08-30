# Customer code automations — build plan

Executable plan for sandboxed TypeScript automations on a Majesta One install.  
**ADR:** [ADR-014](../adr/014-customer-code-automations.md)  
**Backlog:** [BP-009](../../backlog/BP-009-no-in-kernel-language.md)  
**Playbooks:** [agent-authz.md](./agent-authz.md), [agent-worker.md](./agent-worker.md), [agent-data-architecture.md](./agent-data-architecture.md), [agent-deploy.md](./agent-deploy.md), [agent-control-ide.md](./agent-control-ide.md), [agent-api-families.md](./agent-api-families.md)

## Locked product decisions

| Topic | Choice |
|---|---|
| Authoring | Real TypeScript; Build agent chat primary; hand-write OK |
| Viz | Review/approval only |
| Engine | Deno **default-deny** |
| Libraries | **Ban all** third-party / Deno std / URL imports in v1 (agents included) |
| Allowed module | Only virtual `one:automation` (or ambient `ctx` with zero imports) |
| PS grants | Automation **list** (or `allAutomations`) on the permission set |
| Run-as | Starter principal; schedules require explicit `runAsPrincipalId` |
| Sync | Same DB transaction as triggering write; fail → **full rollback** |
| Async | Default; platform jobs/outbox; no customer queues |
| Sync I/O | No outbound HTTP/connectors in sync |

### Library ban (v1) — yes, block everything

**Decision: block all libraries for now.** Not even a Build agent may emit an import of a third-party package.

| Allowed | Forbidden |
|---|---|
| `export default async function run(ctx) { ... }` using SDK on `ctx` | `import … from "npm:…"`, `jsr:`, `https://`, `std/`, relative imports of non-automation files that pull deps |
| Optional: `import type { AutomationContext } from "one:automation"` | `require`, dynamic `import()`, `eval`, `new Function`, Workers |
| Platform-injected bridge only | Vendoring `node_modules` into the customer repo |

Enforcement: static parse at pack/validate + Deno permission flags + CI on customer pack. Playbook text for Build agents must state the ban explicitly.

Revisit only with a new ADR (signed allowlist, hashed pins, legal review).

---

## Current gaps (inventory)

| Area | Today | Need |
|---|---|---|
| `metadata_automations` | apiName, trigger, JSON `actions` | `execution`, `sourcePath` / blob ref, `runAsPrincipalId` (schedule), ownership already exists |
| `afterWrite` | Sync + async + Deno guest (Phases 3–4) | Build agent + Control IDE (Phase 6) |
| Worker `automation.run` | Deno invoke + AuthZ as run-as + audit | Outbound connectors (Phase 7) |
| Deploy tests | `automationUnitPass` / `automationContract` + optional promote gate | IDE wiring (Phase 6) |
| Permission sets | object/field + system caps | `automation_permissions` catalog (+ `allAutomations`) |
| Customer repo | `metadata/automations/*.yaml` actions | `src/automations/**/*.ts` + `tests/automations/**` |
| Deploy tests | Metadata existence steps | Automation unit + contract steps |
| Control IDE | Metadata YAML | Monaco TS, Build chat, PS automation list, review graph |

---

## Phases

Execute in order. Each phase is mergeable and test-gated. Prefer domain agents noted per phase.

### Phase 0 — Docs & contracts (this change set)

**Owner:** architecture / product  
**Done when:** ADR-014 + this plan + BP-009 retargeted; tech-stack note updated.

- [x] ADR-014 accepted text
- [x] Build plan
- [x] BP-009 points here (not declarative-first)

### Phase 1 — AuthZ: automation grants on permission sets

**Packages:** `migrations/`, `internal/authz`, `internal/db`, `internal/httpapi` (Metadata PS), seed  
**Agents:** `authz-security`, `api-families`  
**Playbook:** [agent-authz.md](./agent-authz.md)  
**Status:** Done (migration `0023`, `AutomationAuthz`, Metadata `automationAccess`, catalog ensure on automation create)

**Deliverables**

1. Kernel table `automation_permissions` + `permission_sets.all_automations`
2. Catalog hygiene: on automation create → deny stub on every PS; Admin/`allAutomations` → `can_run=true`
3. Metadata GET/PATCH permission sets expose `automationAccess`
4. Enforce helper: `ActorCanRunAutomation` (OR-union across PSs)
5. Tests: stub on create; grant via PS; deny without grant; Admin / allAutomations

**Exit criteria:** `go test` AuthZ + Metadata PS round-trip; no Deno yet.

### Phase 2 — Metadata model for code automations

**Packages:** `migrations/`, `internal/automation`, `internal/metadata`, `internal/httpapi/metadata_routes.go`, `internal/deploy`, `internal/customerrepo`  
**Agents:** `api-families`, `deploy-ops`, `db-backend-perf`  
**Playbooks:** api-families + deploy  
**Status:** Done (migration `0024`, pack `src/automations/`, import ban, validate/apply/HTTP)

**Deliverables**

1. Columns: `runtime`, `execution`, `entry_file`, `source`, `run_as_principal_id` (+ legacy `actions`)
2. Validate: sync ⇒ no outbound actions; schedule ⇒ runAs; code ⇒ entryFile/source; import ban
3. `one/v1` tree: `src/automations/**/*.ts`, `tests/automations/**/*.ts`
4. Pack embeds Sources; unpack writes them back; reject `npm:` etc.

**Exit criteria:** Pack round-trip with empty `run` returning `{ ok: true }`; validate fails on `import "npm:lodash"`.

### Phase 3 — Sync transactional path (abort + rollback)

**Packages:** `internal/dataengine`, `internal/automation`  
**Agents:** `db-backend-perf`, `worker-jobs`  
**Playbooks:** data-architecture + worker  
**Status:** Done (same-tx Create/Update/Delete; sync `createRecord`/`updateRecord`/`fail` actions; async jobs enqueued in same tx; code sync deferred to Phase 4)

**Deliverables**

1. Create/Update/Delete run in one Postgres transaction with audit/outbox/automations
2. Sync automations use in-tx mutator (no auto-commit Client HTTP)
3. Failure / AuthZ / timeout → `ROLLBACK` entire unit
4. Async automations enqueue `automation.run` on the same tx
5. Caps: 5s deadline, max depth 3, max 50 mutations
6. Sync forbids outbound actions; code runtime uses Deno guest (Phase 4)

**Exit criteria:** Integration test — sync fail → triggering create absent; success → parent + child committed (`TestSyncAutomationCommitAndRollback`, needs `DATABASE_URL`).

### Phase 4 — Deno sandbox executor (async + sync bridge)

**Packages:** `internal/automation`, `internal/worker`, `deploy/Dockerfile` (pinned Deno binary), config  
**Agents:** `worker-jobs`, `deploy-ops`  
**Playbook:** [agent-worker.md](./agent-worker.md)  
**Status:** Done (Deno `2.9.3` guest via NDJSON host RPC; worker `automation.run`; sync code path; import ban before run; AuthZ as run-as)

**Deliverables**

1. Pin Deno version; invoke with `--no-npm --no-remote` (or equivalent) + custom `one:automation` resolver
2. Default-deny permissions; only host callbacks for SDK
3. `run(ctx)` protocol: JSON stdin/stdout or native bridge; capture logs; map errors
4. Worker `automation.run`: resolve definition, resolve run-as (starter from payload or schedule principal), check `ActorCanRunAutomation`, execute, audit
5. Sync path: same executor library called in-request with tx-bound ctx
6. Resource limits: CPU/mem/time; kill → failed run (+ rollback if sync)

**Exit criteria:** Async job creates child record as run-as user; user without create → fail; import of external module → reject before run (`TestRunGuest_*`, `TestProcessJobsAutomationRunCode*`, `TestSyncCodeAutomationCommitAndRollback`).

### Phase 5 — SDK, unit harness, Deploy test steps

**Packages:** `internal/automation`, `internal/deploy`, docs for customer authors, optional `tools/` type stubs (vendor plane only — **not** npm for customers)  
**Agents:** `deploy-ops`, `worker-jobs`  
**Status:** Done (frozen SDK + unit harness; `automationUnitPass` / `automationContract`; `DEPLOY_REQUIRED_TEST_SUITES` promote gate; template `CreateAccount_From_Contact` + `Referral__c`)

**Deliverables**

1. Frozen SDK: `getRecord`, `createRecord`, `updateRecord`, `deleteRecord`, `query`, `log` (+ async-only `http`/`connector` stubs behind feature flag / BP-014)
2. Unit harness: Deno test runner with mock ctx; pack runs `tests/automations/**`
3. Deploy test step types: `automationUnitPass`, `automationContract` (fixture → assert records)
4. Activate gate: promotion requires configured suite green (`DEPLOY_REQUIRED_TEST_SUITES`)
5. Retire reliance on opaque JSON `actions` for new automations (legacy actions still supported; new examples use `runtime=code`)

**Exit criteria:** Example Account→Opportunity automation + unit test in synthetic customer pack; CI-style `/deploy/v1/tests/runs` passes (`TestAutomationUnitAndContractSuite`).

### Phase 6 — Build agent + Control IDE

**Packages:** `tools/control-ide/**`, `internal/seed` / `agents_starter` templates (clone-on-enable)  
**Agents:** `control-ide` (+ backend only if AgentSpec seed changes)  
**Playbooks:** control-ide + ADR-010  
**Status:** Partial — Automations panel (Monaco TS hand-write + YAML), exclusive Build tabs, Repo commit + sample init shipped; Build-agent write loop still BP-006

**Deliverables**

1. Managed Build AgentSpec instructions: ask clarifying questions; emit TS + tests + YAML; **never suggest third-party imports**; propose PS `automationAccess` grants — *pending hosted tools*
2. IDE: Monaco for automation TS; Automations panel; PS editor section for automation list — *Monaco + Automations panel done; PS automation list editor remainder*
3. Review-only graph from static analysis of SDK calls (optional MVP: list of objects touched) — *pending*
4. Manual “Run” / Client invoke respects caller PS `canRun` + run-as = caller — *owned by [integrations-build-plan.md](./integrations-build-plan.md) / [BP-047](../../backlog/BP-047-integrations-callable-oauth.md)*

**Exit criteria:** Vitest for PS editor + pack validate; smoke scripted Build chat → files → unit pass (may be demo-flagged until hosted tool loop completes — see BP-006).

### Phase 7 — Async outbound connectors

**Depends:** [BP-014](../../backlog/BP-014-agent-outbound-integrations.md)  
**Plan:** [outbound-otel-build-plan.md](./outbound-otel-build-plan.md)  
**Agents:** `worker-jobs`, `authz-security`  
**Status:** Done (Metadata secrets/connectors/egress; async `ctx.http`/`ctx.connector`; AgentSpec `allowedSkills`; OTEL egress spans)

Allowlisted GET/POST via secret refs; async only; egress allowlist; still **no** customer-imported HTTP libraries (SDK only). AgentSpec may grant automation apiNames as `allowedSkills`.

---

## Cross-cutting enforcement checklist

| Control | Where |
|---|---|
| Import ban | Pack validate, Deploy validate, runtime loader |
| Deno default-deny | Executor flags |
| PS `canRun` | Manual invoke + (optional) activation UI; schedule uses definition run-as + that principal’s `canRun` |
| Object/FLS | Every SDK mutate as run-as |
| Sync rollback | Single tx in dataengine |
| No kernel mutate | Sandbox cannot reach Metadata managed rows / Deploy / Ops (SDK omits those families) |
| Audit | `audit_log` actor = run-as; automation apiName in details |
| Customer debug | `ExecutionRun` + `ExecutionLogEntry` (HV) via [BP-033](../../backlog/BP-033-customer-runtime-isolation.md) — not `audit_log` |

## Suggested example (Phase 5 acceptance)

**Automation:** `CreateAccount_From_Contact` — on Contact create, create Account + `Referral__c` linking ContactId/AccountId lookups.  
**PS:** grant `CreateAccount_From_Contact` in `automationAccess`.  
**User** with Contact create + Account/Referral create + that PS saves Contact → Account + Referral created (async or sync per definition).  
**User** without Account create → automation fails (async: job failed; sync: Contact create rolled back).

## Non-goals (plan scope)

- Drag-and-drop builder
- npm allowlist
- Proprietary in-kernel scripting language
- Customer Electron plugins
- In-kernel CTI/SMTP (connectors stay Message/allowlisted HTTP later)

## Implementation order (agents)

```text
Phase 1 authz-security
  → Phase 2 api-families + deploy-ops
  → Phase 3 db-backend-perf
  → Phase 4 worker-jobs + deploy-ops
  → Phase 5 deploy-ops + worker-jobs
  → Phase 6 control-ide
  → Phase 7 (BP-014)
```

## Related

- [ADR-014](../adr/014-customer-code-automations.md)
- [ADR-029](../adr/029-platform-actions.md) · [platform-actions-build-plan.md](./platform-actions-build-plan.md) — product verbs customers call via `invokeAction`
- [customer-customizations.md](../customer-customizations.md)
- [customer-repo.md](../customer-repo.md)
- [ci-customer-tests.md](../ci-customer-tests.md)
- [customization-authz.md](./customization-authz.md)
- [ADR-010](../adr/010-customer-agentic-platform.md)
