# Agent playbook: API families

For agents changing HTTP routing, family ownership, or which surface owns an operation. Follow this before writing code.

## Where to look

| Concern | Path |
|---|---|
| Family design | [`docs/api-families.md`](../api-families.md) (overview), [`docs/api/`](../api/) (customer endpoint catalogs), [`docs/adr/004-three-api-families.md`](../adr/004-three-api-families.md) |
| Historical phases (shipped) | [`api-families-build-plan.md`](./api-families-build-plan.md) |
| Server wiring | `internal/httpapi/server.go` |
| Client extras (events, activity-feed, agent runs, audit) | `internal/httpapi/client_extras.go` |
| Metadata routes | `internal/httpapi/metadata_routes.go` |
| Deploy routes | `internal/httpapi/deploy_routes.go` → `internal/deploy` |
| Ops routes | `internal/httpapi/ops_routes.go` → `internal/ops` |
| Auth routes | `internal/httpapi/auth_routes.go` → `internal/authz` |
| Middleware | `internal/httpapi/middleware.go`, `revision.go` (`One-API-Revision` + `/r{N}/`) |
| Module map | [`module-map.md`](./module-map.md) |
| Related backlog | [`BP-010`](../../backlog/BP-010-three-api-families.md) (mitigated), [`BP-006`](../../backlog/BP-006-agent-guardrails.md) ([hosted tool loop](./hosted-agent-tool-loop-build-plan.md)), [`BP-064`](../../backlog/BP-064-install-agent-runtime.md) ([install as agent runtime](./agent-runtime-build-plan.md) · [ADR-030](../adr/030-install-agent-runtime.md)), [`BP-065`](../../backlog/BP-065-ide-backend-coupling.md) ([IDE coupling on the install](./ide-backend-coupling-review.md)), [`BP-025`](../../backlog/BP-025-ide-api-version-compatibility.md) ([API revision pin + discovery](./ide-api-version-compatibility-build-plan.md) · [ADR-025](../adr/025-api-revision-versioning.md)), [`BP-030`](../../backlog/BP-030-deploy-api-digitalocean-apps.md) (Deploy DO cloud), [`BP-041`](../../backlog/BP-041-record-external-id-upsert-bulk.md) ([upsert/Bulk/data packs plan](./external-id-upsert-bulk-build-plan.md)), [`BP-043`](../../backlog/BP-043-cross-object-search-api.md) / [`BP-020`](../../backlog/BP-043-cross-object-search-api.md) ([cross-object search + Operate find](./cross-object-search-build-plan.md)), [`BP-061`](../../backlog/BP-061-platform-actions.md) ([platform actions](./platform-actions-build-plan.md) · [ADR-029](../adr/029-platform-actions.md)), [`BP-063`](../../backlog/BP-063-refresh-token-sessions.md) ([refresh-token sessions](./refresh-token-session-build-plan.md)), [`BP-067`](../../backlog/BP-067-public-docs-site.md) ([public docs pointer](./public-docs-site.md)) |

## What ships today

| Family | Prefix | Owns |
|---|---|---|
| Client | `/client/v1` | Records, query, composite/bulk, **search** (`POST /search` — [cross-object-search-build-plan.md](./cross-object-search-build-plan.md)), **activity-feed** (composed activities), agent **runs**, event reads, audit read, **identity admin** (principals, credentials, Role/PS assignment), **platform actions** (`GET/POST /actions` — [platform-actions-build-plan.md](./platform-actions-build-plan.md)) |
| Metadata | `/metadata/v1` | Objects/fields/rules/automations/permission-set **definitions**/webhooks/playbook **definitions**/projections/snapshot/exposure |
| Deploy | `/deploy/v1` | Bundles, peers, customer tests, promote **customer-owned** artifacts; **planned:** DO App Platform cloud manage/scale/provision (`/cloud/digitalocean/*` — [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)) |
| Ops | `/ops/v1` | Product image confirm / roll / test gate / rollback on **this** install |
| Auth | `/auth/v1` | Token mint (Majesta One access JWT) + opaque refresh/revoke ([refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md)) |

Flat `/v1` remains a compatibility alias during transition — prefer family paths for new work.

## What to do (change types)

### A. Add or move an endpoint

