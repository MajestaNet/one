# Ops, automations, OSS process, integrations — remainder tech design + agentic build plan

**Work-order slot:** 12 of 12 (recommended Finish order from backlog/README.md — last Finish bucket: OTEL, OSS process, automations)
**Backlog:** [BP-008](../../../backlog/BP-008-production-packaging.md) · [BP-026](../../../backlog/BP-026-oss-security-public-backlog.md) · [BP-009](../../../backlog/BP-009-no-in-kernel-language.md) · [BP-047](../../../backlog/BP-047-integrations-callable-oauth.md)
**Track:** Finish
**Status of remainder:** Partial (BP-008 logs exporter Keep, queue-depth waits on BP-033 P2; BP-026 Open process; BP-009 Keep — Phase 7 code is shipped; BP-047 thin invoke-status Keep, ExecutionRun projection Deferred on BP-033)
**Domain agents:** `worker-jobs` / `api-families` / `authz-security` / `deploy-ops` (`db-backend-perf` only if BP-033 Phase 3 objects already exist — do not create them here)
**Playbooks:** [agent-worker.md](../agent-worker.md) · [agent-api-families.md](../agent-api-families.md) · [agent-authz.md](../agent-authz.md) · [agent-deploy.md](../agent-deploy.md) · [agent-data-architecture.md](../agent-data-architecture.md) (projection only)
**Existing plans (do not duplicate):** [outbound-otel-build-plan.md](../outbound-otel-build-plan.md) · [customer-automations-build.md](../customer-automations-build.md) · [integrations-build-plan.md](../integrations-build-plan.md) · [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) · [ADR-014](../../adr/014-customer-code-automations.md)

---

## 1. Remainder inventory

Honest tree state. Do not re-plan shipped phases. Control IDE Govern connector catalog is **shipped and chrome-frozen** — no new Govern UI.

