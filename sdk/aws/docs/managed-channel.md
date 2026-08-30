> **Not product GA.** Community-maintained under [`sdk/aws`](../README.md) — optional Path B extension only. Preferred install: [docs/self-host.md](../../../docs/self-host.md) (Path A App Platform / Path B Compose+Helm).

# Managed subscription channel

Vendor-operated commercial channel parallel to Marketplace self-managed. Architecture stays [ADR-001](../../../docs/adr/001-dedicated-install.md): one isolated install stack per customer environment. Only **who owns the AWS account** changes.

Isolation proof / threat model: [managed-channel-security.md](./managed-channel-security.md).  
Packaging: [sdk/aws/deploy/managed/](../deploy/managed/) (fleet IAM, Cognito quota alarms) + per-install [sdk/aws/deploy/ecs/](../deploy/ecs/).

## Topology

| Aspect | Managed subscription |
|---|---|
| Account | Vendor operates **~one AWS account per offered region**, optionally sharded into **cells** when Cognito/VPC quotas approach limits |
| Isolation | Each customer env = one apply of `sdk/aws/deploy/ecs/` → dedicated VPC, RDS, Secrets Manager secret, Cognito User Pool, ECS api+worker |
| Identity | **One Cognito User Pool per install**; UI app client + service/agent app clients **inside that pool** ([ADR-006](../../../docs/adr/006-jwt-auth.md)) |
| Images | Same OSS `PRODUCT_VERSION` **and image digests** as product releases ([release-cicd.md](../../../docs/release-cicd.md)) |
| Upgrades | Install-local SSM / `/ops/v1` roll ([product-upgrades.md](../../../docs/product-upgrades.md)); vendor **orchestrates** rolls from outside `cmd/api` ([BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md)) |

**Non-goals:** shared Postgres or shared API process across commercial customers; Cognito User Pool shared across customers; embedding a multi-tenant fleet control plane inside the product binary; **auto-rolling managed prod from product `v*` tags or PR merges**.

## Release vs roll (promotion fence)

Having managed reference TF in this monorepo is fine. Wiring “merge/tag → mutate managed prod” in the same pipeline that publishes the distributed product is **not**.

| Step | Owns | Must not |
|---|---|---|
| Product `release.yml` on `vX.Y.Z` | Build/publish images + binaries | Hold managed-prod AWS credentials; start fleet rolls |
| Marketplace publish | Attach **those digests** to the listing / fulfillment | Rebuild a parallel “Marketplace-only” image from `main` |
| Managed roll | Consume the **same digests**; canary → staging → prod with human approval | Float on `:latest`; use a long-lived `managed-prod` product branch |

Full rules and parity gate: [Channel promotion in release-cicd.md](../../../docs/release-cicd.md#channel-promotion-marketplace--managed).

**Branching:** keep product on trunk + semver tags. Advanced structure (when needed) is **gated environments / an ops repo** that promote released digests — not a fork of `cmd/` / `internal/`.

When applying or upgrading an install, pin images by digest (or an immutable tag that maps 1:1 to a digest), and set `PRODUCT_VERSION` to the matching `X.Y.Z`.

## Provisioning flow

Region and commercial `CUSTOMER_ID` are known at signup.

1. **Allocate cell** — pick regional account/cell with Cognito User Pool headroom (see quotas below). Record `cell_id`.
2. **Choose install identity** — unique `INSTALL_ID`, free-form `INSTALL_ROLE` (`prod`, `staging`, `test-eu`, …). Multi-env peers share `CUSTOMER_ID` ([multi-env-deploy.md](./multi-env-deploy.md)).
3. **Allocate VPC CIDR** — default `10.40.0.0/16` is fine for non-peered installs; if vendor ops will attach Transit Gateway / peering later, assign a unique `/16` (or tighter) via `vpc_cidr`.
4. **Apply install stack**

```bash
cd sdk/aws/deploy/ecs
terraform init
terraform apply \
  -var="channel=managed" \
  -var="cell_id=us-east-1-a" \
  -var="customer_id=acme" \
  -var="install_id=acme-prod-use1" \
  -var="install_role=prod" \
  -var="vpc_cidr=10.41.0.0/16" \
  -var="api_image=…/one-api:VERSION" \
  -var="worker_image=…/one-worker:VERSION" \
  -var="db_password=…" \
  -var="api_keys=…" \
  -var="certificate_arn=arn:aws:acm:…" \
  -var='cognito_callback_urls=["https://app.example/callback"]' \
  -var='cognito_logout_urls=["https://app.example/logout"]'
```

5. **Register inventory** — store outputs (`alb_dns_name`, `cognito_user_pool_id`, `oidc_issuer`, `secrets_arn`, `upgrade_automation_document_name`) in the vendor inventory (outside product). Tag resources already carry `Channel`, `CellId`, `CustomerId`, `InstallId`.
6. **Run isolation checklist** — `sdk/aws/deploy/managed/scripts/isolation-checklist.sh` (documented checks; live AWS probes where credentials allow).
7. **Hand off** — customer Admin UI against this install’s Cognito Hosted UI / Control IDE against this install’s API URL only.

Repeat steps 2–7 for each additional environment under the same `CUSTOMER_ID`.

## Cognito scaling (user pool per install)

Quotas are **per AWS account × Region** and shared across all installs in that cell ([Cognito quotas](https://docs.aws.amazon.com/cognito/latest/developerguide/quotas.html)):

| Resource | Default soft | Adjustable max |
|---|---|---|
| User pools / Region | 1,000 | 10,000 |
| App clients / user pool | 1,000 | 10,000 |
| Users / user pool | 40,000,000 | contact AWS |
| API category RPS | pooled across **all** pools in the account | purchasable |

**App clients do not replace pools.** They are the machine-identity mechanism *inside* each install’s pool. Sharing one pool across commercial customers is rejected: Hosted UI session cookies are pool-scoped and would enable cross-app-client session reuse.

### Headroom math

`capacity ≈ floor(quota / envs_per_customer)`. Example at default 1,000 pools with avg 3 envs → ~333 commercial customers per cell before increase.

### Cell-split policy

Managed fleet overlay alarms and runbooks use these thresholds against **User pools / Region** utilization:

| Utilization | Action |
|---|---|
| **50%** | Plan next cell or Service Quotas increase; freeze non-essential pool experiments |
| **70%** | Open quota increase **or** provision sibling cell account; prefer cell split if growth is sustained |
| **85%** | **Block** new commercial signup into this cell until headroom restored; route new customers to another cell |

Terraform: [sdk/aws/deploy/managed/quota_alarms.tf](../deploy/managed/quota_alarms.tf) + metric publisher script.

## Ops IAM fence

| Principal | Purpose |
|---|---|
| ECS execution / API / worker task roles | Product runtime only; Cognito + Secrets ARNs scoped to **this** install |
| `OneManagedFleetOps` | Inventory, describe, start install-local upgrade Automation — **deny** `secretsmanager:GetSecretValue` |
| `OneManagedBreakglass` | MFA-gated secret read for support incidents; tag-conditioned |

See [sdk/aws/deploy/managed/fleet_iam.tf](../deploy/managed/fleet_iam.tf) and [managed-channel-security.md](./managed-channel-security.md).

## Related

- [release-cicd.md](../../../docs/release-cicd.md) — product publish vs channel rolls; same-version parity
- [architecture.md](../../../docs/architecture.md) — managed subscription summary
- [aws-fargate.md](./aws-fargate.md) — ECS reference components
- [ADR-001](../../../docs/adr/001-dedicated-install.md) / [ADR-006](../../../docs/adr/006-jwt-auth.md)
- [BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md) / [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md)
