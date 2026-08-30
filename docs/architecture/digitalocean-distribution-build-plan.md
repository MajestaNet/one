# DigitalOcean distribution — build plan

Product direction: **open-source Majesta One backend**; **dual-path install** — **Path A** DigitalOcean App Platform (default) + **Path B** self-install from image (Compose + Helm); community [`sdk/`](../../sdk/README.md) optional Path B extensions (not GA); optional Control IDE with DO Govern UI frozen ([BP-027](../adr/030-install-agent-runtime.md)).

**Active execution:** [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) (App Platform 1-Click packaging → Deploy API DO cloud; IDE Govern frozen)  
**Playbook:** [agent-deploy.md](./agent-deploy.md) · [agent-api-families.md](./agent-api-families.md) · [agent-control-ide.md](./agent-control-ide.md)  
**Backlog:** [BP-011](../../backlog/BP-011-container-marketplace-fargate.md) · [BP-002](../../backlog/BP-002-dedicated-install-fleet-ops.md) · [BP-015](../adr/030-install-agent-runtime.md) · [BP-026](../../backlog/BP-026-oss-security-public-backlog.md) · [BP-027](../adr/030-install-agent-runtime.md) (IDE Govern — frozen) · [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) (marketplace publish deferred) · [BP-029](../../backlog/BP-029-app-platform-install.md) · [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)  
**Domain agents:** `deploy-ops` (App Spec / Deploy API); `api-families` for routes; `control-ide` only for BP-065/BP-066 lockstep (BP-027 frozen)

---

## Thesis

> Most customers should get Majesta One running on **DigitalOcean App Platform** (Path A — container images + Managed PostgreSQL) with minimal Kubernetes knowledge. Operators who want cluster control use **Path B** — Compose or **Helm on DOKS / EKS / AKS / GKE**. Community cloud SDKs under [`sdk/`](../../sdk/README.md) (e.g. AWS ECS) are optional Path B extensions, not a third install product. Marketplace publish stays deferred ([BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md)). Day-2 manage / scale / provision of DO Apps is owned by the **Deploy API** ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)); Control IDE DO Govern ([BP-027](../adr/030-install-agent-runtime.md)) is frozen.

---

## Product decisions (locked for this plan)

| Decision | Choice | Rationale |
|---|---|---|
| Backend license | **Open source** (entire repository) | Apache-2.0 for the entire repository, including Control IDE |
| **Default customer path (A)** | **DigitalOcean App Platform** + Managed PostgreSQL | **Only** product Path A — lowest friction; higher managed cost OK |
| **Self-install path (B)** | Compose + **Helm** (`deploy/helm/one`) on DOKS, EKS, AKS, GKE, on-prem | Same images; network control |
| Community cloud SDKs | [`sdk/aws`](../../sdk/aws/README.md) (+ azure/gcp stubs) | Optional Path B extensions; **not** product GA; **not** a second Path A |
| AWS “managed PaaS” analog | Opinionated **ECS Fargate** profile (api + worker + ALB + RDS) exposing Deploy cloud verbs | Community `sdk/aws` only until GA’d — see [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) |
| AWS Fargate / ECS reference | **Power path** (Path B community) | Do **not** equate Fargate with App Platform |
| Marketplace (strategy) | App Platform–first Deploy / listing when vendor-ready; **also** maintain K8s 1-Click | Listing publish deferred — [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) |
| Near-term (no vendor account) | Documented App Spec + Helm/self-host docs | [self-host.md](../self-host.md), [BP-029](../../backlog/BP-029-app-platform-install.md) |
| AWS Marketplace / managed subscription | **Not GA goals** | Historical/optional under `sdk/aws/` only |
| Droplet 1-Click / AMI / EC2 | **Non-goal** | Containers only |
| Multi-env baseline | Separate App **or** namespace+DB per env; HA Postgres **prod only** | Small-company topology |
| AuthN | Majesta One JWT + bootstrap API keys; Cognito not required | ADR-006 / ADR-015; Cognito via `sdk/aws` optional |
| Day-2 cloud ops | **Deploy API** host-agnostic **verbs**; DO adapter first ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)) | Install-local credentials; provision peer / scale / resize — [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) |
| IDE DO infra UI | Frozen [BP-027](../adr/030-install-agent-runtime.md) | Existing JWT client of Deploy API; do not add OAuth helper chrome |
| Plane fence | Cloud credentials on the **install**; not a vendor multi-tenant fleet plane in `cmd/api` | ADR-001 / ADR-012 |

