# DigitalOcean App Platform — packaging

Lowest-friction Majesta One install path (no Kubernetes).

| Doc | Purpose |
|---|---|
| [app.yaml](./app.yaml) | Checked-in App Spec (Wave A / [BP-029](../../backlog/BP-029-app-platform-install.md)) |
| [MARKETPLACE_PREP.md](./MARKETPLACE_PREP.md) | Listing prep only — publish deferred ([BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md)) |
| [LIVE_SMOKE.md](./LIVE_SMOKE.md) | Manual live DO smoke (bind / scale / resize / provision) |
| [self-host.md](../../docs/self-host.md) | Operator runbook (Path A) |
| [do-app-platform-deploy-api-build-plan.md](../../docs/architecture/do-app-platform-deploy-api-build-plan.md) | Active plan (Wave A packaging → Wave B Deploy API) |

Prefer **Managed PostgreSQL** attached to the app — do **not** use App Platform’s small “dev database” for production.

## Validate

```bash
go run ./scripts/validate-do-app-spec.go deploy/digitalocean/app.yaml
# After pinning digests on a copy (not required on the checked-in example):
# go run ./scripts/validate-do-app-spec.go -strict-digest /tmp/one-app.yaml
```

## Pin digests from a release

```bash
# Download image-digests-X.Y.Z.txt from the GitHub Release, then:
chmod +x scripts/apply-do-app-digests.sh
./scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt
```

Mapping:

| Digests file key | App Spec field |
|---|---|
| `api_digest=sha256:…` | `services[name=api].image.digest` (+ `tag` → release semver) |
| `worker_digest=sha256:…` | `workers[name=worker].image.digest` |
| filename `image-digests-X.Y.Z.txt` | `PRODUCT_VERSION` env + image `tag`; set `API_REVISION_CURRENT` / `API_REVISION_MIN` for the wire window |

Never float on `:latest`.

## Deploy (near-term)

```bash
# 1) Create Managed PostgreSQL (HA standby on prod)
# 2) Edit app.yaml: CUSTOMER_ID, INSTALL_*, PLATFORM_PUBLIC_URL, digests
# 3) Create app (secrets can also be set in DO UI after create)
doctl apps create --spec deploy/digitalocean/app.yaml

# Set secrets in DO App settings if not injected yet:
#   DATABASE_URL, API_KEYS (…+admin), AUTH_JWT_SIGNING_KEY,
#   INSTALL_CLAIM_TOKEN, WEBHOOK_ENCRYPTION_KEY, DEPLOY_SHARE_SECRET
```

Optional day-2 management: set `DIGITALOCEAN_API_TOKEN` (+ bind app/db ids) so Deploy API `/deploy/v1/cloud/*` can scale/provision ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)).

## Upgrade

```bash
cp deploy/digitalocean/app.yaml /tmp/one-app.yaml
./scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt /tmp/one-app.yaml
go run ./scripts/validate-do-app-spec.go -strict-digest /tmp/one-app.yaml
doctl apps update <app-id> --spec /tmp/one-app.yaml
curl -fsS "$PLATFORM_PUBLIC_URL/healthz"
curl -fsS "$PLATFORM_PUBLIC_URL/readyz"
curl -sS -X POST "$PLATFORM_PUBLIC_URL/deploy/v1/tests/runs" \
  -H "Authorization: Bearer $DEPLOY_KEY" \
  -H "Content-Type: application/json" \
  -d '{"suiteApiName":"PlatformSmoke"}'
```

Product rolls stay **`/ops/v1`**. `POST /deploy/v1/cloud/app/redeploy` is a **temporary helper**. See [product-upgrades.md](../../docs/product-upgrades.md) and [self-host.md](../../docs/self-host.md).

## IDE UI

Backlog only ([BP-027](../../docs/adr/030-install-agent-runtime.md)) — IDE will call Deploy API, not DO directly.
