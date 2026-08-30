# Outbound integrations + operator OTEL — build plan

**Active plan** for [BP-008](../../backlog/BP-008-production-packaging.md) (operator OpenTelemetry) and [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) (customer outbound integrations).

**Playbooks:** [agent-worker.md](./agent-worker.md) · [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md) · [agent-data-architecture.md](./agent-data-architecture.md)  
**Domain agents:** `worker-jobs`, `api-families`, `authz-security`, `db-backend-perf`  
**Related:** [ADR-010](../adr/010-customer-agentic-platform.md) · [ADR-014](../adr/014-customer-code-automations.md) · [automation-sdk.md](../automation-sdk.md) · [customer-agents.md](../customer-agents.md) · [BP-006](../../backlog/BP-006-agent-guardrails.md) · [BP-033](../../backlog/BP-033-customer-runtime-isolation.md)

---

## Thesis

> Customers call external HTTPS APIs from **async** Deno automations through a **platform-owned** host RPC (`ctx.http` / `ctx.connector`), using **secret refs** and an **egress allowlist**. Agents **grant named automations as skills** on AgentSpec (metadata contract shipped). **`invoke_skill`** on MCP and the hosted loop is shipped; remainders are in [03-bp-014-skill-invoke.md](./agentic-remainders/03-bp-014-skill-invoke.md). Operators get **OTLP** traces/metrics; customer debug objects stay BP-033.

```text
Async automation TS
  → one:automation ctx.http | ctx.connector
  → Go host (Deno deny-net)
  → secretcrypt + egress allowlist + SSRF checks
  → HTTPS outbound
  → OTEL span (operator) + job/audit (customer)

AgentSpec.allowedSkills[] → automation apiNames (skill grants)
```

---

## Locked decisions

| Decision | Choice |
|---|---|
| Primary outbound consumer | Async automations (ADR-014 Phase 7) |
| Guest networking | Never — Deno default-deny; egress only via Go |
| Sync automations | No outbound |
| Secrets | `install_secrets` + `enc:v1:`; never in Deploy/Git |
| Connectors | `install_connectors` (base URL, secret ref, methods) |
| Egress | `install_egress_allowlist` + DNS/IP blocklist (reuse webhook rules) |
| Agent surface | `allowedSkills` = automation apiNames; **no** BYO LLM in this plan |
| Operator vs customer telemetry | OTEL = BP-008; ExecutionRun/LogEntry = BP-033 |
| Full LLM multi-tool loop | Out of scope (BP-006 — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md)) |

---

## Phases

### Phase 1 — Docs + backlog alignment

Build plan (this file), BP-008/014 updates, tech-stack/ops/security, automation-sdk + customer-agents cross-links.

### Phase 2 — OTEL foundation

`internal/otel` + config (`OTEL_EXPORTER_OTLP_ENDPOINT`); wire `cmd/api` / `cmd/worker`; HTTP middleware spans; no-op when unset; resource attrs `PRODUCT_VERSION`, `CUSTOMER_ID`, `INSTALL_ID`.

### Phase 3 — Secrets / connectors / egress Metadata

Kernel migration; Metadata CRUD; Deploy snapshot **refs only**; re-bind secrets after promote.

### Phase 4 — Automation host HTTP

Async `ctx.http` / `ctx.connector`; sync reject; header allowlist; body/time caps; tests with `httptest`.

### Phase 5 — AgentSpec skills

`allowed_skills` on `agent_playbooks`; Metadata/Deploy/worker validation.

### Phase 6 — OTEL outbound + closeout

Instrument egress spans (redacted URLs); document env; backlog status.

---

## Security checklist

| Front | Control |
|---|---|
| SSRF | HTTPS; no redirects; block private/metadata IPs |
| Secrets | `enc:v1`; Metadata `hasSecret`; Deploy refs only |
| Guest escape | Deno deny-net; import ban; SDK-only HTTP |
| Sync integrity | Outbound forbidden in sync |
| Agent overreach | Skills must name real automations; run-as + PS `canRun` when invoked |
| OTEL | Never export Authorization / ciphertext |

---

## Non-goals

- Hosted LLM / BYO model keys on AgentSpec
- Full agent tool loop (BP-006 — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md))
- Customer `fetch`/npm inside Deno
- Sync outbound
- Control IDE connector UI (contract only; follow [customer-ide-ux.md](../customer-ide-ux.md))
- Outbound OAuth authorize/refresh (extends this plan under [integrations-build-plan.md](./integrations-build-plan.md) / [BP-047](../../backlog/BP-047-integrations-callable-oauth.md))

---

## Verification

- `go test` egress, connector host RPC, sync-http rejection, OTEL no-op
- Async automation connector GET under allowlist; missing allowlist fails closed
- Distroless image unchanged (no collector sidecar)