1. Decide the family using ADR-004 (data ops → Client; shape model → Metadata; promote between installs → Deploy; image upgrade → Ops).
2. Register on the matching `*_routes.go` / `server.go` / `client_extras.go` file.
3. Require the matching scope (`client` / `metadata` / `deploy` / `ops`); admin only where already patterned.
4. Delegate business logic to the domain package — keep handlers thin.
5. Add `/v1` alias only if compatibility demands it; do not add new aliases by default.

### B. Agent surface (BP-006 / BP-064)

1. **Runs** → Client (`POST/GET /agents/runs`, approve) in `client_extras.go`. Hosted **execution** of tools on those runs is [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) (`internal/agentloop` + `mcp.CallTool` as the run actor). New routes are not required for v1; `awaiting_tool_approval` is a run status, not a new family.
2. **Playbooks** → Metadata in `metadata_routes.go`.
3. **MCP** → adapter over existing family paths only (`internal/mcp`); builder catalog [agent-runtime-build-plan.md](./agent-runtime-build-plan.md). Do not invent MCP-only verbs. Hosted loop executes **MCP names**; harness tokens expand at admission.
4. Do not let agents mutate managed metadata via Client. Hosted v1 catalog is Client read/write + invoke — not Metadata upserts or Deploy `org_*`.
5. Do not add Control IDE panels for new agent APIs (ADR-030). Ignore `graphCalls` / `proposal` / `boardHandoff` in the hosted executor. Do not project run-graphs, IDE conversations, preferences, or principal canvases through MCP. Prefer **removing** those Client routes once the IDE uses local state or generic family APIs ([ide-backend-coupling-review.md](./ide-backend-coupling-review.md) / [BP-065](../../backlog/BP-065-ide-backend-coupling.md)).

### B2. Platform actions (BP-061)

1. **Invoke / catalog** → Client `GET/POST /client/v1/actions/{apiName}` only — [platform-actions-build-plan.md](./platform-actions-build-plan.md) / [ADR-029](../adr/029-platform-actions.md).
2. Domain logic in `internal/actions` + DataEngine; handlers thin.
3. Do **not** add a per-verb route (`/convertLead`) or a Metadata definition for product verbs.
4. Guest `invokeAction` is ADR-014 SDK, not a second HTTP client from Deno.

### C. Cross-family consistency

1. Ownership: Metadata writes tag `ownership=custom` for customer artifacts; managed stays seed/migrate.
2. Deploy must reject managed package internals (`internal/deploy/types.go` allowlists).
3. When both HTTP and domain packages change, cite this playbook **and** the domain playbook (data / deploy / authz).

### D. Integration tests

1. Prefer [`internal/testutil`](../../internal/testutil) (`RequireDatabase`, `BootstrapCore`, `NewTestServer`, `AuthRequest`) for new DB-gated HTTP contracts.
2. Keep smoke coverage in `internal/httpapi/integration_test.go` / `principals_integration_test.go`.
3. Domain unit tests stay next to `internal/<pkg>`.

## Explicit non-goals (until docs say otherwise)

- GraphQL or a fourth commercial family without a new ADR
- Promoting managed `core` / platform internals via Deploy
- Multi-tenant control-plane routes on `cmd/api`
- Reintroducing Node/OpenAPI stubs (ADR-005). A later generated OpenAPI artifact belongs in the **CMS aggregator**, not those stubs.
- Using `PRODUCT_VERSION` as the client wire pin (use API revision per [ADR-025](../adr/025-api-revision-versioning.md) / [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md))

## Checklist before merging an HTTP / family PR

- [ ] Family ownership matches ADR-004
- [ ] Handler lives in the correct route file
- [ ] Scope middleware matches the family
- [ ] Domain logic not dumped into `httpapi` beyond glue
- [ ] Breaking Client wire changes bump / document `apiRevision` (ADR-025) when behavior diverges for pinned clients
- [ ] Tests cover the new/changed route
- [ ] Module map updated if a new key file appears
- [ ] Customer-facing route changes **may** update the matching [`docs/api/{client,metadata,deploy,ops,auth}.md`](../api/) page in the same PR (not by pasting the [overview](../api-families.md) five times); the public host is a separate CMS ([agent-public-docs.md](./agent-public-docs.md)). Do not add `tools/one-docs`. Do not publish `GET /describe` as a docs catalog ([objects.md](../objects.md)).
