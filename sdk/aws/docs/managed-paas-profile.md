# AWS managed PaaS profile (community)

> **Not product GA.** Not a second Majesta One Path A. Product Path A remains DigitalOcean App Platform ([self-host.md](../../../docs/self-host.md)).

How AWS customers get a **managed PaaS–shaped** day-2 experience that aligns with DO App Platform — without equating raw ECS Fargate Terraform with App Platform.

**Contract:** [deploy-cloud-capability-contract.md](../../../docs/architecture/deploy-cloud-capability-contract.md)  
**Power path (existing):** [aws-fargate.md](./aws-fargate.md) — ECS Fargate + ALB + RDS Terraform

**Note:** AWS App Runner is closed to new customers (maintenance mode). Majesta One does **not** document App Runner or ECS Express Mode as paths. The community managed profile is a single **opinionated ECS Fargate** stack: api + worker + ALB + RDS.

---

## Role framing

| Role | DigitalOcean (product) | AWS (this profile) |
|---|---|---|
| Managed app PaaS | App Platform | Opinionated **ECS Fargate** services (api + worker) exposing Deploy cloud **verbs** |
| Managed Postgres | Managed Databases | RDS / Aurora |
| Power / network control | DOKS + Helm | EKS or full ECS+Fargate + Helm ([aws-fargate.md](./aws-fargate.md)) |

Do **not** document or UX “Fargate vs DigitalOcean.” Use **managed PaaS vs power path**.

---

## Target shape: opinionated ECS Fargate + RDS

- **API + worker:** curated ECS Fargate services `one-api` + `one-worker` on one cluster (subset of [deploy/ecs](../deploy/ecs/)).
- **Ingress:** ALB with HTTPS (customer certificate).
- **Database:** RDS Postgres 16+ (one DB per install).
- **Images:** GHCR digests via ECR pull — CI pushes images first; adapter rolls via Ops or `UpdateService` force new deployment.

Implements the **same Deploy cloud verbs** behind the `aws` `CloudHost` adapter ([`cloudhost/host.go`](../cloudhost/host.go)).

---

## Adapter expectations

Until product GA:

1. Live under [`sdk/aws`](../README.md) — custom `main` / ops wiring; **no** AWS driver in the product binary.
2. Map 1:1 to [Deploy cloud capability contract](../../../docs/architecture/deploy-cloud-capability-contract.md) verbs.
3. Install-local AWS credentials (task role / keys); billing on the **customer** AWS account.
4. Advertise `cloudHost: "aws"` + `capabilities.cloud` when wired — IDE remains a JWT client of Deploy/Ops.
5. Consumers call **host-free** `/deploy/v1/cloud/*` (same routes as DO).

### CloudHost package (`sdk/aws/cloudhost`)

| Adapter | File | AWS APIs (target) |
|---|---|---|
| **ECS Fargate + RDS** (api + worker) | `host.go` | `DescribeServices`, `UpdateService`, `ModifyDBInstance`, … |

Verb skeleton behavior:

| Verb | ECS+RDS path |
|---|---|
| status / account | config / STS `GetCallerIdentity` |
| bind | cluster + service ids + RDS id |
| describe | `DescribeServices` + ALB |
| scale | `UpdateService` (desired count / capacity provider) |
| resizeDatabase | `ModifyDBInstance` |
| provisionPeer | create ECS services + RDS in customer account |
| redeploy | `UpdateService` force new deployment (prefer `/ops/v1` for product rolls) |

**Custom main registration (community):**

```go
import awshost "github.com/MajestaNet/ide/sdk/aws/cloudhost"

host := awshost.NewECSHost(awshost.Config{
    Region: "us-east-1", Cluster: "one",
    APIService: "one-api", WorkerService: "one-worker",
    RDSInstanceID: "one-pg",
})
// Bridge cloudhost.Host → product deploy.CloudHost in your custom main,
// then: deployEng.SetCloudHost(bridged)
_ = host
```

Product image rolls stay **Ops** (`/ops/v1`), same as DO (ADR-007).

---

## Non-goals

- Promoting this profile to product Path A without an explicit backlog + docs change
- Equating raw Fargate reference architecture with App Platform in customer-facing copy
- **App Runner** or **ECS Express Mode** (not Majesta One paths)
- Vendor-operated AWS managed subscription fleets (historical notes under [managed-channel.md](./managed-channel.md) remain non-GA)

---

## Related

- [`sdk/aws/README.md`](../README.md) · [aws-fargate.md](./aws-fargate.md)  
- [digitalocean-distribution-build-plan.md](../../../docs/architecture/digitalocean-distribution-build-plan.md)  
- [BP-030](../../../backlog/BP-030-deploy-api-digitalocean-apps.md) (DO adapter first) · [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md)