| Surface | Shipped (cite packages/tests) | Still open | Evidence |
|---|---|---|---|
| **BP-008** OTLP traces + metrics + optional logs | `internal/otel` OTLP/HTTP traces + metrics; no-op when `OTEL_EXPORTER_OTLP_ENDPOINT` unset; resource attrs `PRODUCT_VERSION` / `CUSTOMER_ID` / `INSTALL_ID` / `INSTALL_ROLE`; wired from `cmd/api` + `cmd/worker`; HTTP middleware spans + slog `trace_id`/`span_id`; worker `job.{type}` spans; outbound redacted-URL spans; **optional logs** when `OTEL_LOGS_EXPORTER=otlp` | Pair queue-depth gauges **when BP-033 Phase 2 isolation budgets land** | [otel.go](../../../internal/otel/otel.go), [logs.go](../../../internal/otel/logs.go), [otel_test.go](../../../internal/otel/otel_test.go), [middleware.go](../../../internal/httpapi/middleware.go) (`TraceAttrs`), [config.go](../../../internal/config/config.go) |
| **BP-008** queue-depth metrics | None. Jobs table is undifferentiated; BP-033 job classes / slots / quotas are **Open** | Pair gauges/counters **when BP-033 Phase 2 isolation budgets land** (plan Phase 5 hook) | [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) Phase 2 Open; [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) Phase 5 item 3 |
| **BP-008** Node packaging | Closed (Go-only runtime) | — | [BP-008](../../../backlog/BP-008-production-packaging.md) Closed |
| **BP-026** disclosure + Dependabot | Root [SECURITY.md](../../../SECURITY.md); Dependabot npm / gomod / Actions; IDE CI `npm audit --audit-level=high`; one-time Go vuln-db baseline (not a CI `govulncheck` job); CONTRIBUTING has a one-line Security pointer | GitHub Security Advisories **publish-after-fix** policy; contact-alias verification; RFC 9116 `security.txt`; CONTRIBUTING blurb (no secrets / customer data / PoCs in public issues) | [dependabot.yml](../../../.github/dependabot.yml), [CONTRIBUTING.md](../../../CONTRIBUTING.md), [control-ide-security-audit.md](../control-ide-security-audit.md) Phase 5 row; **no** `.well-known/security.txt`, **no** GHSA process section in SECURITY.md |
| **BP-009** Phases 0–5 | AuthZ grants, code metadata + import ban, sync TX rollback, Deno guest, SDK + Deploy test steps | Keep: do not regress | [customer-automations-build.md](../customer-automations-build.md) Phases 0–5 Done; tests `TestSyncAutomationCommitAndRollback`, `TestRunGuest_*`, `TestProcessJobsAutomationRunCode*`, `TestSyncCodeAutomationCommitAndRollback`, `internal/automation/imports_test.go` |
| **BP-009** Phase 6 IDE | Automations panel (Monaco TS) shipped | Build write-loop → [BP-006](../../../backlog/BP-006-agent-guardrails.md); PS automation-list editor + review graph → **frozen IDE chrome**; Manual/Client invoke → BP-047 (**shipped**) | [customer-automations-build.md](../customer-automations-build.md) Phase 6 Partial; [ADR-030](../../adr/030-install-agent-runtime.md) |
| **BP-009** Phase 7 async outbound | **Shipped:** Metadata secrets/connectors/egress; async `ctx.http` / `ctx.connector`; sync ban; AgentSpec `allowedSkills`; OTEL egress spans. BP-009 is **Mitigated**. | Optional fail-closed unit test on `OutboundHost`; ExecutionRun debug is BP-033 | [customer-automations-build.md](../customer-automations-build.md) Phase 7 **Done**; [outbound.go](../../../internal/automation/outbound.go); [BP-009](../../../backlog/BP-009-no-in-kernel-language.md) |
| **BP-047** Phases 0–4 | Client invoke catalog + POST/GET runs; OAuth auth types + token/state tables; `/auth/v1` authorize/callback; host refresh; Deploy defs/refs; Govern catalog UX | No new product OAuth/invoke surface | [integrations-build-plan.md](../integrations-build-plan.md) Phases 0–4 Done; [client_automation_runs.go](../../../internal/httpapi/client_automation_runs.go), [client_automation_runs_test.go](../../../internal/httpapi/client_automation_runs_test.go), [connectoroauth/](../../../internal/connectoroauth/), [outbound_oauth_routes.go](../../../internal/httpapi/outbound_oauth_routes.go), [outbound_routes.go](../../../internal/httpapi/outbound_routes.go), [deploy/apply.go](../../../internal/deploy/apply.go) token drop on config-hash change |
| **BP-047** invoke status | Thin **job** status: `GET /client/v1/automations/runs/{id}` filters by `actorId` (admin exempt); POST async `202` + job id | Do **not** invent a second debug object. Project `executionRunId` **only after** BP-033 Phase 3 seeds `ExecutionRun` | [client_automation_runs.go](../../../internal/httpapi/client_automation_runs.go) `handleGetAutomationRun`; [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) Phase 3 **Open** — no `ExecutionRun` in `internal/seed` / `internal/packages` |
| **BP-047** OAuth HTTP tests | Package tests: flow URL build, token error redaction | HTTP authorize/callback/status round-trip tests are sparse (no `httpapi` OAuth route test file) | [flow_test.go](../../../internal/connectoroauth/flow_test.go), [token_test.go](../../../internal/connectoroauth/token_test.go); glob finds no `outbound_oauth*_test.go` |
| IDE Govern connectors | Catalog wizard shipped | **Frozen** — no new Govern chrome | [BP-047](../../../backlog/BP-047-integrations-callable-oauth.md), [BP-014](../../../backlog/BP-014-agent-outbound-integrations.md), [ADR-030](../../adr/030-install-agent-runtime.md) |

---

## 2. Detailed design (remainder only)

Cite existing ADRs. Do not invent a parallel stack. Do not unfreeze Control IDE chrome.

### 2.1 BP-008 — optional OTEL logs exporter

**Keep:** stdout JSON `log/slog` ([logging.go](../../../internal/logging/logging.go)); traces/metrics OTLP/HTTP; no collector sidecar (distroless unchanged — [outbound-otel-build-plan.md](../outbound-otel-build-plan.md) Verification).

