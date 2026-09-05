# Self-host Majesta One

**Status:** alpha **0.1.0**. Contracts and packaging can still change in breaking ways.

Install the **open-source** Majesta One product plane (API + worker + migrations). **DigitalOcean App Platform** is the first targeted managed path. Other cloud providers should come later through community SDKs. There are **exactly two** product install paths:

| Path | Who it’s for | Friction |
|---|---|---|
| **A — DigitalOcean App Platform** (first targeted) | Customers who want managed PaaS; OK with managed cost | Lowest |
| **B — Self-install from image** | Operators who run Compose or Helm themselves | Low (Compose) / Medium (Helm) |

Marketplace / one-click publish is **deferred** ([BP-028](../backlog/BP-028-digitalocean-marketplace-listing.md)). Distribution plan: [digitalocean-distribution-build-plan.md](./architecture/digitalocean-distribution-build-plan.md). App Platform packaging: [BP-029](../backlog/BP-029-app-platform-install.md).

**Control IDE** is optional software (`tools/control-ide/`) — not in product images and **not required**. Default topology = one Prod install. Claim via HTTP ([BP-037](../backlog/BP-037-install-claim-customer-sso.md)); builders connect with MCP + CLI ([builder-connect.md](./builder-connect.md) · [ADR-030](./adr/030-install-agent-runtime.md)). Control IDE connect remains documented for humans who use that client ([install-ide-connect-build-plan.md](./architecture/install-ide-connect-build-plan.md)).

## Artifacts

| Artifact | Where |
|---|---|
| Images | `ghcr.io/majestanet/one-api` / `one-worker` on each `v*` tag |
| Digests | GitHub Release `image-digests-X.Y.Z.txt` |
| App Platform App Spec | `deploy/digitalocean/app.yaml` (+ README, MARKETPLACE_PREP) |
| Helm | `deploy/helm/one` (+ `values-doks.yaml`, `values-dev.yaml`) |
| Compose | `deploy/docker-compose.yml` |

**Never float on `:latest`.** Pin digests; set `PRODUCT_VERSION` to the release semver and set `API_REVISION_CURRENT` / `API_REVISION_MIN` to the image’s advertised wire window ([ADR-025](./adr/025-api-revision-versioning.md)).

## Prerequisites

