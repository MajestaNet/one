# BP-002: Install-local upgrades (self-host)

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `deploy/`, `internal/ops/`, ops
- **Remainder (agentic):** [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md)

## Problem

Dedicated install-by-deploy avoids app multi-tenancy but creates an N-install fleet: upgrades, backups, kernel migrations, and secret rotation multiply per customer install.

## Why it matters

OSS self-host success (Compose, Helm on any Kubernetes, optional AWS ECS example) depends on reliable, low-touch **install-local** upgrades—without a vendor-operated multi-tenant control plane or a managed subscription channel.

## What shipped

- Install identity + multi-env Deploy (`CUSTOMER_ID` / `INSTALL_ID` / `INSTALL_ROLE` / `PRODUCT_VERSION`)
- Kernel schema apply-on-boot for safer image rolls
- ECS Fargate **optional** community reference upgrade path: new image tag → new task definition revision → rolling deploy (`sdk/aws/deploy/ecs/`)
- Memory / local Ops roller is the **product default** (Compose / Helm / App Platform)
- **ADR-007** product upgrade model: task-based rolling/canary (not AZ-staged), shared-DB forward-compatible migrations
- ECS **deployment circuit breaker + rollback** on api/worker services (AWS example only)
- Install-local **SSM Automation** `One-ProductUpgrade-*` when on ECS (confirm → roll → health + Deploy tests → rollback)
- **`/ops/v1/upgrades`** (scope `ops`) for API-driven confirm/list/rollback; ECS drive when `ECS_*` env set, otherwise in-memory / local roller
- Managed **`PlatformSmoke`** suite seed + optional customer **`PostUpgradeSmoke`** gate
- Docs: [product-upgrades.md](../docs/product-upgrades.md)

### Historical (cancelled as product goals)

Managed subscription fleet overlay (`sdk/aws/deploy/managed/`, Cognito pool quota alarms, vendor regional cells) and Marketplace↔managed channel promotion fencing were explored under a prior commercial model. **Managed subscription is not a product channel.** Do not extend those remainders; leave artifacts under community `sdk/aws` unless a future ADR reopens them.

## Remaining

Agent-implementable remainder (Compose/Helm playbooks, optional Helm `/ops` roller, backup smoke, rotation, OTEL hooks): [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md).

- Documented **Compose** and **Helm** upgrade playbooks (image pin by digest, roll api+worker, health + optional PostUpgradeSmoke) as the primary self-host path
- Optional non-ECS `/ops/v1` roller for Kubernetes (Helm) parity with the ECS adapter
- Backup smoke tests gated in CI per release
- Secret rotation playbooks with zero-downtime key cutover
- Optional per-operator version inventory across *their* installs (still install-local confirm/roll; no SaaS remote-control plane inside the product binary)
- Observability hooks for upgrade outcomes (`PRODUCT_VERSION` + OTEL — BP-008)

## Direction

- Keep upgrades **install-local** (`/ops/v1` and/or operator-native roll of Compose/Helm/ECS) — do not build a Majesta One multi-tenant deployer into the product API
- **No managed subscription** fleet orchestration in product scope
- Prefer Compose + Helm runbooks; treat ECS SSM Automation as an optional AWS adapter
- Backup/restore runbooks and smoke tests per release
- Observability and version reporting from each install (`PRODUCT_VERSION` + OTEL)