**Add (opt-in logs):**

| Knob | Default | Meaning |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset → full no-op (today) | Shared OTLP/HTTP base URL |
| `OTEL_TRACES_EXPORTER` / `OTEL_METRICS_EXPORTER` | `otlp` when endpoint set (today) | Unchanged |
| `OTEL_LOGS_EXPORTER` | **`none` even when endpoint is set** | Opt-in `otlp` only — logs volume/cost must not surprise operators |

**Libraries (same change set as [tech-stack.md](../../tech-stack.md)):** `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` + SDK log provider + `go.opentelemetry.io/contrib/bridges/otelslog` (or equivalent slog `Handler`). Fan-out with `slog.NewMultiHandler(jsonStdout, otelHandler)` so Docker/journald JSON stays. Do not replace stdout.

**Correlation:** v1 already correlates via slog attrs (`trace_id` / `span_id` on access logs). The OTEL log record must attach the current span context when present so collectors join logs↔traces without a second ID scheme.

**Redaction (hard):** never export `Authorization`, ciphertext, OAuth tokens, secret refs’ values, or cookie headers. Drop those slog keys on the OTEL handler even if a caller logs them. Same rule as outbound spans ([outbound-otel-build-plan.md](../outbound-otel-build-plan.md) Security checklist).

**Resource attrs:** reuse the existing resource (`service.name`, `service.version`, `one.customer_id`, `one.install_id`, `one.install_role`).

**AuthZ / HTTP:** none. Operator plane only. Customer debug remains BP-033 `ExecutionRun` / `ExecutionLogEntry` — do not overload OTEL as the customer DX.

### 2.2 BP-008 — queue-depth metrics (depends on BP-033)

**Do not ship now** unless BP-033 Phase 2 has landed a job **class** (or equivalent slot dimension) on `jobs`. An undifferentiated `COUNT(*)` gauge that will be renamed per-class is churn.

When Phase 2 exists, register on the **worker** meter (API may scrape the same names as 0 if it has no claim loop):

| Metric | Type | Attrs | Source |
|---|---|---|---|
| `one.jobs.queue_depth` | ObservableGauge (int64) | `job.class` = `automation` \| `agent` \| `deploy` \| `default` | queued (not leased) rows in that class |
| `one.jobs.running` | ObservableGauge (int64) | `job.class` | leased/running |
| `one.jobs.throttled` | Counter | `job.class`, `reason` = `quota` \| `slots` | increment on `QUOTA_EXCEEDED` / slot reject |
| `one.deno.oom` | Counter | — | increment when BP-033 RSS/cgroup kill is detected |

Hook already named in [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) Phase 5. Writers of throttle/OOM events belong to BP-033; BP-008 only **exports** them. Never put customer source, secret values, or `last_error` text into metric attrs.

### 2.3 BP-026 — advisory policy, security.txt, CONTRIBUTING

Process/docs. Concrete artifacts (not a second tracking system):

**A. GitHub Security Advisories (publish after fix)**

Extend [SECURITY.md](../../../SECURITY.md) (do not put live vuln detail in `backlog/`):

1. **Intake:** private GitHub vulnerability report (enable in repo settings) **or** `security@majestanet.com`. Public issues are not an intake path.
2. **Ack:** 5 business days (already stated).
3. **Fix first:** patch on the latest product `v*` tag (supported-versions paragraph stays).
4. **Then publish:** GitHub Security Advisory (GHSA), CVE via GHSA if severity warrants. No GHSA / CVE / PoC in `backlog/` while unfixed.
5. **Contact alias:** if `security@majestanet.com` is not yet live, SECURITY.md must say GitHub private reporting is the **authoritative** intake until MX works. Do not invent a second alias.

Operator checklist (cannot be fully encoded in git): GitHub → Settings → Code security → Private vulnerability reporting **on**; Security advisories **on**.

**B. RFC 9116 `security.txt`**

Commit vendor-plane source of truth at repo-root `.well-known/security.txt` (create in the implementing PR). Fields:

