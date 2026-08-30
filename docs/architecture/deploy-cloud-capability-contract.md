# Deploy cloud capability contract

**One-pager** for host-agnostic day-2 cloud ops. Product Path A remains **DigitalOcean App Platform only**. Other clouds implement the same **verbs** via adapters (community `sdk/` until GA’d).

**Playbook:** [agent-deploy.md](./agent-deploy.md) · **Strategy:** [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md)  
**Execution:** [deploy-cloud-agnostic-build-plan.md](./deploy-cloud-agnostic-build-plan.md) (API uplift) · [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) (DO packaging)  
**Backlog:** [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) (DO first) · [BP-027](../adr/030-install-agent-runtime.md) (IDE JWT client) · [BP-011](../../backlog/BP-011-container-marketplace-fargate.md)

---

## Thesis

> Control IDE and operators call **stable, host-free Deploy cloud verbs** under `/deploy/v1/cloud/*`. Each cloud host supplies an **adapter** (install-local credentials, resource binding, provider APIs). DigitalOcean is the only **product** adapter today. `/deploy/v1/cloud/digitalocean/*` remains as **compatibility aliases**. An **opinionated ECS Fargate profile** (api + worker + ALB + RDS) under community `sdk/aws` is the AWS managed PaaS analog — not a second Path A — until explicitly GA’d. (App Runner is closed to new customers; do not use it as a Majesta One path.)

```text
Control IDE Govern  ──JWT──►  Deploy API /deploy/v1/cloud/* (stable verbs)
                                  │
                                  ├── digitalocean  (product; App Platform + Managed DB)
                                  ├── aws           (community; opinionated ECS Fargate + RDS)  ← not Path A
                                  ├── gcp           (future; Cloud Run + Cloud SQL)
                                  └── azure         (future; Container Apps + Postgres)

Ops /ops/v1/upgrades  ──►  product image roll on *this* install (ADR-007; separate from provision)
```

---

## Locked decisions

| Decision | Choice |
|---|---|
| Product Path A | **DigitalOcean App Platform** + Managed PostgreSQL only |
| Path B | Compose + Helm (any K8s); portable images |
| Cloud day-2 surface | **Deploy** family — host-free `/deploy/v1/cloud/*` |
| Compatibility aliases | `/deploy/v1/cloud/digitalocean/*` → same engine |
| Product image upgrades | **Ops** `/ops/v1` (ADR-007) — not Deploy promotions |
| Credentials | **Install-local** token/role; never a Majesta One multi-tenant fleet plane (ADR-001) |
| Billing | Always the **customer** cloud account |
| IDE | JWT client of Deploy/Ops only — no direct cloud console APIs as the primary path |
| AWS Fargate / ECS | **Power / Path B community** — not the DO App Platform equivalent |
| AWS managed PaaS analog | Opinionated **ECS Fargate** profile (api + worker + ALB + RDS) under `sdk/aws` |
| GA of non-DO adapters | Explicit backlog + product decision; until then community best-effort |

---

## Role matrix (managed PaaS vs power path)

Do **not** compare “Fargate vs DigitalOcean.” Compare **roles**:

| Role | DigitalOcean (product) | AWS (community / future) | GCP (future) | Azure (future) |
|---|---|---|---|---|
| **Managed app PaaS** | App Platform | Opinionated **ECS Fargate** services presenting the same verbs | Cloud Run | Container Apps |
| **Managed Postgres** | Managed Databases | RDS / Aurora | Cloud SQL | Azure Database for PostgreSQL |
| **Managed inference** | Inference Engine (Serverless; Deploy `/cloud/inference`) | Bedrock / BYO | Vertex AI / BYO | Azure OpenAI / BYO |
| **Power / network control** | DOKS + Helm | EKS or ECS+Fargate + Helm | GKE + Helm | AKS + Helm |
| **Deploy cloud adapter** | Product `CloudHost` (`digitalocean`) | `sdk/aws` managed profile (not Path A) | `sdk/gcp` stub | `sdk/azure` stub |

**Escape hatch everywhere:** Path B Helm with the portable chart — no cloud adapter required.

---

## Stable verbs (host-agnostic)

These are the **contract**. Primary consumer routes are **host-free**. Provider-namespaced paths are compatibility aliases only.

| Verb | Purpose | Mutating? | AuthZ |
|---|---|---|---|
| **status** | Token/role configured? Binding present? Reachable? Capability flags | No | `deploy` |
| **bind** | Attach this install to existing cloud app + database ids | Yes | `deploy` + admin |
| **describe** | Live summary of bound app (URL, instances/size, image digests) | No | `deploy` |
| **scale** | Scale api/worker instance count or size class | Yes | `deploy` + admin |
| **resizeDatabase** | Resize managed Postgres size / node count | Yes | `deploy` + admin |
| **provisionPeer** | Create peer env (new app + DB) with shared `CUSTOMER_ID`, new `INSTALL_ID` / `INSTALL_ROLE`; upsert peer | Yes | `deploy` + admin |
| **listEnvironments** | Peers + provision audit runs | No | `deploy` |
| **redeploy** | Temporary digest redeploy helper | Yes | `deploy` + admin; prefer **Ops** long-term |
| **inference** | Native managed inference status / enable (DO Serverless; Dev/Standard/Pro) | GET no / PUT yes | `deploy` (+ admin on PUT) |

