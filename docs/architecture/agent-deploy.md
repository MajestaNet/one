# Agent playbook: Deploy / Ops

For agents changing promotions, peers, bundles, multi-env trust, Ops image rolls, or install packaging. Follow this before writing code.

## Where to look

| Concern | Path |
|---|---|
| Multi-env identity | [`docs/multi-env-deploy.md`](../multi-env-deploy.md) — **opt-in**; default install is one Prod |
| Install → IDE connect | [`install-ide-connect-build-plan.md`](./install-ide-connect-build-plan.md) — single-Prod URL + first admin |
| Ops / upgrades | [`docs/ops.md`](../ops.md), [`docs/product-upgrades.md`](../product-upgrades.md), [`docs/adr/007-platform-ops-upgrades.md`](../adr/007-platform-ops-upgrades.md) |
| Deploy engine | `internal/deploy/service.go`, `apply.go`, `validate.go`, `trust.go`, `types.go`, `tests.go`, `compare.go` |
| Deploy HTTP | `internal/httpapi/deploy_routes.go` |
| Ops engine | `internal/ops/engine.go`, `ecs.go`, `roller.go` |
| Ops HTTP | `internal/httpapi/ops_routes.go` |
| Packaging | `deploy/Dockerfile`, `deploy/docker-compose.yml`, `deploy/helm/`, `deploy/digitalocean/`; community AWS under `sdk/aws/deploy/` (not GA) |
| Product boundary | [`docs/monorepo.md`](../monorepo.md), `scripts/assert-product-boundary.sh` |
| Distribution plan | [`digitalocean-distribution-build-plan.md`](./digitalocean-distribution-build-plan.md) (strategy + role matrix) · [`do-app-platform-deploy-api-build-plan.md`](./do-app-platform-deploy-api-build-plan.md) (**active**) |
| Deploy cloud contract | [`deploy-cloud-capability-contract.md`](./deploy-cloud-capability-contract.md) — host-free `/deploy/v1/cloud/*`; DO product adapter; AWS managed profile = community |
| Agnostic API uplift | [`deploy-cloud-agnostic-build-plan.md`](./deploy-cloud-agnostic-build-plan.md) — `CloudHost` port + aliases |
| Backlog | [`BP-010`](../../backlog/BP-010-three-api-families.md), [`BP-002`](../../backlog/BP-002-dedicated-install-fleet-ops.md), [`BP-011`](../../backlog/BP-011-container-marketplace-fargate.md), [`BP-029`](../../backlog/BP-029-app-platform-install.md), [`BP-030`](../../backlog/BP-030-deploy-api-digitalocean-apps.md), [`BP-031`](../../backlog/BP-031-customer-repo-init-sync.md), [`BP-032`](../../backlog/BP-032-customer-dx-validate-deploy.md), [`BP-028`](../../backlog/BP-028-digitalocean-marketplace-listing.md) (listing deferred), [`BP-027`](../adr/030-install-agent-runtime.md) (IDE DO Govern — frozen), [`BP-041`](../../backlog/BP-041-record-external-id-upsert-bulk.md) (business **data packs** — Client apply, not Deploy promote; [plan](./external-id-upsert-bulk-build-plan.md)) |
| Community AWS | [`sdk/aws/`](../../sdk/aws/README.md) — optional Path B; managed PaaS profile [docs](../../sdk/aws/docs/managed-paas-profile.md); **not** managed subscription GA |

## What ships today

```text
CUSTOMER_ID   — shared across a customer’s installs
INSTALL_ID  — unique per environment
INSTALL_ROLE — free-form label (test, staging, prod, …)
DEPLOY_PEER_MODE — customer (default) | allowlist
Deploy promotes customer-owned metadata/tests only (never managed seed internals)
Ops rolls product images on this install (not customer promote)
```

## What to do (change types)

### A. Promote / bundle / validate

