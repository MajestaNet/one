# ADR-007: Platform Ops upgrades (product image rolls)

## Status

Accepted — partially implemented (docs, ECS circuit breaker + SSM Automation, `/ops/v1` upgrade API, product smoke suite).

## Context

Majesta One product upgrades (kernel, API/worker binaries, managed packages) are **ECS image / task-definition rolls**, not Deploy API promotions. Deploy (`/deploy/v1`) moves **customer-owned** metadata and tests between same-`CUSTOMER_ID` installs ([ADR-004](./004-three-api-families.md)).

Marketplace subscribers need a low-friction confirm → roll → test → finish/rollback path inside **their** AWS account. A vendor multi-tenant remote-control plane would violate [ADR-001](./001-dedicated-install.md) and recreate the fleet coupling warned about in BP-002.

The reference stack already runs API tasks across **two AZs** with rolling deploy settings. Operators sometimes assume “upgrade one AZ, then the other.” That is the wrong canary unit: ECS spreads **tasks** behind the ALB; all tasks share **one RDS**. Kernel DDL and managed-package migrate run on boot against that shared database.

## Decision

### 1. Upgrade unit = ECS rolling / canary (not AZ)

- Keep `api` desired count ≥ 2 and rolling `minimumHealthyPercent` / `maximumPercent` so replace is zero-downtime when health checks pass.
- Enable the **ECS deployment circuit breaker with rollback**.
- Do **not** pin or stage by Availability Zone as the primary canary mechanism.
- Optional later: CodeDeploy traffic-shifting canary if a longer mixed-version bake window is required.

### 2. Shared-DB migration rules

- Product migrations must be **forward-compatible** during a roll (old tasks tolerate new schema; new tasks do not require irreversible breaks mid-deploy).
- Canary validates **new binary + shared DB**, not an isolated AZ copy of state.
- Rollback = previous task definition revision; RDS snapshot restore only if a kernel migration is incompatible (rare).

### 3. Two orchestration surfaces (install-local)

| Surface | Audience | Owns |
|---|---|---|
| SSM Automation (shipped with Terraform) | AWS admin in the subscriber account | Confirm version → register task defs → `UpdateService` → wait stable → smoke/tests → rollback on failure |
| `/ops/v1/upgrades` | Majesta One principal with scope `ops` (+ admin for confirm/rollback) | Same lifecycle as an API: record run, drive ECS when configured, gate on product + customer tests |

Neither surface is a One-hosted multi-account control plane. Vendor side publishes images and a version manifest; optional EventBridge/SNS “update available” may notify the customer account.

### 4. Do not overload Deploy

- `/deploy/v1` remains customer bundles, peer promote, and customer test definitions/runs.
- Product image rolls **never** go through `/deploy/v1/promotions`.
- Post-roll gates **reuse** `POST /deploy/v1/tests/runs` (product `PlatformSmoke` suite + optional customer `PostUpgradeSmoke`).

### 5. New API family scope: `ops`

- Scope `ops` gates `/ops/v1/*`.
- Admin privilege does **not** bypass a missing `ops` scope (same rule as other families).
- Bare API keys that grant all scopes include `ops`.

## Consequences

- Upgrade friction drops for Marketplace admins (Console Automation or Ops API) without a SaaS deployer.
- Docs and runbooks must keep product upgrade vs customer promote clearly separated.
- Fleet version reporting (`PRODUCT_VERSION` + OTEL) remains follow-up; upgrades stay customer-initiated per install.

## Related

- [docs/product-upgrades.md](../product-upgrades.md)
- [BP-002](../../backlog/BP-002-dedicated-install-fleet-ops.md), [BP-011](../../backlog/BP-011-container-marketplace-fargate.md)
- [sdk/aws/docs/aws-fargate.md](../../sdk/aws/docs/aws-fargate.md), [sdk/aws/docs/marketplace.md](../../sdk/aws/docs/marketplace.md)
