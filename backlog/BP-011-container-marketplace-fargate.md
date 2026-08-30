# BP-011: Distribution packaging — OSS images, dual-path install

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `deploy/docker-compose.yml`, `deploy/helm/one/`, `deploy/Dockerfile`, `deploy/digitalocean/`, community `sdk/`
- **Plan:** [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md)
- **Remainder (agentic):** [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md)
- **AuthN:** Follows [ADR-006](../docs/adr/006-jwt-auth.md) / [ADR-015](../docs/adr/015-idp-agnostic-social-login.md) / [BP-013](./BP-013-jwt-unified-principals.md) — Cognito is **not** the product default (optional via [`sdk/aws`](../sdk/aws/README.md))

## Problem

Operators need a clear install path: published OSS images, a **lowest-friction managed path** for non-K8s customers, and a **self-install from image** path — without forcing AWS Marketplace or a vendor managed fleet.

## Why it matters

**Path A:** DigitalOcean **App Platform** + Managed PostgreSQL ([BP-029](./BP-029-app-platform-install.md)). **Path B:** Compose + Helm on any K8s. Marketplace publish deferred ([BP-028](./BP-028-digitalocean-marketplace-listing.md)). IDE DO Govern ([BP-027](../docs/adr/030-install-agent-runtime.md)) frozen. Community cloud helpers live under [`sdk/`](../sdk/README.md) (not product GA).

## Decision (distribution)

| Path | Role |
|---|---|
| **A — App Platform** (`deploy/digitalocean/`) | **Only** product Path A — default lowest-friction customer install |
| **B — Self-install** (`deploy/helm/one`, Compose) | Images on Compose or Helm (DOKS / EKS / AKS / GKE / on-prem) |
| **Community `sdk/`** | Optional Path B extensions (AWS first) — **not** a third install product; AWS managed PaaS profile is community only |
| **Marketplace** | App Platform–first — publish deferred [BP-028](./BP-028-digitalocean-marketplace-listing.md) |
| **Public registry** | GHCR `ghcr.io/majestanet/one-{api,worker}` digests on `v*` |

- **Cancelled:** AWS Marketplace GA; managed subscription fleets; Droplet 1-Click / AMI  
- **IDE DO ops (frozen):** [BP-027](../docs/adr/030-install-agent-runtime.md) — do not add Govern OAuth or live E2E chrome
- **Day-2 cloud verbs:** [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md) — DO product adapter first; do not equate Fargate with App Platform

## What shipped

- Dockerfile, Compose, Helm (+ DOKS/dev overlays), GHCR release digests  
- Apache-2.0 (entire repository, including Control IDE); `SECURITY.md`  
- Self-host dual-path guide; App Spec under `deploy/digitalocean/`  
- Community [`sdk/aws`](../sdk/aws/README.md) (former `deploy/aws` packaging + Cognito/ECS/WAF adapters)
- Role matrix + Deploy cloud capability contract (docs)

## Remaining

Agent-implementable remainder (first `v*` process + CIS/OTEL/social runbooks — not Marketplace publish): [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md).

- [ ] Cut first public `v*` tag exercising GHCR end-to-end  
- [ ] CIS/OTEL/social-broker runbook remainders  
- [ ] **Deferred:** Marketplace publish [BP-028](./BP-028-digitalocean-marketplace-listing.md)  
- [x] BP-027 frozen (DO OAuth helper / live E2E chrome are not a product track)
- [ ] BP-029 live App Spec smoke remainders  
- [ ] Optional: community AWS managed PaaS adapter implementing Deploy cloud verbs ([managed-paas-profile.md](../sdk/aws/docs/managed-paas-profile.md)) — not Path A

## Direction

- **Path A App Platform** = only product managed PaaS path  
- **Path B** = portable images (Compose / Helm) + community power stacks (ECS Fargate)  
- **No** AWS Marketplace GA; **no** managed subscription; **no** second Path A without explicit GA  
- Digests via Compose / App Spec / Helm upgrades — [ADR-007](../docs/adr/007-platform-ops-upgrades.md)
