# Majesta One — ECS Fargate community reference stack

Community best-effort AWS topology (optional Path B). **Not product GA.**

## What this creates

- VPC with 2 public + 2 private subnets (2 AZs)
- Application Load Balancer → Fargate `api` (desired count ≥ 2)
- Fargate `worker` service
- RDS Postgres 16 Multi-AZ
- Secrets Manager install secret (`DATABASE_URL`, bootstrap `API_KEYS`, `AUTH_JWT_SIGNING_KEY`, `DEPLOY_SHARE_SECRET`, `OIDC_*`, …)
- Cognito User Pool + MFA (software token) + groups (`one-client|metadata|deploy|admin`) — **transitional** AuthN backend; Majesta One Roles/PS remain AuthZ SoR ([ADR-006](../../../../docs/adr/006-jwt-auth.md))
- CodeCommit repo `one-<customer_id>` (`provision_customer_repo`, ADR-012) + `CUSTOMER_REPO_*` task env; peer installs reuse via `customer_repo_url`
- WAFv2 WebACL: Client/Auth public by default; Metadata/Deploy/Ops blocked until Metadata exposure reconcile
- Separate IAM task roles for API (Cognito/WAF/Ops) vs worker (logs only)

See [docs/aws-fargate.md](../../docs/aws-fargate.md) and [docs/marketplace.md](../../docs/marketplace.md).
Marketplace Quick Launch (sanitized customer package): [../marketplace/](../marketplace/).

## Images

Prefer **Go** distroless images (ADR-005):

```bash
docker build -f deploy/Dockerfile --build-arg CMD=api -t one-api:go .
docker build -f deploy/Dockerfile --build-arg CMD=worker -t one-worker:go .
```

The image includes `migrations/` at `/migrations` (`MIGRATIONS_PATH`). The API applies kernel DDL on boot.

## Prerequisites

- Terraform ≥ 1.5, AWS credentials with VPC/ECS/RDS/Secrets permissions (Cognito only if enabling transitional OIDC)
- Published `api` / `worker` images (Go distroless from `deploy/Dockerfile`)

## Apply

```bash
cd sdk/aws/deploy/ecs
terraform init
terraform apply \
  -var='api_image=public.ecr.aws/one/api:0.1.0' \
  -var='worker_image=public.ecr.aws/one/worker:0.1.0' \
  -var='db_password=REDACTED_LONG_PASSWORD' \
  -var='api_keys=prod-admin-key:client+metadata+deploy+ops+admin' \
  -var='customer_id=acme' \
  -var='install_id=acme-prod-use1' \
  -var='install_role=prod' \
  -var='certificate_arn=arn:aws:acm:REGION:ACCOUNT:certificate/ID'  # required unless allow_http=true
```

Outputs include `alb_dns_name`, `oidc_issuer`, `cognito_app_client_id`.

## Upgrade

Prefer the guided path ([docs/product-upgrades.md](../../../../docs/product-upgrades.md)):

1. Publish new `api` / `worker` image tags.
2. Optionally set `DEPLOY_SMOKE_API_KEY` (deploy-scoped) in the install Secrets Manager secret for PlatformSmoke / PostUpgradeSmoke.
3. In **Systems Manager → Automation**, run document `One-ProductUpgrade-<stack>` with `ApiImage`, `WorkerImage`, `ProductVersion` only (cluster/service targets are pinned; smoke key is read from Secrets Manager).
4. Or call `POST /ops/v1/upgrades` (scope `ops` + admin) from a Majesta One principal.
5. Circuit breaker rolls back unhealthy ECS deployments automatically; Automation/Ops also roll back when health or Deploy tests fail.

Manual fallback:

1. `terraform apply` with new `api_image` / `worker_image` / `product_version` (forces new task definitions).
2. Smoke `GET http(s)://$alb/healthz`, Client describe, worker logs.

## Auth notes

- **Target (ADR-006 / BP-013):** One-issued JWT for humans, agents, and services; API keys as bootstrap only. `AUTH_JWT_SIGNING_KEY` is generated into the install secret.
- **Cognito:** MFA (software token) required; public UI client uses SRP + auth code (no `USER_PASSWORD_AUTH`); `OIDC_AUTO_PROVISION_USERS=0` by default. One User Pool per install; service/agent app clients are created via Client identity admin write-through.
- **Managed channel:** set `channel=managed` + `cell_id=…` (and optionally unique `vpc_cidr`). Fleet IAM / Cognito quota alarms live in [`../managed/`](../managed/). See [docs/managed-channel.md](../../docs/managed-channel.md).
- Do **not** enable ALB `authenticate-cognito` for API traffic — it blocks API-key / JWT machine clients.

## Non-goals

- AMI / EC2
- Shared multi-tenant control plane
- Cognito as a required Marketplace dependency
- Shared Cognito User Pool across commercial customers (managed or otherwise)