```text
Contact: mailto:security@majestanet.com
Preferred-Languages: en
Canonical: <raw GitHub URL of this file on main>
Policy: <GitHub blob URL of SECURITY.md>
Expires: <ISO-8601 ≤ 1 year out; calendar reminder in the implementing PR>
```

No PGP `Encryption:` line unless a real key exists — do not invent one.

**Product serve (small, allowed):** unauthenticated `GET /.well-known/security.txt` on `cmd/api` (same body, `text/plain; charset=utf-8`), next to `/healthz` — not an API-family resource, no JWT. Optional `Cache-Control: max-age=86400`. Embed or read the committed file so install and repo cannot drift. Out of scope: a marketing site, GitHub Pages, or Control IDE chrome.

**C. CONTRIBUTING**

Expand the Security section (file already exists):

- Do not open public GitHub issues for vulnerabilities ([SECURITY.md](../../../SECURITY.md)).
- Do not paste secrets, customer data, exploit PoCs, or unfixed advisory IDs into issues, PRs, or `backlog/`.
- `backlog/` is the public **product risk** list, not a vuln tracker.

Optional: `.github/ISSUE_TEMPLATE/config.yml` `contact_links` pointing at SECURITY.md (discourages blank vuln issues). No second issue tracker.

### 2.4 BP-009 — Phase 7 remainder (honest)

[customer-automations-build.md](../customer-automations-build.md) Phase 7 (**async outbound connectors**) is **Done in product code**. [BP-009](../../../backlog/BP-009-no-in-kernel-language.md) is **Mitigated**.

**Do not re-implement** `ctx.http` / `ctx.connector`, secrets, egress, or `allowedSkills`.

Remainder:

1. **Status closeout:** mark BP-009 Phase 7 Done; refresh the plan’s “Current gaps” row that still says outbound is a Need; if no BP-009-owned product work remains, set BP-009 to **Mitigated — Keep** (import ban, Deno deny-net, sync rollback).
2. **Verification gap from the original outbound plan:** add `TestOutboundHTTPDeniedWhenNotAllowlisted` (empty/mismatch allowlist fail-closed on `OutboundHost`). Optional: one `internal/testutil` async automation that `ctx.connector` GET succeeds under allowlist and fails closed without it — only if Deno is already available in that package’s tests. Do not add npm, guest `fetch`, or sync outbound ([ADR-014](../../adr/014-customer-code-automations.md)).
3. **Not this BP:** Build-agent write loop (BP-006); ExecutionRun (BP-033); frozen IDE PS editor / review graph; Govern UI.

### 2.5 BP-047 — invoke status without a second debug object

**Shipped contract (keep):**

```http
GET /client/v1/automations          # callable catalog; no source
POST /client/v1/automations/{apiName}/runs
GET /client/v1/automations/runs/{id}
```

AuthZ: Client scope + `AssertCanRunAutomation`; run-as = caller ([ADR-014](../../adr/014-customer-code-automations.md)). GET run: `jobs` row `job_type=automation.run` and (`actor.IsAdmin` OR `payload.actorId = caller`). Async → `202` + job `id`; sync → inline result. This **is** invoke status until BP-033 Phase 3.

**Thin remainder (no ExecutionRun):**

- Do not add `DebugRun`, `InvokeStatus`, `automation_run_log`, or any table/object that duplicates `jobs` or pre-creates ExecutionRun.
- Optional JSON field **omitted** until the object exists.
- Add HTTP tests for OAuth authorize start (inactive connector / wrong auth type / egress deny) and callback fail-closed (bad state) if still missing. Token ciphertext never in responses (`hasSecret` / status only).

**Projection (only if BP-033 Phase 3 already seeded `ExecutionRun`):**

- BP-033 writers create the `ExecutionRun` row (Kind=`automation`, `JobId`=jobs.id).
- BP-047 only **projects** an optional `executionRunId` on POST `202` and GET run JSON. Same GET AuthZ as today (caller-owned job or admin). Reading the debug object itself requires `debug.read` via normal Client sobject GET — do not bypass that on the invoke status route.
- If `internal/seed` / `internal/packages` has no `ExecutionRun`, **stop**. Do not seed it from this remainder.

