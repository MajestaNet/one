# Module map (agents)

Single router from concern → packages → key files. Prefer these paths over broad `cmd/` exploration: binaries are thin; domain logic lives under `internal/`.

Use with [agent-routing.md](./agent-routing.md) and the matching domain playbook.

## Entry binaries (thin)

| Binary | Package | Role |
|---|---|---|
| API | `cmd/api` | Wires config, pool, authz, httpapi server |
| Worker | `cmd/worker` | Wires pool + `internal/worker` claim/process loop |
| Migrate | `cmd/migrate` | Applies kernel SQL under `migrations/` |

## Concern → packages

| Concern | Primary packages | Key files | Playbook | Related |
|---|---|---|---|---|
| Record CRUD / query | `internal/dataengine` | `service.go`, `query.go`, `record.go`, `projections.go`, `limits.go` | [agent-data-architecture.md](./agent-data-architecture.md) | ADR-003, ADR-013, BP-001 |
| Cross-object search | `internal/dataengine`, `internal/metadata`, `internal/httpapi`, `internal/worker`, `migrations/` | `search.go` (planned); `POST /client/v1/search`; `searchable` metadata; `search.reindex` | [cross-object-search-build-plan.md](./cross-object-search-build-plan.md) | BP-043, BP-020, ADR-003 |
| External ID / upsert / Bulk ingest | `internal/dataengine`, `internal/metadata`, `internal/httpapi`, `internal/worker` | upsert + unique projections; `server.go` / ingest routes; `ingest.process` job | [external-id-upsert-bulk-build-plan.md](./external-id-upsert-bulk-build-plan.md) | BP-041 |
| Peer-sourced data packs | `internal/datapack`, `cmd/one` | `manifest.go`, `apply.go`; `datapack validate|apply` | [external-id-upsert-bulk-build-plan.md](./external-id-upsert-bulk-build-plan.md) | BP-041 |
| Metadata describe/write | `internal/metadata` | `service.go`, `write.go`, `types.go`, `cache.go` | [agent-data-architecture.md](./agent-data-architecture.md) | ADR-002, ADR-008 |
| Managed seed (`core` + modules) | `internal/seed`, `internal/packages` | `packages.go`, `modules.go`, `module_*.go`; `registry.go` | [agent-data-architecture.md](./agent-data-architecture.md) | BP-007, BP-049, ADR-020 |
| Platform actions | `internal/actions`, `internal/packages`, `internal/httpapi`, `internal/dataengine`, `internal/automation`, `internal/seed` | `ActionDef` on modules; `GET/POST /client/v1/actions/{apiName}`; guest `invokeAction`; `lead.convert` | [platform-actions-build-plan.md](./platform-actions-build-plan.md) | ADR-029, BP-061, BP-049, BP-046 |
| Kernel DDL / stores | `migrations/`, `internal/db` | numbered SQL + `meta/_journal.json`; `pool.go`, `users.go`, `migrate.go` | [agent-data-architecture.md](./agent-data-architecture.md) | ADR-001 |
| Client HTTP | `internal/httpapi` | `client_extras.go`, `agent_run_stream.go`, `server.go` (sobjects/query/composite/bulk; `/search`); `client_actions.go` (`GET/POST /actions`) | [agent-api-families.md](./agent-api-families.md) | ADR-004, ADR-029, BP-006, BP-052, BP-043, BP-061 |
| API revision pin | `internal/compat`, `internal/httpapi`, `internal/config` | `revision.go`, `product.go`; `httpapi/revision.go` + middleware; `API_REVISION_*` | [ide-api-version-compatibility-build-plan.md](./ide-api-version-compatibility-build-plan.md) | ADR-025, BP-025 |
| Inference (BYO + DO native) | `internal/inference`, `internal/httpapi`, `internal/worker` | `client.go`, `store.go`; `inference_routes.go`; `deploy_cloud_routes.go` (inference); agent run SSE | [inference-build-plan.md](./inference-build-plan.md) | BP-052, BP-014, BP-030 |
| Agent section harness | `internal/agentharness`, `internal/httpapi`, `internal/deploy`, `internal/seed`, `internal/worker`, `migrations/` | `catalog.go`, `apply.go`; Metadata playbooks + `/agents/harnesses`; Deploy snapshot/apply; starter clone; stream + worker `Apply` | [agent-section-harness-build-plan.md](./agent-section-harness-build-plan.md) · **job class:** [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) | BP-053 (keep), BP-064, BP-006, ADR-010, ADR-030 |
| Metadata HTTP | `internal/httpapi` | `metadata_routes.go` | [agent-api-families.md](./agent-api-families.md) | ADR-004 |
| Deploy HTTP + engine | `internal/httpapi`, `internal/deploy`, `internal/digitalocean` | `deploy_routes.go`, `deploy_cloud_routes.go`; `service.go`, `async.go`, `cloud.go`, `cloud_host.go`, `cloud_types.go`, `apply.go`, `trust.go`, `initialize_repo.go`, `gitremote/`; `digitalocean/client.go` | [agent-deploy.md](./agent-deploy.md) | ADR-004, BP-030, BP-031, multi-env-deploy, [deploy-cloud-agnostic-build-plan.md](./deploy-cloud-agnostic-build-plan.md) |
| Customer package format | `internal/customerrepo`, `cmd/one` | `pack.go`, environments/change helpers; CLI auth/project/org validate|deploy|retrieve | [customer-repo.md](../customer-repo.md) · [one-cli-build-plan.md](./one-cli-build-plan.md) | ADR-012, BP-031, BP-032, BP-048 |
| Ops HTTP + rolls | `internal/httpapi`, `internal/ops` | `ops_routes.go`; `engine.go`, `ecs.go`, `roller.go` | [agent-deploy.md](./agent-deploy.md) | ADR-007, BP-002/011 |
| AuthN / AuthZ | `internal/authz`, `internal/authlogin`, `internal/identity`, `internal/httpapi`, `internal/integration`, `internal/scim` | `scopes.go`, `jwt.go`, `refresh_token.go`, `apikey.go`, `object_perms.go`, `users.go`; social broker (`authlogin`); `auth_routes.go`; `principal_routes.go`; `directory_tag_routes.go`; `scim_routes.go` + `internal/scim` (`group.go`); `internal/db/directory_tags.go`; `integration_routes.go` + `internal/integration` (Connected Apps); kernel `refresh_tokens` ([refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md)) | [agent-authz.md](./agent-authz.md) · [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md) · [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md) · [refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) | ADR-006, ADR-015, ADR-009, BP-013/003/017/058/063 |
| MCP adapter | `internal/mcp`, `internal/httpapi` | `gateway.go`, `protocol.go`; `mcp_routes.go` | [customer-agents.md](../customer-agents.md) · [customer-connect.md](../customer-connect.md) · [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) | ADR-010, ADR-030, BP-006, BP-064 |
| Hosted agent tool loop | `internal/agentloop`, `internal/inference`, `internal/mcp`, `internal/httpapi`, `internal/worker`, `internal/agentharness` | `loop.go`, `persist.go`; `mcp.CallTool` as run Actor; worker `agent.run` + SSE approve/resume | [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) | BP-006 remaining, BP-052, ADR-010, ADR-030 |
| Customer MCP scaffold (vendor) | `tools/one-mcp` | stdio + JWT Client API tools | [customer-connect.md](../customer-connect.md) | ADR-010; not in product image |
| Public docs site (vendor) | `tools/one-docs` | Starlight publisher; `content-map.yaml`; include allowlisted `docs/` at build time | [agent-public-docs.md](./agent-public-docs.md) · [public-docs-site-build-plan.md](./public-docs-site-build-plan.md) | [BP-067](../../backlog/BP-067-public-docs-site.md); not in product image |
| Jobs / outbox | `internal/worker` | `claim.go`, `process.go`, `loop.go` | [agent-worker.md](./agent-worker.md) | SKIP LOCKED leases, [BP-033](../../backlog/BP-033-customer-runtime-isolation.md) |
| Customer code automations | `internal/automation`, `internal/worker`, `internal/dataengine`, `internal/authz`, `internal/customerrepo` | Import ban + metadata (Phase 2); Deno executor (Phase 4); sync tx (Phase 3); PS `automationAccess`; guest `invokeAction` → platform actions | [customer-automations-build.md](./customer-automations-build.md) · [platform-actions-build-plan.md](./platform-actions-build-plan.md) | ADR-014, ADR-029, BP-009, BP-033, BP-061 |
| Runtime isolation / debug objects | `internal/httpapi`, `internal/worker`, `internal/automation`, `internal/deploy`, `internal/seed` | Admission lanes (`admission.go`); async Deploy (`async.go`); job-class slots (Phase 2); `ExecutionRun` + `ExecutionLogEntry` (HV, Phase 3) | [customer-runtime-isolation-build-plan.md](./customer-runtime-isolation-build-plan.md) | BP-033 |
| Config / logging / OTEL | `internal/config`, `internal/logging`, `internal/otel` | `config.go`, `logging.go`, `otel.go` | [outbound-otel-build-plan.md](./outbound-otel-build-plan.md) | BP-008 |
| Outbound connectors / egress | `internal/egress`, `internal/automation`, `internal/connectoroauth`, Metadata + Auth routes | host HTTP RPC; allowlist; OAuth authorize/callback | [outbound-otel-build-plan.md](./outbound-otel-build-plan.md) · [integrations-build-plan.md](./integrations-build-plan.md) | BP-014, BP-047 |
| Client automation invoke | `internal/httpapi`, `internal/worker`, `internal/authz` | `POST /client/v1/automations/{apiName}/runs`; run-as = caller | [integrations-build-plan.md](./integrations-build-plan.md) | BP-047, ADR-014 |
| Install packaging | `deploy/` | `Dockerfile`, `docker-compose.yml`, `helm/`, `digitalocean/`, optional `aws/ecs/` | [agent-deploy.md](./agent-deploy.md) | monorepo, [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md), [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md), [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) |
| Control IDE (vendor, optional client) | `tools/control-ide` | `src/main/`, `src/preload/`, `src/renderer/` (`App.tsx`, `theme.ts`, `workspace/`, `run/`, `run/graph/`, `api.ts`, panels), `package.json` | [agent-control-ide.md](./agent-control-ide.md) | ADR-012, ADR-030; lockstep OK for [BP-065](../../backlog/BP-065-ide-backend-coupling.md) |
| Go integration harness | `internal/testutil` | `db.go`, `http.go` | data / api-families playbooks | prefer for new DB+HTTP integration tests |