1. Keep managed package names rejected (`packages.IsManagedPackageName` — registry + core/legacy aliases).
2. Validate customer ownership before apply; never treat the Majesta One monorepo as the promotion unit.
3. Prefer **repo → org** DX: `CompareArtifacts` + `POST /deploy/v1/packages/validate-local`, then promote on the connected install ([BP-032](../../backlog/BP-032-customer-dx-validate-deploy.md)). Peer push and inbound artifact promote are removed.
4. Extend `validate.go` / `compare.go` / `apply.go` with tests in `compare_test.go` / `service_test.go`.

### B. Peer registry / multi-env

1. `POST/GET /peers` remain for IDE env switcher topology only — not a promote channel.
2. Install→install peer push and inbound artifact promote are **removed**; multi-env is repo→org ([multi-env-deploy.md](../multi-env-deploy.md)).
3. `DEPLOY_SHARE_SECRET` / `DEPLOY_PEER_MODE` are optional legacy env knobs (no longer required in production for promote trust).

### C. Customer tests in Deploy

1. Customer test runners live with Deploy (`tests.go`); product CI may invoke them against installs — not by baking customer fixtures into `internal/seed`.

### D. Ops image roll

1. Ops confirms / rolls / gates / rollbacks **product** versions on this install (ADR-007).
2. Keep ECS/Marketplace specifics under `internal/ops` + community [`sdk/aws`](../../sdk/aws/README.md) — do not embed a multi-tenant fleet control plane inside `cmd/api`.

### E. Packaging / Dockerfile

1. Product image COPY allowlist: `go.mod`/`go.sum`, `cmd/`, `internal/`, `migrations/` only.
2. Run `scripts/assert-product-boundary.sh` after Dockerfile or `.dockerignore` changes. After image builds, run `scripts/assert-image-contents.sh <image>`.
3. Docs, backlog, `.cursor/`, `sdk/`, and `*.md` must remain excluded from the image context. DO Marketplace listing assets live under `deploy/digitalocean/`; AWS listing assets under `sdk/aws/deploy/marketplace/` are community/historical only.

### F. DigitalOcean App Platform cloud (active plan)

1. Packaging first: App Spec under `deploy/digitalocean/` ([BP-029](../../backlog/BP-029-app-platform-install.md)).
2. Day-2 manage/scale/provision peers via Deploy **host-free** `/deploy/v1/cloud/*` ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md), [deploy-cloud-agnostic-build-plan.md](./deploy-cloud-agnostic-build-plan.md)) — install-local DO token; DO paths remain compatibility aliases.
3. Product digest rolls stay Ops ([ADR-007](../adr/007-platform-ops-upgrades.md)); shared DO client may serve both.
4. Follow [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md).
5. Non-DO hosts: community adapters only (`sdk/aws` managed PaaS profile) — **not** a second Path A until GA’d.

## Explicit non-goals (until docs say otherwise)

- Promoting managed `core` via Deploy
- Cap on number of test installs per customer
- AMI/EC2 as a distribution path
- Shipping `docs/` or `backlog/` in release images
- Control IDE DO Govern UI ([BP-027](../adr/030-install-agent-runtime.md) frozen) — do not block App Spec / Deploy DO cloud waves on IDE chrome
- Kubernetes Marketplace / deep K8s enhancements while Wave A/B (App Platform + Deploy API) is active
- Equating ECS Fargate with App Platform; promoting AWS ECS community profile to product Path A without an explicit decision

## Checklist before merging a Deploy / Ops PR

- [ ] Read multi-env-deploy + this playbook
- [ ] Managed artifacts still rejected on promote
- [ ] Trust mode behavior covered by tests
- [ ] Dockerfile allowlist unchanged (or boundary script updated intentionally)
- [ ] Path A/B or env-var changes **may** update `/install` (`docs/self-host.md`) in the same PR; otherwise the merge-event docs agent covers it ([agent-public-docs.md](./agent-public-docs.md))
- [ ] BP-002 / BP-011 / BP-028 / BP-029 / BP-030 updated if fleet/Marketplace / App Platform / Deploy DO cloud risk changed