**Frozen:** Govern connector catalog. No new Govern UI. Per-user OAuth, inbound provider webhooks, sync outbound, Deno `fetch` remain non-goals of [integrations-build-plan.md](../integrations-build-plan.md).

---

## 3. Concrete agentic build plan

### Phase A — BP-008 logs exporter

- **Owner:** `worker-jobs` (shared `internal/otel`) + `deploy-ops` (Helm / `.env.example` / ops docs)
- **Packages allowed:** `internal/otel`, `internal/logging`, `internal/config`, `cmd/api`, `cmd/worker`, `deploy/helm/one`, `.env.example`, `docs/ops.md`, `docs/tech-stack.md`, `docs/security.md`, `go.mod` / `go.sum`
- **Forbidden:** `tools/control-ide/**`; collector sidecar; replacing slog stdout; customer `ExecutionLogEntry` writers
- **Files likely:** `internal/otel/otel.go`, `otel_test.go`, `internal/logging/logging.go`, `internal/config/config.go`, `cmd/api/main.go`, `cmd/worker/main.go`, Helm `values.yaml` + deployment templates
- **Tests:** `go test ./internal/otel/...` — unset endpoint still no-op; `OTEL_LOGS_EXPORTER` empty/`none` does not start a log provider; redaction drops `authorization` / token keys; existing `TestSetupNoopWhenUnset` / `TestRedactURL` stay green. `go test ./internal/logging/...` if a handler unit test is added
- **Exit:** operator can set endpoint + `OTEL_LOGS_EXPORTER=otlp` and see log records at the collector **without** losing stdout JSON; default installs (endpoint unset **or** logs exporter unset) behave as today
- **Depends:** none

### Phase B — BP-008 queue-depth metrics

- **Owner:** `worker-jobs`
- **Packages allowed:** `internal/otel`, `internal/worker`, `internal/config` (only if a new env is required — prefer none)
- **Forbidden:** inventing `job.class` here; `tools/control-ide/**`; putting `last_error` on metrics
- **Files likely:** `internal/worker/loop.go` or a small `metrics.go`; `internal/otel` meter helpers
- **Tests:** `go test ./internal/worker/...` — gauge callback does not panic with empty queue; if classes exist, counts split by class. Skip the whole phase if Phase 2 schema is absent (exit = comment in BP-008 pointing at BP-033)
- **Exit:** with BP-033 classes, `one.jobs.queue_depth{job.class=...}` is observable when OTLP metrics are on
- **Depends:** [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) Phase 2 (job classes / slots). Phase 5 of the isolation plan names this pairing

### Phase C — BP-026 process artifacts

- **Owner:** docs / `api-families` (only for `GET /.well-known/security.txt`)
- **Packages allowed:** `SECURITY.md`, `CONTRIBUTING.md`, `.well-known/security.txt`, optional `.github/ISSUE_TEMPLATE/config.yml`, `internal/httpapi` (public well-known handler), `internal/httpapi` tests, `docs/security.md` one-line pointer
- **Forbidden:** `backlog/` vuln detail; `tools/control-ide/**`; DRM; a second issue tracker; inventing PGP keys
- **Files likely:** those listed above; maybe `internal/httpapi/server.go` route register
- **Tests:** `go test ./internal/httpapi/...` — `GET /.well-known/security.txt` is 200, `text/plain`, contains `Contact:` and `Policy:`, no auth. Docs-only slice needs no Go test if the HTTP route is deferred
- **Exit:** SECURITY.md states publish-after-fix GHSA; security.txt committed (and served if route included); CONTRIBUTING forbids secrets/PoCs in public issues; BP-026 remaining list matches
- **Depends:** none (mailbox liveness is an operator check documented in SECURITY.md)

### Phase D — BP-009 Phase 7 closeout + fail-closed test