## HTTP family → route file

| Family | Path prefix | Register / handlers |
|---|---|---|
| Client | `/client/v1` (+ `/v1` alias) | `server.go` (CRUD/query) + `client_extras.go` (events, agent runs, audit) + `client_actions.go` (`GET/POST /actions`) |
| Metadata | `/metadata/v1` | `metadata_routes.go` |
| Deploy | `/deploy/v1` | `deploy_routes.go` → `internal/deploy` |
| Ops | `/ops/v1` | `ops_routes.go` → `internal/ops` |
| Auth | `/auth/v1` | `auth_routes.go` → `internal/authz` |
| SCIM | `/scim/v2` | `scim_routes.go` → `internal/scim` + Client identity SoR |

Shared middleware (request-id, body limit, **admission lanes**, API revision pin, access log): `internal/httpapi/middleware.go` + `admission.go` + `revision.go`. Family paths (and `/mcp`) accept `One-API-Revision` or `/r{N}/`; `/version` `/healthz` `/readyz` `/scim/v2` stay revision-agnostic.

## Backlog Area → packages

| Typical BP `Area:` | Resolve via |
|---|---|
| `internal/dataengine`, `migrations/` | dataengine + migrations row above |
| `internal/authz`, `/auth/v1` | authz row |
| `internal/httpapi` agents routes | Client HTTP + [BP-006](../../backlog/BP-006-agent-guardrails.md) + [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) |
| `internal/agentloop` | Hosted loop row + [BP-006](../../backlog/BP-006-agent-guardrails.md) |
| `internal/actions`, `/client/v1/actions` | Platform actions row + [BP-061](../../backlog/BP-061-platform-actions.md) |
| `internal/worker` | worker row |
| `internal/deploy`, peers / promote | deploy row |
| Marketplace / App Platform | `deploy/digitalocean/` (App Spec + future 1-Click) + Deploy DO cloud ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)) + Path B Helm; optional community `sdk/aws` + ops row |

## Scope rules

1. Open the **primary package** first; expand only when the task crosses a documented boundary.
2. Do not put customer fixtures under `internal/seed` or `migrations/`.
3. Do not widen `deploy/Dockerfile` COPY beyond `cmd/`, `internal/`, `migrations/`.
4. Prefer family paths (`/client/v1`, …); keep `/v1` aliases for compatibility only.
5. Control IDE agents stay in `tools/control-ide/**`; Go backend agents do not edit that tree **except** install-coupling lockstep ([BP-065](../../backlog/BP-065-ide-backend-coupling.md) · [ide-backend-coupling-review.md](./ide-backend-coupling-review.md)). Do not add Electron-only product chrome ([ADR-030](../adr/030-install-agent-runtime.md)).
6. Public-docs agents stay in allowlisted `docs/` + `tools/one-docs/**` ([agent-public-docs.md](./agent-public-docs.md)); they do not edit Go handlers or Control IDE.