- Postgres **16+** (one database **per install**) — Managed PostgreSQL on DO for App Platform / DOKS
- Bootstrap secrets: `DATABASE_URL`, `API_KEYS`, `AUTH_JWT_SIGNING_KEY` ([ADR-006](./adr/006-jwt-auth.md))
- Wire window: set `API_REVISION_CURRENT` and `API_REVISION_MIN` on every image (defaults `1`/`1` until a sunset policy exists). Clients pin `One-API-Revision`; see [Connect troubleshooting](./architecture/install-ide-connect-build-plan.md#connect-troubleshooting-api-revision).

---

## Path A — DigitalOcean App Platform (managed)

No cluster or Helm required. Active packaging: [deploy/digitalocean/app.yaml](../deploy/digitalocean/app.yaml) · [BP-029](../backlog/BP-029-app-platform-install.md). Day-2 manage/scale/provision: Deploy API ([BP-030](../backlog/BP-030-deploy-api-digitalocean-apps.md)).

### Install (copy-paste)

1. **Create Managed PostgreSQL** (prod: plan with standby / HA). Do **not** use App Platform’s small “dev database” for real traffic.
2. **Edit the App Spec** — set identity + digests:

```bash
cp deploy/digitalocean/app.yaml /tmp/one-app.yaml
# Edit CUSTOMER_ID, INSTALL_ID, INSTALL_ROLE, PLATFORM_PUBLIC_URL
# Pin digests from a GitHub Release asset:
#   ./scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt /tmp/one-app.yaml
go run ./scripts/validate-do-app-spec.go /tmp/one-app.yaml
```

3. **Create the app:**

```bash
doctl auth init   # once
doctl apps create --spec /tmp/one-app.yaml
```

4. **Set secrets** in App Platform (UI or API) if not already in the spec:

| Secret | Notes |
|---|---|
| `DATABASE_URL` | Managed Postgres connection string (`sslmode=require`) |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | Optional pool sizing per process (defaults `10` / `1`) |
| `RETENTION_JOBS_DAYS` / `RETENTION_OUTBOX_DAYS` / `RETENTION_AUDIT_LOG_DAYS` | Optional worker purge windows (`0` disables; defaults 30 / 30 / 180) |
| `API_KEYS` | Cryptographically random bootstrap / break-glass key of at least 32 bytes with explicit `+admin` |
| `AUTH_JWT_SIGNING_KEY` | Random HMAC secret of at least 32 bytes |
| `WEBHOOK_ENCRYPTION_KEY` | Random at-rest secret of at least 32 bytes; optional only when the JWT key is used as fallback |
| `INSTALL_CLAIM_TOKEN` | One-time day-0 claim secret (create first SystemAdmin via `/auth/v1/install/claim`) |
| `DEPLOY_SHARE_SECRET` | Optional; local bundle HMAC when set (not used for cross-install promote) |


5. **Claim the install** (no IDE required):

```bash
curl -sS -X POST "$PLATFORM_PUBLIC_URL/auth/v1/install/claim" \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"$INSTALL_CLAIM_TOKEN\",\"email\":\"admin@example.com\",\"password\":\"choose-a-long-password\"}"
```

Then optionally open Control IDE Connect, or configure SSO:

```bash
curl -sS -X PUT "$PLATFORM_PUBLIC_URL/metadata/v1/install/auth" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  -d '{"oidcIssuer":"https://idp.example.com","oidcAudience":"…","oidcClientId":"…","oidcClientSecret":"…","oidcDisplayName":"Company SSO","jitProvisionUsers":false}'
```

See [install-claim-sso-build-plan.md](./architecture/install-claim-sso-build-plan.md).

### Digests mapping

| Release file (`image-digests-X.Y.Z.txt`) | App Spec |
|---|---|
| `api_digest=sha256:…` | `services[name=api].image.digest` (+ `tag` / `PRODUCT_VERSION` → `X.Y.Z`) |
| `worker_digest=sha256:…` | `workers[name=worker].image.digest` |

Never float on `:latest`.

### Upgrade

Copy-paste Path A product roll (operator-native App Spec). Product image rolls belong on **`/ops/v1`** ([ADR-007](./adr/007-platform-ops-upgrades.md)); until an App Platform roller exists, pin digests in a spec copy and `doctl apps update`. `POST /deploy/v1/cloud/app/redeploy` is a **temporary helper** on the bound app — not the product upgrade API.

```bash
# 1) Download image-digests-X.Y.Z.txt from the GitHub Release, then:
cp deploy/digitalocean/app.yaml /tmp/one-app.yaml
# keep your identity + secrets; only refresh pins:
./scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt /tmp/one-app.yaml
go run ./scripts/validate-do-app-spec.go -strict-digest /tmp/one-app.yaml

# 2) Roll api + worker on the existing App Platform app
doctl apps update <app-id> --spec /tmp/one-app.yaml

# 3) Wait until HTTPS is up, then health:
curl -fsS "$PLATFORM_PUBLIC_URL/healthz"
curl -fsS "$PLATFORM_PUBLIC_URL/readyz"

# 4) Product smoke (seeded suite; deploy-scoped key)
curl -sS -X POST "$PLATFORM_PUBLIC_URL/deploy/v1/tests/runs" \
  -H "Authorization: Bearer $DEPLOY_KEY" \
  -H "Content-Type: application/json" \
  -d '{"suiteApiName":"PlatformSmoke"}'
```

See [product-upgrades.md](./product-upgrades.md). Rollback = previous `image-digests-*.txt` through the same apply → `doctl apps update` sequence.

### Day-2 cloud management (Deploy API)

Host-free primary surface ([deploy-cloud-capability-contract.md](./architecture/deploy-cloud-capability-contract.md)):

```bash
DIGITALOCEAN_API_TOKEN=…          # customer DO personal access token / PAT
DIGITALOCEAN_APP_ID=…             # optional bootstrap bind
DIGITALOCEAN_DATABASE_ID=…        # optional bootstrap bind
```

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/deploy/v1/cloud/status` | Adapter configured? binding? |
| `PUT` | `/deploy/v1/cloud/binding` | Bind this install to app/db resource ids |
| `PATCH` | `/deploy/v1/cloud/app/scale` | Scale api/worker instances or size class |
| `PATCH` | `/deploy/v1/cloud/database/resize` | Resize managed Postgres / HA nodes |
| `POST` | `/deploy/v1/cloud/environments` | Provision peer app + DB + peer row |
| `POST` | `/deploy/v1/cloud/app/redeploy` | **Temporary helper** — digest push to the bound app; product rolls stay `/ops/v1` |

`GET /deploy/v1/environment` advertises `cloudHost`, `capabilities.cloud`, and `capabilities.digitaloceanCloud` when the DO token is set. `/deploy/v1/cloud/digitalocean/*` remains as compatibility aliases. Control IDE Govern uses the host-free routes ([BP-027](adr/030-install-agent-runtime.md)). Manual live smoke: [deploy/digitalocean/LIVE_SMOKE.md](../deploy/digitalocean/LIVE_SMOKE.md).

**Peer environment:** `POST …/environments` spins up a **separate** App Platform app + Managed Postgres (not a second service on the same app), registers a Deploy peer, and bills the customer DO account.

**Network / edge:** App Platform provides HTTPS and platform DDoS mitigation. Use DO trusted sources on Managed DB and App Platform settings.

**Multi-env:** one App Platform app + one Managed DB per env (`dev` / `test` / `prod`), shared `CUSTOMER_ID`, unique `INSTALL_ID`. Marketplace 1-Click publish remains deferred ([BP-028](../backlog/BP-028-digitalocean-marketplace-listing.md)).

---

## Path B — Self-install from image

Same GHCR images as Path A. You own the runtime (Compose or Kubernetes).

### Compose (local / simple)

```bash
cp .env.example .env
docker compose -f deploy/docker-compose.yml up --build
```

API on `:8080`. Kernel migrate + optional core seed on boot when `AUTO_SEED=1`. This file is a **single** install (`INSTALL_ROLE=dev`) for the everyday local loop.

**Two sibling installs (prod + test)** for a customer-rollout lab: [deploy/docker-compose.multi-env.yml](../deploy/docker-compose.multi-env.yml) — shared `CUSTOMER_ID`, unique DBs and ports `:8080` / `:8081`. Scenario cards: [customer-rollout-test-run.md](./customer-rollout-test-run.md).

**Three sibling installs (dev / test / prod)** for the customer-install simulation: [deploy/docker-compose.dev-test-prod.yml](../deploy/docker-compose.dev-test-prod.yml) — ports `:8080` / `:8081` / `:8082`. Runbook: [customer-install-simulation-test-run.md](./customer-install-simulation-test-run.md). Do not run the lab overlays at the same time as this everyday Compose file (port collision).

Without Docker, use the matching runbook’s native multi-DB fallback. Wait for each API `/readyz` before starting its worker (both binaries apply kernel SQL). Product images set `DENO_PATH`; native `go run` needs Deno 2.9.3 on `PATH` in **both** processes.

### Helm on Kubernetes (DO / AWS / Azure / …)

For customers who want VPC/network control or already run Kubernetes. Same chart on **DOKS, EKS, AKS, GKE, on-prem**.

#### 1. Namespace + secrets

```bash
NS=one-prod
kubectl create namespace "$NS"

kubectl -n "$NS" create secret generic one-db \
  --from-literal=DATABASE_URL='postgres://user:pass@host:25060/one?sslmode=require'

kubectl -n "$NS" create secret generic one-api \
  --from-literal=API_KEYS="$(openssl rand -hex 32)+admin" \
  --from-literal=AUTH_JWT_SIGNING_KEY="$(openssl rand -hex 32)" \
  --from-literal=INSTALL_CLAIM_TOKEN="$(openssl rand -hex 24)"
# optional: --from-literal=DEPLOY_SHARE_SECRET="$(openssl rand -hex 32)"
```

#### 2. Pin digests

```bash
API_DIGEST=sha256:…
WORKER_DIGEST=sha256:…
VERSION=X.Y.Z
```

#### 3. Install (DigitalOcean DOKS example)

```bash
helm upgrade --install one ./deploy/helm/one \
  -n "$NS" \
  -f deploy/helm/one/values.yaml \
  -f deploy/helm/one/values-doks.yaml \
  --set image.digest="$API_DIGEST" \
  --set worker.image.digest="$WORKER_DIGEST" \
  --set install.productVersion="$VERSION" \
  --set install.customerId=acme \
  --set install.installId=acme-prod \
  --set install.installRole=prod \
  --set install.platformPublicUrl=https://one.example.com
```

`values-doks.yaml` uses `service.type=LoadBalancer`. For Ingress + cert-manager:

```bash
--set service.type=ClusterIP \
--set ingress.enabled=true \
--set ingress.className=nginx \
--set ingress.hosts[0].host=one.example.com \
--set ingress.hosts[0].paths[0].path=/ \
--set ingress.hosts[0].paths[0].pathType=Prefix
```

#### 4. Other clouds (EKS / AKS / GKE / on-prem)

Same chart and images — configure Ingress/TLS/network policy per cloud.

#### 5. Smoke

```bash
kubectl -n "$NS" rollout status deploy/one-api
curl -fsS "https://one.example.com/healthz"
```

**Upgrade:** `helm upgrade` with new digests + `install.productVersion` ([product-upgrades.md](./product-upgrades.md)).

**Network:** Pod hardening is in the chart; cloud firewall / WAF / network policy are operator-owned on K8s.

---

## Community cloud SDKs

Optional **Path B extensions** for cloud-specific identity, edge, and deploy helpers — **not** a third install product, **not** a second Path A, and **not** product GA. Community-maintained under [`sdk/`](../sdk/README.md) (Apache-2.0; not in product images).

| Cloud | Managed PaaS analog (community) | Power path |
|---|---|---|
| AWS | [Opinionated ECS Fargate profile](../sdk/aws/docs/managed-paas-profile.md) | [ECS Fargate](../sdk/aws/docs/aws-fargate.md) or Helm on EKS |
| Azure / GCP | Stubs — Container Apps / Cloud Run later | Helm on AKS / GKE |

Day-2 cloud ops use **host-free Deploy routes** `/deploy/v1/cloud/*` ([deploy-cloud-capability-contract.md](./architecture/deploy-cloud-capability-contract.md)); product implements DigitalOcean first (`CloudHost`); `/cloud/digitalocean/*` are aliases. Community AWS adapter skeleton: [`sdk/aws/cloudhost`](../sdk/aws/cloudhost/). Do **not** equate AWS Fargate with DigitalOcean App Platform — use **managed PaaS vs power path**.

For AWS power installs, prefer Path B Helm on EKS unless you deliberately opt into the community ECS stack.

---

## Multi-env (dev / test / prod)

| Env | App Platform (Path A) | Kubernetes (Path B) | Postgres HA |
|---|---|---|---|
| dev | Separate app | `values-dev.yaml` | Single node |
| test | Separate app | `values-dev.yaml` | Single node |
| prod | Separate app | `values-doks.yaml` (or cloud Ingress) | Standby / Multi-AZ-style |

Share `CUSTOMER_ID`; unique `INSTALL_ID` / `INSTALL_ROLE`. Multi-env is repo→org validate/deploy ([multi-env-deploy.md](./multi-env-deploy.md)).

## Auth notes

- Default: Majesta One JWT + bootstrap `API_KEYS` (no Cognito required).
- Optional Google/Apple social broker: [ADR-015](./adr/015-idp-agnostic-social-login.md).
- Optional customer OIDC via `OIDC_*`.
- Optional AWS Cognito via community [`sdk/aws`](../sdk/aws/README.md) ([auth-adapters.md](./auth-adapters.md)).

## Security

Report vulnerabilities privately — [SECURITY.md](../SECURITY.md).
