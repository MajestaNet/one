> **Not product GA.** Community-maintained under [`sdk/aws`](../README.md) — optional Path B extension only. Preferred install: [docs/self-host.md](../../../docs/self-host.md) (Path A App Platform / Path B Compose+Helm).

# AWS Marketplace packaging notes (historical / optional)

> **Not a GA channel.** Preferred distribution is OSS images + Helm ([self-host.md](../../../docs/self-host.md)); DigitalOcean Kubernetes 1-Click is the preferred marketplace **when** [BP-028](../../../backlog/BP-028-digitalocean-marketplace-listing.md) unblocks. Keep this file as an optional AWS reference only ([BP-011](../../../backlog/BP-011-container-marketplace-fargate.md)).

Majesta One can be packaged as a **dedicated install container** install on **ECS Fargate**. Each subscriber gets their own VPC stack (or isolated services), RDS database, and secrets. There is no shared multi-tenant application layer (ADR-001).

**AuthN (target):** One-issued JWT ([ADR-006](../../../docs/adr/006-jwt-auth.md)); Cognito User Pool in the reference Terraform is **transitional only** and not required for GA.

**AMI / EC2 is not a fulfillment path.** Compose remains local/dev; Helm is the portable production path (`deploy/helm/one`).

## Artifacts

| Artifact | Path |
|---|---|
| Container images (Go) | `deploy/Dockerfile` (`CMD=api\|worker`) — static binary / distroless |
| Customer install package (Marketplace) | [`sdk/aws/deploy/marketplace/`](../deploy/marketplace/) — Quick Launch CFN + allowlisted assets |
| ECS Fargate reference (detailed) | `sdk/aws/deploy/ecs/` (Terraform) — self-managed / ops; not listing copy |
| Architecture | [aws-fargate.md](./aws-fargate.md), [ADR-005](../../../docs/adr/005-go-runtime.md) |
| Helm (optional EKS) | `deploy/helm/one` |
| Compose (local/dev) | `deploy/docker-compose.yml` |
| Control IDE | Separate `control-ide-v*` desktop installers — **not** in Marketplace images |

## IP / distribution posture