- **Owner:** `worker-jobs` (test) + docs
- **Packages allowed:** `internal/automation` tests, `backlog/BP-009-no-in-kernel-language.md`, `docs/architecture/customer-automations-build.md`
- **Forbidden:** re-implementing outbound; guest net; npm allowlist; Govern/IDE chrome; seeding ExecutionRun
- **Files likely:** `internal/automation/outbound_test.go`; BP-009; customer-automations-build.md “Current gaps” + Phase 7/BP-009 table
- **Tests:** `go test ./internal/automation/...` — deny when allowlist empty or host not listed. Optional `go test ./internal/worker/...` connector GET if added
- **Exit:** BP-009 Phase 7 marked Done; allowlist fail-closed covered; no new outbound API
- **Depends:** none (outbound already shipped)

### Phase E — BP-047 thin invoke-status + OAuth HTTP tests

- **Owner:** `api-families` + `authz-security`
- **Packages allowed:** `internal/httpapi` (client invoke + outbound OAuth routes/tests), `docs/customer-connect.md` (status contract sentence only)
- **Forbidden:** new debug objects; Govern UI; per-user OAuth; sync outbound; pretending `ExecutionRun` exists
- **Files likely:** `internal/httpapi/outbound_oauth_routes.go` + new `outbound_oauth_routes_test.go`; possibly `client_automation_runs_test.go` GET-run actor isolation (extend if a case is missing)
- **Tests:** `go test ./internal/httpapi/...` — authorize rejects non-OAuth / inactive / not-allowlisted token URL; callback rejects unknown state; GET run 404 for another principal (existing pattern in `client_automation_runs_test.go`)
- **Exit:** invoke status remains job GET; OAuth HTTP fail-closed is tested
- **Depends:** none

### Phase F — BP-047 ExecutionRun projection (conditional)

- **Owner:** `api-families` (JSON field) — **not** `db-backend-perf` unless objects already exist
- **Packages allowed:** `internal/httpapi/client_automation_runs.go` (+ tests)
- **Forbidden:** creating `ExecutionRun` / `ExecutionLogEntry` / HV tables / `debug.read` seed — that is BP-033 Phase 3
- **Files likely:** `client_automation_runs.go` POST/GET JSON
- **Tests:** if objects exist: POST/GET include `executionRunId` matching the platform-written row for that job; still omit/404 for other principals. If objects **do not** exist: do not add the field; do not write this phase
- **Exit:** callers poll the same GET run URL; when debug objects exist they receive an id, not a parallel status schema
- **Depends:** [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) Phase 3 **done in tree** (seeded managed objects + writers). If not done, Phase E is the entire BP-047 remainder

---

## 4. Explicit non-goals

- Re-implementing OTLP traces/metrics, outbound connectors, Client invoke, or OAuth token lifecycle
- Collector / Fluent Bit sidecar; Node/`tsx` packaging (closed)
- Using OTEL or `audit_log` as the customer automation debugger
- Creating `ExecutionRun` / `ExecutionLogEntry` from BP-047 or BP-009
- Per-user OAuth, inbound provider webhooks, sync outbound, Deno `fetch` / npm
- New Control IDE Govern (or any) chrome; unfreezing BP-015/016/018/019/021/024/027/034
- Publishing live vulnerability detail, PoCs, or unfixed GHSA IDs in `backlog/`
- A second public risk tracker parallel to `backlog/`
- Claiming OSS closes AuthZ/identity BPs
- `govulncheck` CI gate (optional later Keep — not in BP-026 remaining list)

---

## 5. Agentic implementation prompt(s)

### Prompt 1 — BP-008 optional OTEL logs exporter — **Keep (in tree)**

Do not paste this prompt. `OTEL_LOGS_EXPORTER=otlp` shipped (`internal/otel/logs.go`). Queue-depth metrics stay gated on BP-033 Phase 2. Next executable remainder is **Prompt 2 (BP-026 GHSA policy)**.

### Prompt 1 — BP-008 optional OTEL logs exporter (historical)

