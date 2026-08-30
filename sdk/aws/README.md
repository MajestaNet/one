# Majesta One AWS community SDK

Community best-effort helpers for AWS-shaped Path B installs. **Not product GA.** **Not** a second product Path A.

Preferred product paths remain **DigitalOcean App Platform** (only Path A) and portable Helm ([docs/self-host.md](../../docs/self-host.md)).

## Managed PaaS vs power path

| Role | AWS in this SDK | DO product equivalent |
|---|---|---|
| **Managed app PaaS** | [Opinionated ECS Fargate profile](./docs/managed-paas-profile.md) exposing Deploy cloud **verbs** | App Platform + Managed Postgres |
| **Power / network control** | [ECS Fargate + ALB](./docs/aws-fargate.md) or Helm on EKS | DOKS + Helm |

Do **not** equate Fargate with App Platform. Host-agnostic day-2 verbs: [deploy-cloud-capability-contract.md](../../docs/architecture/deploy-cloud-capability-contract.md). DO implements them in product first ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)); an AWS adapter stays community until GA’d.

## What’s included

| Path | Contents |
|---|---|
| [`deploy/ecs/`](./deploy/ecs/) | ECS Fargate + ALB Terraform reference (**power** path) |
| [`deploy/marketplace/`](./deploy/marketplace/) | Historical AWS Marketplace Quick Launch CFN |
| [`deploy/managed/`](./deploy/managed/) | Historical managed-cell fleet overlay TF (non-GA) |
| [`docs/`](./docs/) | [managed-paas-profile.md](./docs/managed-paas-profile.md), Fargate, Marketplace, managed-channel notes |
| [`cloudhost/`](./cloudhost/) | CloudHost-shaped ECS+RDS adapter **skeleton** (Deploy `/deploy/v1/cloud/*` verbs; not in product binary) |
| `identity/` | Cognito write-through (`NewCognitoBackend`) |
| `ops/` | ECS product rolls (`NewAWSRoller`) |
| `edge/` | WAFv2 exposure reconcile (`NewAWSWAFRoller`) |

## How to wire

1. Install Majesta One via **Path B** (Compose or Helm) **or** apply the community ECS stack under `deploy/ecs/`.
2. For Cognito / WAF / Ops ECS rollers outside the stock product binary, build a **custom `main`** that imports this module — do not fork `cmd/api` into a product variant.
3. Pin the same GHCR digests as product releases; never float on `:latest`.
4. Treat managed-subscription and Marketplace docs as **optional / historical** — not a Majesta One commercial channel.

```bash
# Go module (standalone — no product internal imports)
cd sdk/aws
go get github.com/MajestaNet/ide/sdk/aws@latest   # or replace with a local path
```

```go
import (
	"github.com/MajestaNet/ide/sdk/aws/edge"
	"github.com/MajestaNet/ide/sdk/aws/identity"
	"github.com/MajestaNet/ide/sdk/aws/ops"
)

cognito := identity.NewCognitoBackend(identity.CognitoConfig{UserPoolID: "…", Region: "us-east-1"})
roller := ops.NewAWSRoller(ops.ECSConfig{Cluster: "…", /* … */})
waf := edge.NewAWSWAFRoller(edge.WAFConfig{WebACLName: "…", WebACLID: "…", IPSetIDs: map[string]string{ /* … */ }})
```

```bash
# Example: apply community ECS reference
cd sdk/aws/deploy/ecs
terraform init
terraform apply \
  -var='api_image=ghcr.io/majestanet/one-api@sha256:…' \
  -var='worker_image=ghcr.io/majestanet/one-worker@sha256:…' \
  # … customer_id, install_id, certificate_arn, secrets …
```

Auth adapter notes: [docs/auth-adapters.md](../../docs/auth-adapters.md) (Cognito via this SDK).

| Doc | Role |
|---|---|
| [docs/managed-paas-profile.md](./docs/managed-paas-profile.md) | AWS managed PaaS analog (opinionated ECS Fargate api+worker) — **not** Path A |
| [docs/aws-fargate.md](./docs/aws-fargate.md) | Power-path ECS Fargate reference |
| [deploy-cloud-capability-contract.md](../../docs/architecture/deploy-cloud-capability-contract.md) | Stable Deploy cloud verbs across hosts |

## Support model

- Apache-2.0; contributions welcome.
- Best-effort community maintenance — no managed subscription SLA.
- Product AuthN default remains Majesta One JWT; Cognito is optional.
- AWS cloud drivers do **not** ship in the product binary until an explicit GA decision.
