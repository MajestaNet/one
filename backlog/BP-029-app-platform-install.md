# BP-029: DigitalOcean App Platform install path (default low-friction)

- **Severity:** High
- **Status:** Partially mitigated
- **Area:** `deploy/digitalocean/`, `docs/self-host.md`
- **Plan:** [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md) Wave A · strategy [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md)
- **Remainder (agentic):** [09-bp-029-030-011-002-install-distro.md](../docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md)
- **Related:** [BP-011](./BP-011-container-marketplace-fargate.md), [BP-028](./BP-028-digitalocean-marketplace-listing.md) (Marketplace publish deferred), [BP-030](./BP-030-deploy-api-digitalocean-apps.md) (Deploy API day-2)

## Problem

Many target customers will not operate Kubernetes. Helm/DOKS is powerful but high friction. Without a first-class **App Platform** path (App Spec + Managed PostgreSQL + GHCR digests), the “default” install story forces cluster ops or waits on a Marketplace vendor account.

## Why it matters

Product strategy: **App Platform = default lowest-friction customer path**; Helm/K8s remains for DO/AWS/Azure network-style deploys. Packaging must be **1-Click / Marketplace-ready** (validated App Spec, digest pins, secrets checklist) even while [BP-028](./BP-028-digitalocean-marketplace-listing.md) publish stays deferred.

## What shipped (Wave A)

1. Checked-in **`deploy/digitalocean/app.yaml`** (api + worker; Managed Postgres guidance)
2. Validator `scripts/validate-do-app-spec.go` + CI step in `.github/workflows/ci.yml`
3. Digest apply helper `scripts/apply-do-app-digests.sh` + README mapping from `image-digests-*.txt`
4. Operator runbook in [self-host.md](../docs/self-host.md) Option A (`doctl apps create --spec`)
5. Marketplace **prep notes** only — [MARKETPLACE_PREP.md](../deploy/digitalocean/MARKETPLACE_PREP.md) (no Vendor Portal submit)

## What shipped (packaging remainder)

1. App Spec SECRET `WEBHOOK_ENCRYPTION_KEY`; comment-only OTEL + `AUTH_LOGIN_PROVIDERS` / Google-Apple secrets (example identity placeholders; commented digest pins)
2. Validator requires api `PLATFORM_PUBLIC_URL` and `API_KEYS` / `AUTH_JWT_SIGNING_KEY` / `INSTALL_CLAIM_TOKEN` as `type: SECRET`; `-strict-digest` for operator copies; table-driven tests
3. `apply-do-app-digests.sh` rewrites live digest lines and `tag` / `PRODUCT_VERSION` from `image-digests-X.Y.Z.txt` (not only `0.1.0` / `REPLACE_WITH_*`); fixture test
4. CI: validate example spec; apply fixture + validate + `-strict-digest`
5. Path A upgrade copy-paste in [self-host.md](../docs/self-host.md) / [product-upgrades.md](../docs/product-upgrades.md) (digest file → apply → `doctl apps update` → healthz/readyz → `PlatformSmoke`); Deploy `POST /deploy/v1/cloud/app/redeploy` labeled temporary helper

## Remaining

- Live Marketplace / Vendor Portal publish ([BP-028](./BP-028-digitalocean-marketplace-listing.md))
- End-to-end smoke on a real DO team account after first `v*` digests are published for operators (operator-run; [LIVE_SMOKE.md](../deploy/digitalocean/LIVE_SMOKE.md) — do not mark executed from CI)

## Explicit non-goals (this item)

- Live Marketplace / Vendor Portal publish ([BP-028](./BP-028-digitalocean-marketplace-listing.md))
- IDE OAuth provisioner ([BP-027](../docs/adr/030-install-agent-runtime.md))
- Replacing the Helm chart / K8s enhancement work
- Droplet 1-Click

## Related

- [do-app-platform-deploy-api-build-plan.md](../docs/architecture/do-app-platform-deploy-api-build-plan.md)
- [digitalocean-distribution-build-plan.md](../docs/architecture/digitalocean-distribution-build-plan.md)
- DO [App Spec](https://docs.digitalocean.com/products/app-platform/reference/app-spec/) · [Apps API create](https://docs.digitalocean.com/products/app-platform/how-to/create-apps/)