### Primary routes (host-free)

| Verb | Route |
|---|---|
| status | `GET /deploy/v1/cloud/status` |
| bind | `PUT /deploy/v1/cloud/binding` |
| describe | `GET /deploy/v1/cloud/app` |
| scale | `PATCH /deploy/v1/cloud/app/scale` |
| resizeDatabase | `PATCH /deploy/v1/cloud/database/resize` |
| provisionPeer | `POST /deploy/v1/cloud/environments` |
| listEnvironments | `GET /deploy/v1/cloud/environments` |
| redeploy | `POST /deploy/v1/cloud/app/redeploy` |
| inference | `GET|PUT /deploy/v1/cloud/inference` |

### Compatibility aliases (DigitalOcean)

Same verbs under `/deploy/v1/cloud/digitalocean/*` (legacy field names `appId` / `databaseId` / size slugs still accepted).

### Capability advertisement

`GET /deploy/v1/environment` (and cloud **status**) expose capabilities so the IDE shows one Govern “Cloud” panel without hard-coding a host forever:

| Flag / field | Meaning |
|---|---|
| `cloudHost` (string) | Active adapter id: `"digitalocean"`, `"aws"`, or `""` |
| `cloud` | Any cloud adapter configured |
| `digitaloceanCloud` | DO adapter active (migration alias) |

Per-adapter **status.capabilities** may refine verbs (`bind`, `scaleApp`, `resizeDatabase`, `provisionPeer`, `redeploy`) when a host cannot support one.

### Binding & credentials

| Concern | Rule |
|---|---|
| Binding | Persist host + opaque resource ids on **this** install (`appResourceId`, `databaseResourceId`, region, display name, `providerMeta`) so scale/ops cannot target arbitrary account resources |
| Credentials | Env/secret on the install (`DIGITALOCEAN_API_TOKEN` today); never echo in API responses; never bake into product images |
| Peer provision | Same customer cloud account/team; shared `CUSTOMER_ID`; unique `INSTALL_ID`; return public API URL; register `/deploy/v1/peers` |
| Errors | Map provider 401/403/429 to Majesta One problem responses; no token leakage |
| Size classes | Abstract `sizeClass` (`small` / `medium` / `large` / …); adapters map to provider slugs; raw provider slugs accepted as escape hatch |

### Fence vs Ops / promote

| Concern | Family |
|---|---|
| Provision peer / scale / resize DB / bind | **Deploy** cloud verbs |
| Roll **this** install’s product image digests | **Ops** `/ops/v1/upgrades` |
| Customer metadata promote (repo→org) | **Deploy** bundles/promotions — unrelated to cloud host |

---

## DO size class mapping (product adapter)

| Majesta One `sizeClass` | App Platform slug | Managed DB slug |
|---|---|---|
| `small` | `apps-s-1vcpu-1gb` | `db-s-1vcpu-1gb` |
| `medium` | `apps-s-1vcpu-2gb` | `db-s-1vcpu-2gb` |
| `large` | `apps-s-2vcpu-4gb` | `db-s-2vcpu-4gb` |
| `xlarge` | `apps-s-4vcpu-8gb` | `db-s-4vcpu-8gb` |

Unknown `sizeClass` values that look like provider slugs (`apps-…`, `db-…`) are passed through unchanged.

---

## AWS managed profile (community — not Path A)

**Intent:** Give AWS customers a **managed PaaS–shaped** day-2 experience that speaks these verbs, without promoting AWS to product Path A.

| Option | When |
|---|---|
| **Opinionated ECS (Fargate) profile** | api + worker services + ALB + RDS that **implements these verbs** behind an `aws` adapter — the community managed PaaS analog for Majesta One |

Ship under [`sdk/aws`](../../sdk/aws/README.md) (custom `main` / ops wiring). Do **not** add AWS drivers to the product binary until GA’d. See [managed-paas-profile.md](../../sdk/aws/docs/managed-paas-profile.md) and the CloudHost skeleton under `sdk/aws/cloudhost/`.

---

## IDE expectations

1. Call host-free `/deploy/v1/cloud/*`; gate on `cloud` / `cloudHost` (keep `digitaloceanCloud` fallback during migration).
2. Never call cloud provider APIs directly as the primary control plane.
3. Deep-link to the cloud console only as fallback when the adapter capability is false.

---

## Explicit non-goals

- Second product Path A on AWS/GCP/Azure
- Vendor-operated multi-tenant fleet credential store
- IDE-as-cloud-console
- Equating ECS Fargate with App Platform in docs or UX
- Moving product upgrades off Ops into Deploy cloud verbs

---

## Related

- [deploy-cloud-agnostic-build-plan.md](./deploy-cloud-agnostic-build-plan.md) — API uplift execution  
- [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) — role matrix in strategy context  
- [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) — DO Wave A/B packaging  
- [self-host.md](../self-host.md) · [sdk/README.md](../../sdk/README.md) · [sdk/aws/README.md](../../sdk/aws/README.md)