---

## Role matrix (managed PaaS vs power path)

Compare **roles** across clouds — not “Fargate vs DigitalOcean”:

| Role | DigitalOcean (product) | AWS (community / future) | GCP (future) | Azure (future) |
|---|---|---|---|---|
| **Managed app PaaS** | App Platform | Opinionated ECS Fargate (curated api+worker presenting the same Deploy verbs) | Cloud Run | Container Apps |
| **Managed Postgres** | Managed Databases | RDS / Aurora | Cloud SQL | Azure Database for PostgreSQL |
| **Managed inference** (agents) | Inference Engine | Bedrock / BYO | Vertex AI / BYO | Azure OpenAI / BYO |
| **Power / network control** | DOKS + Helm | EKS or ECS+Fargate + Helm | GKE + Helm | AKS + Helm |
| **Deploy cloud adapter** | Product `/cloud/digitalocean/*` | `sdk/aws` managed profile (**not** Path A) | `sdk/gcp` stub | `sdk/azure` stub |

Contract detail (stable verbs, AuthZ, IDE expectations): [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md).

---

## Target install shapes

### A. Path A — DigitalOcean App Platform (default)

```text
Customer DO account
  └── App Platform app (per install / env)
        ├── service: one-api   (GHCR image @ digest)
        ├── worker:  one-worker
        └── Managed PostgreSQL 16+ (prod: primary + standby)
  Secrets / env: DATABASE_URL, API_KEYS, AUTH_JWT_*, CUSTOMER_ID, INSTALL_*
  Client: Control IDE → App Platform HTTPS URL
```

**Happy path (near-term docs; Marketplace later)**

1. Create/attach Managed PostgreSQL (prod-sized; not App Platform “dev DB” for real use)  
2. Deploy App Spec from `deploy/digitalocean/` (images pinned by digest)  
3. Open Control IDE against the app URL  

**Later (BP-028):** Marketplace / Deploy-to-DigitalOcean flow wraps the same App Spec.

### B. Path B — Self-install from image (Compose + Helm)

```text
Compose (local/simple)  OR  DOKS | EKS | AKS | GKE | on-prem + Helm deploy/helm/one
  + external Postgres 16+
  + optional cloud LB / Ingress / network policy / firewall
```

Use when the customer wants VPC/network control, multi-namespace topology, or already runs Kubernetes. Future **K8s 1-Click** (DO Marketplace) and cloud-specific network notes stay in scope; Droplet/AMI do not.

### Community SDK (optional Path B extension)

[`sdk/aws`](../../sdk/aws/README.md) (ECS / Cognito / WAF notes) and azure/gcp stubs — **not** a third install product.

---

## DigitalOcean APIs — Deploy API first (Q: provision a new dev env?)

**Yes.** DigitalOcean Apps + Databases APIs can create/resize Majesta One environments. **Primary surface:** install-local Deploy API ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)). IDE Govern UI ([BP-027](../adr/030-install-agent-runtime.md)) is frozen.

| Capability | DO API surface | Majesta One surface |
|---|---|---|
| Create App Platform app from App Spec | `POST /v2/apps` | Deploy `POST …/cloud/digitalocean/environments` |
| Update / redeploy (product upgrade) | `PUT /v2/apps/{id}`, create deployment | **Ops** `/ops/v1/upgrades` (App Platform roller); optional thin Deploy helper |
| Managed PostgreSQL create / resize | Databases API | Deploy scale/resize + provision |
| DOKS create / node pool scale | Kubernetes API | **Backlog** (power path; not Wave A/B) |
| Auth | Install-local token / PAT (`DIGITALOCEAN_API_TOKEN`) | Not a One-hosted fleet credential store |

**Constraints:** billing stays on the **customer** DO account; prefer **App Platform + Managed DB** for provisioned **dev** envs. Execution detail: [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md).

---

## Phases

### Phase 0 — Strategy alignment (docs / backlog)

