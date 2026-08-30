# Agent routing — spawn and focus

How to pick the right docs, packages, and domain agents before editing Majesta One.

## Read order (every task)

1. Root [`AGENTS.md`](../../AGENTS.md) — global constraints and task → playbook table.
2. This file + [`module-map.md`](./module-map.md) — decide domain and code scope.
3. The **one** domain playbook that matches (below).
4. Relevant ADR(s) and `backlog/BP-*.md` items listed in that playbook.
5. Only then open packages listed in the module map.

Do not load every playbook. Prefer a thin, correct slice.

## Task → playbook → domain agent

| If the task is about… | Playbook | Domain agent | Stay in packages |
|---|---|---|---|
| Data model, seed, JSONB store, query/indexes | [agent-data-architecture.md](./agent-data-architecture.md) | `db-backend-perf` | `internal/dataengine`, `metadata`, `seed`, `db`, `migrations/` |
| Platform actions (Client `/actions`, `lead.convert`, `quote.accept`, guest `invokeAction`) | [platform-actions-build-plan.md](./platform-actions-build-plan.md) + data + api-families (+ worker for SDK) | `api-families` then `db-backend-perf` then `worker-jobs` | `internal/actions`, `packages`, `httpapi`, `dataengine`, `automation`, `seed` |
| Cross-object search (`/client/v1/search`) + Operate find bar | [cross-object-search-build-plan.md](./cross-object-search-build-plan.md) + data + api-families + control-ide playbooks | `db-backend-perf` then `api-families` then `control-ide` | Go: `dataengine`, `metadata`, `httpapi`, `worker`, `migrations/`; IDE: `tools/control-ide/**` only |
| Operate graph surface (click, drop Tools, hygiene, graph search) | [ADR-028](../adr/028-operate-graph-surface.md) + [agent-control-ide.md](./agent-control-ide.md) | `control-ide` | `tools/control-ide/**` only (`internal/rungraph` only if compact/mount allowlist must lockstep) |
| JWT, API keys, Roles, scopes, permission sets, principals; refresh tokens | [agent-authz.md](./agent-authz.md) + [refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) | `authz-security` | `internal/authz`, `httpapi/auth_routes.go`, `migrations/` (`refresh_tokens`) |
| User metadata object, `users.data`, SCIM custom attrs, install provisioning | [agent-authz.md](./agent-authz.md) + [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md) | `authz-security` (+ `db-backend-perf` for kernel User seed) | `internal/authz`, `internal/db`, `internal/metadata`, `internal/scim`, `internal/seed`, `migrations/` |
| Which API family owns a route; httpapi wiring | [agent-api-families.md](./agent-api-families.md) | `api-families` | `internal/httpapi` (+ domain package the handler calls) |
| Promote, peers, bundles, multi-env, Ops rolls, ECS | [agent-deploy.md](./agent-deploy.md) | `deploy-ops` | `internal/deploy`, `ops`, `httpapi/deploy_routes.go`, `ops_routes.go`, `deploy/` |
| Jobs, outbox, webhooks delivery, worker concurrency | [agent-worker.md](./agent-worker.md) | `worker-jobs` | `internal/worker`, `cmd/worker`, related migrations |
| Agent runtime (job-class harness, builder MCP, builder DX) | [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) + [agent-api-families.md](./agent-api-families.md) | `api-families` then `worker-jobs` | `internal/agentharness`, `internal/mcp`, `internal/httpapi`, `internal/worker`; **not** `tools/control-ide` |
| Hosted agent tool loop (`/agents/runs` executes MCP tools) | [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) + api-families + worker | `api-families` then `worker-jobs` | `internal/agentloop`, `internal/inference`, `internal/mcp`, `internal/httpapi`, `internal/worker`, `internal/agentharness`; **not** `tools/control-ide` |
| Control IDE UI, Electron, Vite, Vitest, panels (optional client; lockstep OK for [BP-065](../../backlog/BP-065-ide-backend-coupling.md); demo honesty [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md)) | [agent-control-ide.md](./agent-control-ide.md) · [ide-demo-client-uplift-build-plan.md](./ide-demo-client-uplift-build-plan.md) | `control-ide` | `tools/control-ide/**` only (plus mapped Go when the task is install-coupling cleanup or a BP-066 API-gap handoff) |
| Public docs site (`one.majesta.net`) — allowlisted markdown, Astro Starlight publisher, merge-event docs updates | [agent-public-docs.md](./agent-public-docs.md) · [public-docs-site-build-plan.md](./public-docs-site-build-plan.md) | `docs-publisher` | Allowlisted public `docs/` + `tools/one-docs/**` (Phase 1–2: `scripts/docs-impact.sh`, `docs-impact.yml`, `netlify.toml`); **not** `tools/control-ide` or Go handlers |
| Latency, N+1, pool, JSONB indexes (cross-cutting) | data playbook + module map | `db-backend-perf` | packages named in the agent description |

## Plane fence

| Track | Edit | Do not edit |
|---|---|---|
| **IDE** (`control-ide`) | `tools/control-ide/**` (+ IDE docs in the playbook) | `cmd/`, `internal/`, `migrations/`, `deploy/` |
| **Backend** (Go domain agents) | Mapped Go / deploy paths | `tools/control-ide/**` |
| **Docs** (`docs-publisher`) | Allowlisted public `docs/` + `tools/one-docs/**` (+ impact workflow when scaffolding) | Go product handlers; `tools/control-ide/**` |

Cross-plane work must cite **both** playbooks: API ownership stays with the Go agent; the IDE agent consumes JWT Bearer family routes **or** moves state local so the Go route can be deleted ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)). Do not add Electron-only consumers for new agent-runtime APIs ([ADR-030](../adr/030-install-agent-runtime.md)).

## Spawning subagents

1. **Name the concern** in the parent prompt (e.g. “AuthZ Role scopes” or “Control IDE Connect panel”, not “fix the API”).
2. **Attach the playbook path** and the module-map row for that concern.
3. **List allowed packages** explicitly; tell the subagent not to edit outside that set unless the task requires a documented cross-boundary change.
4. **Point at open BPs** from [`backlog/README.md`](../../backlog/README.md) that touch the area.
5. Prefer the domain specialist in `.cursor/agents/` whose description matches the paths — do not invent a parallel stack or a second product tree.
6. For IDE work, verify with `npm test` / `make test-ide`; for Go work, verify with `go test` / `make test`. Do not tell an IDE agent to run product `make ci` as its primary check.
7. For public-docs work, verify with `make docs-check` when that target exists. Do not tell a docs-publisher agent to run product `make ci` as its primary check.

## Focus rules

- `cmd/api`, `cmd/worker`, `cmd/migrate` are wiring only — expect most backend edits under `internal/`.
- Cross-family work (e.g. Metadata write + Deploy reject list) must cite both playbooks and keep ownership rules (`managed` vs `custom`) intact.
- When closing or de-risking a backlog item, update the BP file and the table in `backlog/README.md`.
- Vendor/agent docs live under `docs/`, `backlog/`, `.cursor/` — they must never enter product images (see [monorepo.md](../monorepo.md)).
- Control IDE under `tools/control-ide` is vendor plane (ADR-012) — never widen the product image COPY allowlist to include it. Spawn `control-ide` for install-coupling lockstep ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)); do not spawn it to add Electron-only product chrome ([ADR-030](../adr/030-install-agent-runtime.md)).
- Spawn `docs-publisher` for allowlisted public docs / `tools/one-docs` ([agent-public-docs.md](./agent-public-docs.md)). Do not spawn it to change Go routes or Control IDE.

## Architecture index

Start at [README.md](./README.md) for the full document map.
