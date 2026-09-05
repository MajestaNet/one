# ADR-004: Three API families (Client, Metadata, Deployment)

## Status

Accepted — **implemented** (Phases A–E; see [api-families.md](../api-families.md) and [`docs/api/`](../api/), BP-010 mitigated). Product image upgrades are a separate Ops family ([ADR-007](./007-platform-ops-upgrades.md), Phase F).

## Context

Majesta One is a commercial product: the **vendor ships one product codebase** (ECS Fargate / Compose / optional Helm). Each B2B customer installs **their own dedicated install instance** on AWS (or equivalent). That install is not a fork of Majesta One source—it is a runtime of the product plus a **customer implementation** (metadata, automations, tests, optional plugins) that lives in that customer's environments.

Today the Platform API is a flat `/v1` surface mixing record CRUD, metadata mutation, and ops concerns. We need clear product boundaries for:

1. Applications and integrations that **do work** on business data.
2. Customer developers who **shape** the model in a given install (test or prod).
3. Release engineers who **promote** customer-owned changes between **any of that customer’s environments** (N test/staging/prod installs under one commercial customer—not a fixed single test→prod pair) without shipping Majesta One kernel code or other customers' artifacts.

## Decision

Expose **three versioned API families** on each install, with scoped credentials:

| Family | Base path | Audience | Mutates |
|---|---|---|---|
| **Client API** | `/client/v1` | Apps, integrations, agents | Business records, queries, bulk/composite, events consume, agent *runs* |
| **Metadata API** | `/metadata/v1` | Customer developers / admins on **this** install | Objects, fields, validation, automations, permission sets, webhooks config |
| **Deployment API** | `/deploy/v1` | CI/CD and release tooling for **this customer's** environments (unlimited installs per customer) | Bundles, test runs, validate/promote of **customer-owned** artifacts between same-customer peers |

### Product vs customer implementation (non-negotiable)

```
┌─────────────────────────────────────────────────────────┐
│  Majesta One PRODUCT codebase (vendor)                      │
│  apps/api, packages/*, deploy/helm, kernel migrations   │
│  Upgraded via Marketplace containers / ECS — NOT Deploy API │
└─────────────────────────────────────────────────────────┘
                         │ installed into
                         ▼
┌─────────────────────────────────────────────────────────┐
│  Customer AWS install A (Test)     Customer install A (Prod)
│  Same product binary version       Same product binary
│  DB + CUSTOMER IMPLEMENTATION ──Deploy API──► DB + impl  │
└─────────────────────────────────────────────────────────┘
```

- **One product repo** for Majesta One. Customers do **not** get a private fork of the platform as the unit of customization.
- **Per-install customer implementation** is the customization unit: metadata packages owned by the customer, customer tests, and (later) approved plugins.
- **Deploy API never ships** Majesta One kernel source, Drizzle migrations, or managed package internals as customer payloads—only customer-owned manifests and artifacts.
- Cross-customer isolation remains **infrastructure** (ADR-001). There is still no SaaS `tenant_id` isolation column in the app; “customer” here means **customer install / environment**.

### Ownership tags on metadata

Every metadata artifact is classified:

| Ownership | Examples | Deploy API |
|---|---|---|
| `managed` | Kernel + seed packages `platform`, `core` | Excluded from promote; upgraded with product version |
| `custom` | Custom objects/fields, customer automations, customer permission sets, customer tests | Included in bundles |

### Auth scopes

Principals (Majesta One JWT target per [ADR-006](./006-jwt-auth.md); transitional API keys / OIDC) carry one or more scopes: `client`, `metadata`, `deploy`. Routes reject mismatched scopes. Prefer least privilege (integrations get `client` only). Roles assign scopes in the long-run model (BP-013).

### Compatibility

Keep `/v1/*` as **deprecated aliases** for one major release, mapping to Client or Metadata as appropriate. New Deployment endpoints have no flat `/v1` equivalent.

Graduated wire compatibility **inside** family majors uses a client-pinnable **API revision** integer (`One-API-Revision` / optional `/r{N}/` under the family path)—not a second family major and not `PRODUCT_VERSION`. See [ADR-025](./025-api-revision-versioning.md) and [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md).

## Consequences

- Clearer SDK / OpenAPI surfaces and commercial packaging (Client for ISVs; Metadata for builders; Deploy for DevOps).
- New kernel tables for bundles, promotions, and customer test definitions/runs.
- Requires a **deploy-engine** that diffs/applies customer metadata from packed `one/v1` artifacts on the **connected** install (repo→org). Install→install inbound artifact promote is not part of the model.
- Aligns with BP-007 (package versioning): managed packages version with product; customer packages version in Deployment bundles.
- Flat `/v1` must be migrated carefully so existing tests and demos keep working during the transition.

## Related

- [API families overview](../api-families.md) · [family reference](../api/) · [historical plan](../architecture/api-families-build-plan.md)
- [ADR-001 Dedicated install deploy](./001-dedicated-install.md)
- [ADR-006 Majesta One JWT auth](./006-jwt-auth.md)
- [ADR-025 API revision versioning](./025-api-revision-versioning.md)
- [BP-010 Three API families](../../backlog/BP-010-three-api-families.md)