```text
Implement Majesta One BP-008 remainder: optional OTEL logs exporter. Queue-depth metrics only if BP-033 Phase 2 job classes already exist in this tree.

Read first:
- docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md (§2.1–2.2, Phase A–B)
- docs/architecture/outbound-otel-build-plan.md
- docs/architecture/customer-runtime-isolation-build-plan.md (Phase 5 OTEL hook only)
- backlog/BP-008-production-packaging.md
- docs/tech-stack.md, docs/ops.md, docs/security.md
- internal/otel/otel.go, internal/logging/logging.go, internal/config/config.go
- cmd/api/main.go, cmd/worker/main.go

Edit scope (allowed):
- internal/otel, internal/logging, internal/config, cmd/api, cmd/worker
- deploy/helm/one (optional OTEL_LOGS_EXPORTER), .env.example
- docs/tech-stack.md, docs/ops.md, docs/security.md, backlog/BP-008-production-packaging.md
- go.mod / go.sum for otlploghttp + otelslog (or equivalent) — update tech-stack in the same change set

Do:
1. Keep stdout JSON slog. Add slog MultiHandler fan-out to OTLP logs only when OTEL_EXPORTER_OTLP_ENDPOINT is set AND OTEL_LOGS_EXPORTER=otlp. Default logs exporter is none.
2. Attach current span context on log records. Reuse existing resource attrs.
3. Redact/drop authorization, token, ciphertext, cookie keys on the OTEL log handler. Never export secrets.
4. No-op when endpoint unset (existing TestSetupNoopWhenUnset must stay green). Shutdown must flush the log provider if started.
5. Queue-depth: IF jobs have a BP-033 class/slot dimension, add one.jobs.queue_depth / one.jobs.running gauges (and throttle/OOM counters only if those events already exist). IF class does not exist, do not invent it and do not ship an undifferentiated gauge that will be renamed.
6. Distroless image unchanged — no collector sidecar.

Tests:
- go test ./internal/otel/...
- go test ./internal/logging/... (if you add a handler test)
- go test ./internal/worker/... (only if you touch queue-depth)

Out of scope:
- tools/control-ide/**
- ExecutionRun / ExecutionLogEntry
- Replacing traces/metrics exporters
- Govern UI, automations SDK, OAuth
```

### Prompt 2 — BP-026 advisory policy, security.txt, CONTRIBUTING

```text
Implement Majesta One BP-026 remainder: GitHub Security Advisories publish-after-fix policy, RFC 9116 security.txt, CONTRIBUTING blurb. Process/docs; a tiny unauthenticated well-known route is allowed.

Read first:
- docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md (§2.3, Phase C)
- backlog/BP-026-oss-security-public-backlog.md
- SECURITY.md, CONTRIBUTING.md, docs/security.md
- backlog/README.md section “Security & transparency” (read only — do not edit README)
- .github/dependabot.yml (read only unless you add contact_links)

Edit scope (allowed):
- SECURITY.md, CONTRIBUTING.md, docs/security.md
- .well-known/security.txt (create)
- optional .github/ISSUE_TEMPLATE/config.yml contact_links to SECURITY.md
- internal/httpapi public GET /.well-known/security.txt + test (same body as the file)
- backlog/BP-026-oss-security-public-backlog.md (remaining list / status)

Do:
1. SECURITY.md: intake = GitHub private vulnerability reporting OR security@majestanet.com; publish GHSA only after a fix on the latest v* tag; no unfixed GHSA/PoC in backlog/; if the mailbox is not live, GitHub private reporting is authoritative.
2. Commit .well-known/security.txt (Contact, Preferred-Languages: en, Canonical, Policy, Expires ≤ 1 year). No invented PGP key.
3. CONTRIBUTING Security: no public vuln issues; no secrets, customer data, exploit PoCs, or unfixed advisory IDs in issues/PRs/backlog.
4. Optional: serve GET /.well-known/security.txt unauthenticated as text/plain.

Tests:
- go test ./internal/httpapi/... if you add the well-known route (200, no auth, Contact: present)

Out of scope:
- tools/control-ide/**
- Product AuthZ/identity work (BP-003/006/013/017)
- Putting advisory detail into backlog/
- A second tracking system
- Dependabot redesign, govulncheck CI (unless already trivial — not required)
```

### Prompt 3 — BP-009 Phase 7 closeout (outbound is already shipped)

