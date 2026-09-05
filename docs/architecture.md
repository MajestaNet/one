# Majesta One Architecture

Majesta One is a dedicated-install, metadata-driven enterprise platform. It is **not a CRM**. Each customer runs their own instance (one database, one API). **Status: alpha `0.1.0`** — contracts can still change in breaking ways.

There are **exactly two** product install paths ([self-host.md](./self-host.md)):

- **Path A** — DigitalOcean App Platform (first targeted managed path)
- **Path B** — Self-install from image (Compose + Helm)

Other cloud providers are expected later through community SDKs under [`sdk/`](../sdk/README.md). Those helpers are **not** a third install product and **not** product GA. Isolation is a deployment property—there is no shared multi-tenant application layer. Marketplace publish is deferred ([BP-028](../backlog/BP-028-digitalocean-marketplace-listing.md)). There is **no** managed subscription GA channel.

## Principles

1. **Dedicated install (one customer per database)** — one database, one API control plane per install.
2. **Metadata is the source of truth** — objects, fields, relationships, validation, automation, and permissions are metadata.
3. **Hybrid storage** — kernel/system tables use real Postgres DDL; business/custom objects use a flexible JSONB row store so field changes do not require production migrations.
4. **API is the product** — three versioned families (Client, Metadata, Deployment). The install is the **agent runtime** ([ADR-030](./adr/030-install-agent-runtime.md)): MCP, harnesses, skills, hosted loop. Control IDE (`tools/control-ide`) is an optional frozen JWT client, not part of the product image. Flat `/v1` is a deprecated compatibility alias during transition.
5. **Agents are platform citizens** — they call the **Client** API (and Metadata when Role-granted) under the same AuthZ model as users/services (playbook *definitions* are Metadata). Builder coding agents use MCP + CLI; they are not hosted inside Control IDE. See [ADR-006](./adr/006-jwt-auth.md) · [ADR-010](./adr/010-customer-agentic-platform.md).
6. **Product ≠ customer implementation** — Majesta One ships one product codebase; each install holds that customer's metadata and tests. Deployment API promotes customer-owned artifacts between **any** of that customer's environments (unlimited test/staging/prod peers under one `CUSTOMER_ID`), not vendor source.

## Runtime components

| Component | Role |
|---|---|
| `cmd/api` | Platform API (Go): Client + Metadata + Deploy (+ `/auth/v1`) |
| `cmd/worker` | Async jobs (Go) |
| `cmd/migrate` | Kernel SQL migrate CLI |
| `internal/metadata` | Object/field describe, writes, cache epoch |
| `internal/dataengine` | Flexible record CRUD + SQL-native query |
| `internal/deploy` | Bundles, promotions, peers, customer tests, package pack/export |
| `internal/customerrepo` | `one/v1` pack/unpack |
| `internal/worker` | Job claim + outbox/webhook delivery |
| `internal/authz` | API keys (`+admin`), transitional OIDC, object CRUD authz; Majesta One JWT + Roles (ADR-006 / ADR-009); opaque refresh tokens ([BP-063](../backlog/BP-063-refresh-token-sessions.md)) |
| `internal/seed` | Bootstrap admin + managed `core` package migrate |
| `migrations/` | Kernel SQL + journal |
| `deploy/digitalocean/` | Path A — App Platform Spec |
| `deploy/helm/`, Compose | Path B — self-install packaging |
| `sdk/` | Community SDKs (other clouds later; not product GA) |

## Storage model

- **System tables** (migrated): users, roles, permission sets, `user_roles`, `role_api_scopes`, `user_permission_sets`, `principal_credentials`, `identity_links`, `refresh_tokens`, metadata_*, outbox, jobs, audit_log, automations, agent_runs, webhooks.
- **Flexible store**: `records` with `data JSONB` plus system columns (`object_api_name`, optional `owner_id`, `created_by_id`, `last_modified_by_id`, timestamps). Custom fields are metadata rows only—no DDL. Deletes are hard deletes (no `deleted_at`).
- **Managed core objects**: Account and Contact (package `core` — [ADR-020](./adr/020-cdm-managed-packages.md)). See [data-model.md](./data-model.md) and [architecture/](./architecture/).
- **AuthZ packaging**: Roles grant API scopes only; permission sets assign to users for object/field access ([ADR-009](./adr/009-record-audit-authz-packaging.md)).

## Migration philosophy

Ship SQL migrations under `migrations/` only when the **kernel** changes (or one-time product data cleanups such as dropping retired managed objects). Adding a customer field like `Account.Region__c` is a Metadata API write (+ optional projection job), not an app migration.

## API families (commercial surfaces)

