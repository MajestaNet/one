# BP-030: Deploy API — DigitalOcean App Platform manage / scale / provision

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `internal/httpapi/deploy_cloud_routes.go`, `internal/deploy/cloud.go`, `internal/deploy/cloud_host.go`, `internal/digitalocean/`, `migrations/0030_digitalocean_cloud.sql`, `migrations/0036_deploy_cloud_host.sql`, `docs/api-families.md`
- **Plan:** [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md) Wave B · [deploy-cloud-agnostic-build-plan.md](../docs/architecture/deploy-cloud-agnostic-build-plan.md) (host-free API uplift)
- **Remainder (agentic):** [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md)
- **Contract:** [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md) (host-free `/deploy/v1/cloud/*`; DO is the product adapter)
- **Depends on:** App Spec packaging ([BP-029](./BP-029-app-platform-install.md)); existing Deploy peers/trust ([BP-010](./BP-010-three-api-families.md))
- **Related:** [BP-028](./BP-028-digitalocean-marketplace-listing.md) (Marketplace deferred) · [BP-027](../docs/adr/030-install-agent-runtime.md) (IDE UI — frozen) · [ADR-007](../docs/adr/007-platform-ops-upgrades.md) (product rolls stay Ops)

## Problem

App Platform is the default Majesta One install path, but day-2 ops (scale app instances, resize Managed Postgres, spin up a peer **dev/test** App) require the DigitalOcean console. Without Deploy API endpoints, operators leave Majesta One, and the IDE has no install-local API to call.

## Why it matters

Environment lifecycle belongs with **Deploy** on the customer’s install: same `CUSTOMER_ID`, peer registration, and admin-gated writes. Install-local `DIGITALOCEAN_API_TOKEN` keeps ADR-001 (no vendor multi-tenant fleet plane) while enabling 1-Click-era day-2 management. DO is the **first** implementation of the host-agnostic [Deploy cloud capability contract](../docs/architecture/deploy-cloud-capability-contract.md); other clouds stay community adapters until GA’d.

## What shipped (Wave B)

1. Shared Go DigitalOcean Apps + Databases client (`internal/digitalocean`) with HTTP mocks
2. Deploy routes under `/deploy/v1/cloud/digitalocean/*`:
   - `GET /status`, `PUT /binding`, `GET /app`
   - `PATCH /app/scale`, `PATCH /database/resize`
   - `POST /environments`, `GET /environments`
   - `POST /app/redeploy` (temporary operator helper; prefer Ops long-term)
3. AuthZ: scope `deploy`; mutating cloud routes require `+admin`
4. `GET /deploy/v1/environment` → `capabilities.digitaloceanCloud`
5. Migration `0030_digitalocean_cloud` (binding + provision audit)
6. Docs: [api-families.md](../docs/api-families.md), [self-host.md](../docs/self-host.md); config via `DIGITALOCEAN_API_TOKEN` / `_APP_ID` / `_DATABASE_ID`
7. Provision validation: unique peer `installId`; required `apiKeys` + `authJwtSigningKey`
8. Manual live-smoke checklist: [deploy/digitalocean/LIVE_SMOKE.md](../deploy/digitalocean/LIVE_SMOKE.md)
9. Host-agnostic verb contract documented: [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md)
10. Host-free primary routes `/deploy/v1/cloud/*` + DO compatibility aliases; `CloudHost` port; host-keyed `deploy_cloud_*` tables (0036); `cloudHost` / `capabilities.cloud` on environment; IDE Govern client on host-free paths ([deploy-cloud-agnostic-build-plan.md](../docs/architecture/deploy-cloud-agnostic-build-plan.md))
11. Community AWS CloudHost skeleton under [`sdk/aws/cloudhost`](../sdk/aws/cloudhost/) (not in product binary)

## Remaining

Agent-implementable remainder (Ops App Platform roller — not community AWS Path A): [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md).

- Live DO smoke against a real team token (operator-run checklist above)
- Ops App Platform roller for product upgrades (ADR-007) — migrate redeploy helper
- IDE Govern remainders are frozen ([BP-027](../docs/adr/030-install-agent-runtime.md); OAuth / live E2E)
- Fill in AWS `sdk/aws/cloudhost` ECS/RDS API calls (community; not Path A)

## Explicit non-goals (this item)

- Control IDE UI or DO OAuth in the IDE ([BP-027](../docs/adr/030-install-agent-runtime.md))
- Marketplace Vendor Portal publish ([BP-028](./BP-028-digitalocean-marketplace-listing.md))
- Helm/DOKS provision or node-pool scale APIs (community `sdk/` / Path B only)
- Moving **product** upgrade confirm/roll off `/ops/v1` as the long-term surface (ADR-007)
- AWS/GCP/Azure cloud drivers in the product binary (see [`sdk/`](../sdk/README.md); AWS managed PaaS profile is community — [managed-paas-profile.md](../sdk/aws/docs/managed-paas-profile.md))
- Embedding DO tokens in product images or a One-hosted fleet credential store
- A second product Path A on AWS

## Related

- [deploy-cloud-agnostic-build-plan.md](../docs/architecture/deploy-cloud-agnostic-build-plan.md)
- [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md)
- [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md)
- [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md)
- [BP-029](./BP-029-app-platform-install.md) · [BP-027](../docs/adr/030-install-agent-runtime.md)