| Concern | Guidance |
|---|---|
| Go binary reverse engineering | Strip symbols (`-s -w`) + trimpath + distroless. **Do not** treat obfuscation (`garble`) as Marketplace GA default — subscribers already pull images. See [docs/security.md](../../../docs/security.md). |
| AWS services (RDS, ALB, Secrets, WAF) | Not hideable; they are AWS-managed. Protect **secret values** in Secrets Manager; publish only secret **names** and least-privilege IAM. |
| Marketplace vs multi-step | **One-shot** container Quick Launch for the runtime ([aws/marketplace/](../deploy/marketplace/)). Multi-step only for separate channels (Control IDE private download; enterprise SSO onboarding; future managed subscription). |
| Vendor plane | Never attach `docs/`, `backlog/`, `.cursor/`, or agent playbooks as Marketplace listing assets. |
| Channel rolls | Product `v*` release **publishes** digests only; Marketplace publish and managed fleet rolls are separate, human-gated, and must pin the **same** digests ([release-cicd.md](../../../docs/release-cicd.md#channel-promotion-marketplace--managed)). |

## Procurement / listing checklist

1. **Seller registration** — AWS Marketplace Management Portal; tax/banking complete.
2. **Product type** — **Container** listing (ECS); dedicated install SKU dimensions map to `FEATURE_FLAGS` (e.g. `agents`).
3. **Pricing model** — Contract / usage; correlate billable usage with stack tag `marketplace.sku` / `PRODUCT_VERSION`.
4. **Security review** — Provide Fargate architecture (ALB + private tasks + RDS Multi-AZ + Majesta One JWT AuthN), IAM, and [security.md](../../../docs/security.md).
5. **Listing assets** — Description, architecture diagram, EULA (Apache-2.0 + customer data isolation), support contacts. Use the **customer install package** only — not the vendor monorepo narrative.
6. **Fulfillment** — Publish images to ECR Public or Marketplace registry; attach [`sdk/aws/deploy/marketplace/quickstart.yaml`](../deploy/marketplace/quickstart.yaml) for Quick Launch (detailed TF remains at `sdk/aws/deploy/ecs/` for self-managed ops).

Remaining portal work (listing copy, metering, private offers) is tracked in [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md).

## Reference topology (per install)

```text
Internet → ALB (2 AZs, TLS)
              → ECS Fargate: one-api (**Go** static binary, ≥2 tasks)
              → ECS Fargate: one-worker (**Go**, ≥1)
         RDS Postgres 16 Multi-AZ
         Secrets Manager · JWT signing keys (BP-013) · CloudWatch / OTEL
         (optional transitional Cognito — not product default)
```

See [aws-fargate.md](./aws-fargate.md) for AuthN split (Majesta One JWT target; API keys bootstrap; ADR-006).

## Required IAM (subscriber account)

Grant the ECS **task execution** and **task** roles least privilege:

| Action | Purpose |
|---|---|
| `secretsmanager:GetSecretValue` on the install secret ARN | `DATABASE_URL`, bootstrap `API_KEYS`, JWT signing material, optional `DEPLOY_SHARE_SECRET` |
| `kms:Decrypt` (if secret uses CMK) | Read encrypted secrets |
| `logs:CreateLogGroup/Stream`, `logs:PutLogEvents` | CloudWatch logs |
| `ecr:GetAuthorizationToken` + `ecr:BatchGetImage` | Pull API/worker images |

Do **not** grant cross-account DB access; each install uses its own RDS.

## Required customer inputs

| Input | Notes |
|---|---|
| `DATABASE_URL` | Postgres 16+ (RDS from the stack). One DB per install. |
| `API_KEYS` | Bootstrap/break-glass keys; scopes `key:client+metadata+deploy+ops`. Prefer Majesta One JWT (BP-013). |
| JWT signing keys | Install-local keys for Majesta One Token Service (BP-013); store in Secrets Manager |
| `OIDC_*` (optional, transitional) | Legacy Cognito/OIDC humans path — not GA default (ADR-006) |
| `CUSTOMER_ID` | Shared across that customer's installs (multi-env Deploy). |
| `INSTALL_ID` / `INSTALL_ROLE` | Unique install id; free-form role (`prod`, `staging`, …). |
| `PRODUCT_VERSION` | Platform version string for Deploy compatibility checks. |
| Optional | `DEPLOY_PEER_MODE`, `DEPLOY_SHARE_SECRET`, `OTEL_EXPORTER_OTLP_ENDPOINT` |

## Networking

- Place Fargate tasks + RDS in **private** subnets across **2 AZs**.
- Expose API via **ALB only** (no public task IPs).
- Security groups: ALB→API :8080; API/worker→RDS :5432; egress to Secrets Manager, ECR (and transitional OIDC JWKS only if still enabled).
- Optional WAF on the ALB for Marketplace hardening.

## RDS

- Postgres 16+, Multi-AZ for production.
- Enable automated backups (see ops / backup guidance).
- Do not share one RDS across Marketplace subscribers.

## Secrets

Store at minimum:

```json
{
  "DATABASE_URL": "postgres://…",
  "API_KEYS": "prod-bootstrap-key:client+metadata+deploy+ops,…",
  "CUSTOMER_ID": "acme",
  "INSTALL_ID": "acme-prod-use1",
  "INSTALL_ROLE": "prod",
  "DEPLOY_SHARE_SECRET": "optional-hmac-secret",
  "JWT_SIGNING_KEY": "install-local-signing-material"
}
```

Transitional installs may still include `OIDC_ISSUER` / `OIDC_AUDIENCE`; prefer Majesta One JWT per ADR-006.

## Upgrade procedure

Prefer the guided install-local path ([product-upgrades.md](../../../docs/product-upgrades.md), [ADR-007](../../../docs/adr/007-platform-ops-upgrades.md)) — **not** `/deploy/v1` (that API promotes customer metadata between installs).

1. **Images** — Build/push new `PRODUCT_VERSION` tags for `api` and `worker`.
2. **Confirm** — AWS admin runs SSM Automation `One-ProductUpgrade` (shipped with `sdk/aws/deploy/ecs/`) **or** `POST /ops/v1/upgrades` with scope `ops` + admin. Inputs: target images + version.
3. **ECS** — Register a new task definition revision; update services (`desired_count` preserved). Rolling deploy + **deployment circuit breaker with rollback**. Multi-AZ gives HA for zero-downtime task replace; do not stage by AZ (shared RDS).
4. **Kernel DDL** — API/worker apply idempotent kernel schema on boot; keep migrations forward-compatible across the roll window.
5. **Gate** — `GET /healthz`, `GET /readyz`, then Deploy test runs: product `PlatformSmoke` + optional customer `PostUpgradeSmoke`.
6. **Rollback** — Automation/Ops API (or circuit breaker) redeploys the previous task definition revision; restore RDS from snapshot only if a kernel migration is incompatible (rare).

Customer custom fields never require Majesta One DDL migrations (metadata + JSONB only).

## SKU / feature flags

Map Marketplace dimensions to `FEATURE_FLAGS` / `feature_flags` table:

- `agents` — agent run API surface (keep off until BP-006)

The managed `core` data model (User / Account / Contact) always ships with the product image when `AUTO_SEED=1`; it is not a Marketplace SKU flag.

Stack variable / Helm value `marketplace.sku` is informational for billing correlation.

## Related

- [BP-011 Container Marketplace](../../../backlog/BP-011-container-marketplace-fargate.md)
- [BP-013 Majesta One JWT + unified principals](../../../backlog/BP-013-jwt-unified-principals.md)
- [ADR-006 Majesta One JWT auth](../../../docs/adr/006-jwt-auth.md)
- [BP-003 Enterprise AuthZ remainders](../../../backlog/BP-003-enterprise-auth.md)
- [AWS Fargate architecture](./aws-fargate.md)
- [Security](../../../docs/security.md)