```text
Implement Majesta One BP-009 Phase 7 remainder. Async outbound connectors are ALREADY shipped. Do not re-implement ctx.http / ctx.connector. Align docs and add the missing allowlist fail-closed test.

Read first:
- docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md (§1 inventory, §2.4, Phase D)
- docs/architecture/customer-automations-build.md (Phase 7 is marked Done — believe the code)
- docs/adr/014-customer-code-automations.md
- backlog/BP-009-no-in-kernel-language.md (Mitigated — do not reopen Phase 7)
- docs/architecture/outbound-otel-build-plan.md (verification list)
- internal/automation/outbound.go, outbound_test.go, internal/egress/egress.go

Edit scope (allowed):
- internal/automation/outbound_test.go (and worker test only if you add a real connector GET case)
- backlog/BP-009-no-in-kernel-language.md
- docs/architecture/customer-automations-build.md (Current gaps row + phase status only)

Do:
1. Mark BP-009 Phase 7 Done. If nothing else is BP-009-owned, set status to Mitigated — Keep (import ban, Deno deny-net, sync rollback). Point leftover IDE write-loop at BP-006, debug objects at BP-033, invoke at BP-047.
2. Fix stale “Current gaps” that still list outbound connectors as a Need.
3. Add TestOutboundHTTPDeniedWhenNotAllowlisted (empty allowlist and wrong host fail closed). Do not weaken SSRF/HTTPS/no-redirect rules.
4. Optional only: existing Deno guest test style — async connector GET under allowlist vs deny. Skip if it requires new infrastructure.

Tests:
- go test ./internal/automation/...
- go test ./internal/egress/... (must stay green)

Out of scope:
- Re-implementing secrets, connectors, OAuth, allowedSkills, invoke routes
- npm / guest fetch / sync outbound
- tools/control-ide/** and any Govern UI
- Creating ExecutionRun
```

### Prompt 4 — BP-047 invoke status (no second debug object)

```text
Implement Majesta One BP-047 remainder for Client automation invoke status. Do NOT create ExecutionRun, ExecutionLogEntry, DebugRun, or any second debug/status object. BP-033 Phase 3 is still the owner of those objects and is Open unless you prove otherwise in this tree.

Read first:
- docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md (§2.5, Phase E–F)
- docs/architecture/integrations-build-plan.md (Debug row: thin job status now; project later)
- backlog/BP-047-integrations-callable-oauth.md
- backlog/BP-033-customer-runtime-isolation.md (Phase 3 Open?)
- docs/adr/014-customer-code-automations.md (run-as = caller)
- internal/httpapi/client_automation_runs.go, client_automation_runs_test.go
- internal/httpapi/outbound_oauth_routes.go, internal/connectoroauth/*

Gate check (required before any projection):
- Search internal/seed and internal/packages for object apiName ExecutionRun.
- If ABSENT: implement only the thin remainder below. Do not seed ExecutionRun.
- If PRESENT (BP-033 Phase 3 already merged): you may add optional executionRunId on POST 202 and GET /client/v1/automations/runs/{id}, populated from the platform-written row keyed by jobs.id. Same GET AuthZ as today (caller actorId or admin). Reading log lines stays debug.read via sobjects — do not bypass on this route.

Thin remainder (always):
1. Keep GET /client/v1/automations/runs/{id} as invoke status SoT (jobs row: status, lastError, timestamps, input). Document one sentence in docs/customer-connect.md if missing: poll this URL; ExecutionRun is BP-033.
2. Add HTTP tests for OAuth authorize/callback fail-closed (wrong authType, inactive connector, egress deny, unknown state). No tokens/ciphertext in JSON.
3. No new Govern UI. Connector catalog chrome is frozen.

Tests:
- go test ./internal/httpapi/...
- go test ./internal/connectoroauth/...

Out of scope:
- tools/control-ide/**
- Per-user OAuth, inbound provider webhooks, sync outbound, Deno fetch
- Inventing job-class isolation (BP-033)
- Re-implementing Client invoke or OAuth token store
```
