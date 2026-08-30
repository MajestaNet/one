# Architecture decision records

Index of accepted ADRs. Open the ADR that locks the decision for your change; do not re-litigate accepted decisions without a new ADR.

| ADR | Title | Open when… |
|---|---|---|
| [001](./001-dedicated-install.md) | Dedicated install deploy | Tenancy, isolation, managed vs Marketplace account topology |
| [002](./002-hybrid-metadata-storage.md) | Hybrid metadata storage | Kernel DDL vs `records.data` JSONB; custom fields |
| [003](./003-sql-query-engine.md) | SQL query engine | Query planner, indexes, projections, list/query performance |
| [004](./004-three-api-families.md) | Three API families | Client / Metadata / Deploy ownership; where an endpoint belongs |
| [005](./005-go-runtime.md) | Go runtime | Language/runtime; rejecting Node/TS/Python sidecars |
| [006](./006-jwt-auth.md) | Majesta One JWT auth | Token Service, principals, IdP exchange, opaque refresh tokens |
| [015](./015-idp-agnostic-social-login.md) | IdP-agnostic social login | Google/Apple broker default; Cognito optional; other IdPs as exchange adapters |
| [007](./007-platform-ops-upgrades.md) | Platform Ops upgrades | Product image rolls vs customer promote |
| [008](./008-core-data-model.md) | Core data model | Managed `core` objects/fields contract |
| [009](./009-record-audit-authz-packaging.md) | Record audit + AuthZ packaging | OwnerId/audit fields; Roles vs permission sets |
| [010](./010-customer-agentic-platform.md) | Customer agentic platform | AgentSpec, MCP-over-API, starter clone-on-enable, plane separation |
| [011](./011-sales-service-managed-modules.md) | Sales & Service managed modules | Optional catalog/sales/service/crm_bridge; thin catalog vs CPQ; Quote-centric sales |
| [012](./012-customer-repo-and-control-ide.md) | Customer repo + Control IDE | CodeCommit per `CUSTOMER_ID`, `one/v1`, pack/export APIs, `tools/control-ide` |
| [013](./013-high-volume-flexible-storage.md) | High-volume flexible storage | Shared `records` scale, `storage_mode=high_volume`, LIST/RANGE partitioning |
| [014](./014-customer-code-automations.md) | Customer code automations | Deno guest TS, PS automation grants, sync rollback, zero third-party imports |
| [016](./016-record-sharing.md) | Record sharing | OWD, data roles, criteria rules, materialized grants |
| [017](./017-canonical-field-types.md) | Canonical field types | Metadata allowlist, DataEngine casts, compound/autonumber/richtext |
| [018](./018-crm-canvas-document.md) | CRM CanvasDocument | Allowlisted declarative nodes; product IDE surface relocated to Run (ADR-021) |
| [019](./019-client-experience-oss-kits.md) | Client Experience + OSS kits | Customer browser apps via `sdk/client/`; Connected Apps Client-only defaults; not in-IDE Tools |
| [020](./020-cdm-managed-packages.md) | Managed domain packages | Curated CRM + industry packs; Account/Contact enrichment; Lead only in `lead_marketing` |
| [021](./021-run-mode-toolspec.md) | Run mode + ToolSpec | Fifth IDE mode; metadata-driven Tools; evolves CanvasSpec; no customer React |
| [022](./022-agent-conversations.md) | Agent IDE conversations | Principal-scoped chat threads + preferences; not CRM Message |
| [023](./023-run-personal-graph.md) | Run personal graph | Principal-scoped reference-only Run home graph; hydrate-on-read; no GraphQL |
| [024](./024-run-graph-interactions.md) | Run graph interactions | Pin / Wire / Apply CRM-replacement contract on ADR-023 graph; proposal staging; Operate→Run handoff |
| [025](./025-api-revision-versioning.md) | API revision versioning | Client-pinnable `apiRevision` vs `PRODUCT_VERSION` vs family `/v1`; Connect/SDK wire compat ([BP-025](../../backlog/BP-025-ide-api-version-compatibility.md)) |
| [026](./026-kernel-user-metadata.md) | Kernel User metadata object | `storage_mode=kernel`, `users.data`, customer User fields; not a flexible record object ([BP-058](../../backlog/BP-058-user-identity-extension.md)) |
| [027](./027-run-graph-collection-nodes.md) | Run graph collection nodes | Object/list-view replacement on the personal graph; click → list in focus; search stays chrome (remainders frozen — [ADR-030](./030-install-agent-runtime.md)) |
| [028](./028-operate-graph-surface.md) | Operate graph surface | Glance cards, work sheets, drop-to-mount Tools, attention hygiene, graph command bar ([BP-060](../../backlog/BP-060-operate-graph-surface.md)) |
| [029](./029-platform-actions.md) | Platform actions | Package-gated Client verbs (`/client/v1/actions`); guest `invokeAction`; Lead convert ([BP-061](../../backlog/BP-061-platform-actions.md)) |
| [030](./030-install-agent-runtime.md) | Install as agent runtime | Go install is the agent runtime; Control IDE optional (refactor for install cleanup — [BP-065](../../backlog/BP-065-ide-backend-coupling.md)); MCP + `one` are builders ([BP-064](../../backlog/BP-064-install-agent-runtime.md)) |
| [031](./031-billing-managed-module.md) | Billing managed module | Optional `billing` Order/OrderLine; `quote.accept` ([BP-044](../../backlog/BP-044-billing-module-order-from-quote.md)) |
| [032](./032-retire-messages-polymorphic-lookup.md) | Retire Messages + polymorphic lookup | Drop `messages` module and `polymorphic_lookup`; agent audit stays on conversations / `agent_runs` |

Agent entry: [architecture README](../architecture/README.md) · [module map](../architecture/module-map.md) · [agent routing](../architecture/agent-routing.md).
