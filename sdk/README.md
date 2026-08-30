# Majesta One community SDKs

Optional, **community-maintained** helpers for Majesta One. **Not product GA** and **not** a third install path. DigitalOcean is the first targeted managed path; other cloud providers should arrive here later.

Product install remains dual-path only ([docs/self-host.md](../docs/self-host.md)):

- **Path A** — DigitalOcean App Platform (first targeted managed path)
- **Path B** — Self-install from image (Compose + Helm)

## SDK families

| Tree | Role | ADR / plan |
|---|---|---|
| **`client/`** | OSS auth + Client API kits for customer-hosted **Client Experience** apps (`@one/auth`, `@one/client`) | [ADR-019](../docs/adr/019-client-experience-oss-kits.md) · [client-experience-build-plan.md](../docs/architecture/client-experience-build-plan.md) · [BP-040](../backlog/BP-040-client-experience-oss-kits.md) |
| **`aws/`**, **`azure/`**, **`gcp/`** | Cloud identity / ops / edge / deploy helpers (Path B extensions) | [sdk/aws/README.md](./aws/README.md) |

**Client Experience vs Control IDE:** OSS `sdk/client/` kits are for **end-user browser apps** calling `/auth/v1` + `/client/v1` only. Control IDE is an optional frozen admin client ([ADR-030](../docs/adr/030-install-agent-runtime.md)), not a replacement for customer Experiences.

## Cloud SDKs (Path B extensions)

These SDKs are **Path B extensions**: wire them into a custom `main` or ops pipeline when you need cloud-specific identity, edge, or deploy automation beyond the portable Helm chart. They may also host a **community managed PaaS profile** (e.g. AWS opinionated ECS Fargate api+worker) that speaks the same [Deploy cloud verbs](../docs/architecture/deploy-cloud-capability-contract.md) as DO — **not** a second Path A until GA'd.

**Role framing:** managed app PaaS vs power path (K8s/ECS) — see the matrix in [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md) and [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md).

## License and boundary

| Fact | Detail |
|---|---|
| License | **Apache-2.0** (same as product plane) |
| In product image? | **No** — excluded from `deploy/Dockerfile` / `.dockerignore` |
| Support | Best-effort / community; not a managed subscription channel |
| Contribution | PRs welcome under the usual product boundary rules — do not bake customer customizations into `cmd/` / `internal/` / `migrations/` |

## Layout

```text
sdk/
├── README.md          This file
├── client/            OSS Client Experience kits (ADR-019; scaffold in BP-040 Phase 2)
├── aws/               AWS identity / ops / edge / deploy + docs
├── azure/             Stub (expected layout)
└── gcp/               Stub (expected layout)
```

Each cloud tree aims for the same shape:

| Subdir | Role |
|---|---|
| `identity/` | IdP / Cognito / Entra / IAM helpers |
| `ops/` | Upgrade / roll helpers |
| `edge/` | WAF / exposure reconcile adapters |
| `deploy/` | Terraform / CFN / cloud packaging |
| `docs/` | Cloud-specific runbooks (not product GA) |

## Start here

| Cloud | README | Managed PaaS analog (community) | Power path |
|---|---|---|---|
| AWS | [aws/README.md](./aws/README.md) | [Opinionated ECS Fargate](./aws/docs/managed-paas-profile.md) | [ECS Fargate](./aws/docs/aws-fargate.md) / EKS+Helm |
| Azure | [azure/README.md](./azure/README.md) | Container Apps (future) | AKS+Helm |
| GCP | [gcp/README.md](./gcp/README.md) | Cloud Run (future) | GKE+Helm |
