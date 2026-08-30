# Agnostic Deploy Cloud API — active build plan

**Active execution plan** for uplifting Deploy day-2 cloud ops from DO-only routes/types to a **host-free consumer API** with a verb-shaped `CloudHost` port. DigitalOcean remains the only **product Path A** adapter; AWS (and later GCP/Azure) plug in as community adapters without changing clients.

**Contract:** [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md)  
**DO packaging (orthogonal):** [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md)  
**Playbooks:** [agent-deploy.md](./agent-deploy.md) · [agent-api-families.md](./agent-api-families.md)  
**Backlog:** [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [BP-027](../adr/030-install-agent-runtime.md) · [BP-011](../../backlog/BP-011-container-marketplace-fargate.md)

---

## Thesis

> Control IDE and operators call **stable, host-free Deploy cloud routes**. The install selects an **active cloud host** via install-local credentials + binding. Consumers never call provider-specific paths as the primary surface. `/deploy/v1/cloud/digitalocean/*` remains as compatibility aliases.

```text
Control IDE / operators
        │  JWT + deploy (+admin on writes)
        ▼
 /deploy/v1/cloud/*          ← agnostic consumer contract (primary)
        │
        ▼
 DeployEngine → CloudHost port (status|bind|describe|scale|…)
        ├── digitalocean  (product; App Platform + Managed DB)
        ├── aws           (community sdk/aws; opinionated ECS Fargate + RDS)
        ├── gcp / azure   (future stubs)
        └── none          (Path B Helm/Compose — no adapter)

 Compatibility aliases: /deploy/v1/cloud/digitalocean/* → same engine
 Ops /ops/v1/upgrades stays product image rolls (ADR-007)
```

---

## Locked decisions

| Decision | Choice |
|---|---|
| Primary consumer HTTP | Host-free `/deploy/v1/cloud/{status\|binding\|app\|…}` |
| Host selection | Install `cloudHost` from config/binding; advertised on `GET /deploy/v1/environment` |
| DO routes | Kept as **aliases** that delegate to the agnostic handlers |
| Go boundary | Verb-shaped `CloudHost` port with Majesta One DTOs |
| Size / scale model | Abstract `sizeClass` + `instanceCount`; DO maps to App Platform / DB slugs |
| Binding ids | Opaque `appResourceId` / `databaseResourceId` (+ `region`, `displayName`); extras in `providerMeta` |
| Path A | Still DigitalOcean App Platform only |
| AWS / other | Community under `sdk/aws` until explicit GA; product binary registers only DO by default |
| Credentials | Install-local per host; never a Majesta One fleet credential store (ADR-001) |
| Product upgrades | Stay on Ops; `redeploy` remains temporary helper |

---

## Target HTTP contract (primary)

Prefix: `/deploy/v1/cloud`

| Method | Path | Verb |
|---|---|---|
| `GET` | `/status` | status |
| `PUT` | `/binding` | bind |
| `GET` | `/app` | describe |
| `PATCH` | `/app/scale` | scale |
| `PATCH` | `/database/resize` | resizeDatabase |
| `POST` | `/environments` | provisionPeer |
| `GET` | `/environments` | listEnvironments |
| `POST` | `/app/redeploy` | redeploy (temporary; prefer Ops) |

`GET /deploy/v1/environment` capabilities:

- `cloudHost` (string field): `"digitalocean" | "aws" | ""`
- `cloud`: `true` when any adapter is configured
- `digitaloceanCloud`: alias for DO during migration
- Status body continues per-verb flags (`bind`, `scaleApp`, …)

AuthZ: scope `deploy`; mutating routes require `+admin`.

---

## Waves

| Wave | Work | Status |
|---|---|---|
| W0 | Design lock — host-free primary in contract + this plan | Done when docs merged |
| W1 | `CloudHost` port, Majesta One DTOs, host-keyed persistence + DO backfill | In progress with this change set |
| W2 | Agnostic HTTP + DO aliases + environment capabilities | In progress with this change set |
| W3 | DO adapter sizeClass mapping + secret hygiene | In progress with this change set |
| W4 | `sdk/aws` CloudHost-shaped community skeleton | In progress with this change set |
| W5 | Control IDE Govern → agnostic `/cloud/*` | In progress with this change set |
| W6 | Backlog / api-families / self-host / agent-deploy closeout | In progress with this change set |

---

## Explicit non-goals

- Second product Path A on AWS/GCP/Azure
- AWS/GCP/Azure drivers in the default product binary this cycle
- Multi-host binding on a single install
- Helm/K8s provision APIs as Deploy cloud verbs
- Moving product image rolls off Ops
- Vendor-operated multi-tenant cloud credential fleet
- IDE-as-cloud-console

---

## Related

- [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md)
- [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md)
- [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md)
- [sdk/aws/docs/managed-paas-profile.md](../../sdk/aws/docs/managed-paas-profile.md)