| Work | Status |
|---|---|
| Build plan + BP-011 / BP-028 / BP-029 / BP-030 strategy (App Platform default; Deploy API day-2) | This revision + [active plan](./do-app-platform-deploy-api-build-plan.md) |
| BP-027 reframed then frozen: IDE JWT client of Deploy API | Historical |

### Phase 1 — OSS + public images

| Work | Status |
|---|---|
| Apache-2.0 for the entire repository (including Control IDE); GHCR on `v*`; digests file | **Done** |

### Phase 2 — Helm (multi-cloud K8s path)

| Work | Status |
|---|---|
| `values-doks.yaml` / digests / install identity / NOTES | **Done** |
| Network notes for EKS/AKS (same chart) | Ongoing in [self-host.md](../self-host.md) |

### Phase 2b — App Platform packaging ([BP-029](../../backlog/BP-029-app-platform-install.md)) — **active Wave A**

See [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) Wave A. Prioritize validated App Spec + `doctl` runbook + digest mapping (1-Click ready; Marketplace publish still deferred).

### Phase 2c — Deploy API DO cloud ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)) — **active Wave B**

Manage / scale / provision App Platform installs via `/deploy/v1/cloud/digitalocean/*`. Product image rolls remain Ops (ADR-007).

### Phase 3 — Marketplace listings (**deferred → BP-028**)

App Platform–oriented listing / Deploy flow **first**; optional Kubernetes 1-Click **backlog**. Blocked on DO Vendor Portal.

### Phase 4 — Other-cloud / registry docs

| Work | Status |
|---|---|
| [self-host.md](../self-host.md) Compose + Helm + other clouds | **Done** (extend for App Platform default) |
| Further K8s / EKS / AKS depth | **Backlog** — not Wave A/B |

### Phase 5 — Control IDE DigitalOcean infra ([BP-027](../adr/030-install-agent-runtime.md))

**Frozen.** Do not add Control IDE DigitalOcean Govern chrome ([ADR-030](../adr/030-install-agent-runtime.md)). Existing panels may keep calling Deploy API as a JWT client.

---

## Explicit non-goals (until backlog says otherwise)

- Publishing Marketplace listings before BP-028 blockers clear  
- Droplet 1-Click / AMI  
- Vendor-operated managed subscription fleets  
- A second **product** Path A on AWS/GCP/Azure (community managed profiles only until GA’d)  
- Using App Platform **development database** for production  
- Embedding DO credentials in product images / vendor fleet control  
- Implementing K8s enhancements in the same wave as App Spec / Deploy API (IDE Govern is frozen)  
- Equating ECS Fargate with App Platform in product docs or IDE UX

---

## Suggested sequencing

```text
Strategy (this) ──► Wave A BP-029 (App Spec / 1-Click packaging)
                 ──► Wave B BP-030 (Deploy API DO cloud)
Helm path remains supported (already shipped; enhancements backlog)
BP-028 Marketplace ── deferred (uses Wave A artifacts)
BP-027 IDE DO Govern ── frozen (ADR-030)
```

---

## Checklist — packaging ready (without live Marketplace)

- [x] OSS license + public `v*` images (GHCR workflow)  
- [x] Helm values + NOTES for DOKS / generic K8s  
- [x] App Spec + App Platform section in self-host (Wave A)  
- [x] Self-host / multi-env guidance published  
- [x] BP-028 still deferred — not silently started  
- [x] BP-030 Partially mitigated — Deploy API DO cloud shipped  
- [x] BP-027 Frozen — IDE Govern is not a product track  

## Checklist — Marketplace GA (BP-028)

- [ ] Vendor Portal + App Platform–first listing / Deploy flow  
- [ ] Optional DO Kubernetes 1-Click maintained (backlog track)  
- [ ] Digests pinned; install-tested  

---

## Related

- [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) — host-agnostic Deploy cloud verbs  
- [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) — **active execution**  
- [self-host.md](../self-host.md) · [`sdk/aws`](../../sdk/aws/README.md)  
- [BP-011](../../backlog/BP-011-container-marketplace-fargate.md) · [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) · [BP-029](../../backlog/BP-029-app-platform-install.md) · [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [BP-027](../adr/030-install-agent-runtime.md)  
- DO: [App Spec](https://docs.digitalocean.com/products/app-platform/reference/app-spec/) · [Apps API](https://docs.digitalocean.com/products/app-platform/reference/api/)