| Family | Path | Role |
|---|---|---|
| Client | `/client/v1` | Record CRUD, query, bulk/composite, agent runs, event reads |
| Metadata | `/metadata/v1` | Objects/fields/rules/automations/permissions on **this** install |
| Deployment | `/deploy/v1` | Bundles, customer test runs, promote customer-owned changes between any same-customer installs |
| Ops | `/ops/v1` | Product image upgrades on this install (confirm / roll / test gate / rollback); not customer promote |

See [API families](./api-families.md), family pages under [docs/api/](./api/), [ADR-004](./adr/004-three-api-families.md), [ADR-007](./adr/007-platform-ops-upgrades.md), and [ADR-025](./adr/025-api-revision-versioning.md) (client-pinnable API revision inside family majors).

## Community cloud SDKs (later)

The first targeted managed path is DigitalOcean. Other providers should arrive later as community SDKs under [`sdk/`](../sdk/README.md). Those trees are **not** a managed subscription channel and **not** the preferred install path. See [ADR-001](./adr/001-dedicated-install.md).

## Further reading

- [Architecture docs index (agents start here)](./architecture/README.md)
- [Glossary](./glossary.md) — customer, install, org, custom vs managed
- [Module map](./architecture/module-map.md) / [Agent routing](./architecture/agent-routing.md)
- [Data architecture agent playbook](./architecture/agent-data-architecture.md)
- [ADR index](./adr/README.md)
- [Confirmed tech stack](./tech-stack.md)
- [Core data model](./data-model.md)
- [Sales & Service data model](./architecture/sales-service-data-model.md)
- [ADR-005: Go runtime](./adr/005-go-runtime.md)
- [ADR-006: Majesta One JWT auth](./adr/006-jwt-auth.md) (refresh grant: [refresh-token-session-build-plan.md](./architecture/refresh-token-session-build-plan.md))
- [ADR-008: Core data model](./adr/008-core-data-model.md)
- [ADR-009: Record audit + AuthZ packaging](./adr/009-record-audit-authz-packaging.md)
- [ADR-010: Customer agentic platform](./adr/010-customer-agentic-platform.md)
- [ADR-011: Sales & Service managed modules](./adr/011-sales-service-managed-modules.md)
- [ADR-031: Billing managed module](./adr/031-billing-managed-module.md)
- [ADR-014: Customer code automations](./adr/014-customer-code-automations.md)
- [ADR-029: Platform actions](./adr/029-platform-actions.md)
- [ADR-018: CanvasDocument](./adr/018-crm-canvas-document.md) (historical)
- [ADR-019: Client Experience + OSS kits](./adr/019-client-experience-oss-kits.md)
- [ADR-021: Run mode + ToolSpec](./adr/021-run-mode-toolspec.md)
- [ADR-023: Run personal graph](./adr/023-run-personal-graph.md)
- [ADR-024: Run graph interactions](./adr/024-run-graph-interactions.md)
- [Run mode build plan](./adr/021-run-mode-toolspec.md)
- [Run personal graph build plan](./adr/023-run-personal-graph.md)
- [Run graph interactions build plan](./adr/024-run-graph-interactions.md)
- [Client Experience build plan](./architecture/client-experience-build-plan.md)
- [Customer automations build plan](./architecture/customer-automations-build.md)
- [Platform actions build plan](./architecture/platform-actions-build-plan.md)
- [Billing module build plan](./architecture/billing-module-build-plan.md)
- [Customer agents](./customer-agents.md)
- [Customer connect paths](./customer-connect.md)
- [Builder connect (MCP + CLI)](./builder-connect.md)
- [Install as agent runtime (ADR-030)](./adr/030-install-agent-runtime.md) · [build plan](./architecture/agent-runtime-build-plan.md) · [hosted tool loop](./architecture/hosted-agent-tool-loop-build-plan.md)
- [Self-host (Path A / Path B)](./self-host.md)
- [Community cloud SDKs](../sdk/README.md)
- [Security](./security.md)
- [API families](./api-families.md) · [family reference](./api/)
- [Monorepo structure](./monorepo.md)
- [Release CI/CD](./release-cicd.md)
- [Public docs (`one.majesta.net`)](./architecture/public-docs-site.md) — separate CMS aggregator ([BP-067](../backlog/BP-067-public-docs-site.md))
- [Control IDE build plan](./control-ide-build.md)
- [Customer customizations](./customer-customizations.md)
- [CI customer tests (Phase D)](./ci-customer-tests.md)
- [Multi-env deploy (Phase E)](./multi-env-deploy.md)
- [Product upgrades](./product-upgrades.md)
- [Backlog / foreseeable problems](../backlog/README.md)
- [ADR-001: Dedicated install deploy](./adr/001-dedicated-install.md)
- [ADR-002: Hybrid metadata storage](./adr/002-hybrid-metadata-storage.md)
- [ADR-003: SQL query engine](./adr/003-sql-query-engine.md)
- [ADR-004: Three API families](./adr/004-three-api-families.md)
- [ADR-030: Install as agent runtime](./adr/030-install-agent-runtime.md)
