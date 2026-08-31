# Product release CI/CD

How a Majesta One change becomes a published, installable product version. Customer customizations are **not** part of this pipeline; see [customer-customizations.md](./customer-customizations.md). Operator install: [self-host.md](./self-host.md).

## Pipelines

| Workflow | Trigger | Purpose |
|---|---|---|
| `.github/workflows/ci.yml` | Push / PR | **Path-filtered:** Go jobs in parallel (lint, tests+coverage, `one` GOOS/GOARCH compile-only, Docker smoke) gated by a `Go platform` check, and/or Control IDE job (Vitest, build, AppImage smoke, IDE artifact assert). **Gitleaks** on every run. IDE-only changes skip Go. PR Go tests omit `-race` (kept on `main`). Docker image smoke runs on `main` and on PRs that touch Dockerfile / `.dockerignore` / migrations / image-assert. |
| `.github/workflows/release.yml` | Version tag `v*` | Boundary + gitleaks + build versioned api/worker images, image-contents audit, **push to GHCR**, attach digests + static binaries, create GitHub Release |
| `.github/workflows/control-ide-release.yml` | Tag `control-ide-v*` | Gitleaks + private Mac/Windows/Linux installers + IDE artifact assert (not product images) |

Control IDE packaging details: [control-ide-build.md](./control-ide-build.md).

## Actions and Packages storage hygiene

GitHub’s free **Actions and Packages storage** quota (2 GB/month) counts **workflow artifacts**, **Actions caches**, and **GHCR container layers** together. Peak usage during the billing period is what matters — not just retention length.

### What we changed to stay under quota

| Source | Before | Now |
|---|---|---|
| Control IDE CI | Uploaded ~100–200 MB AppImage + coverage dir every run | **No CI uploads** — `dist:linux` + `assert-ide-artifacts.sh` still run; artifacts stay on the runner only |
| Go CI | Uploaded `coverage.out` every run | **Removed** — floor checked in-job |
| Gitleaks | Uploaded SARIF artifact every run (default) | **`GITLEAKS_ENABLE_UPLOAD_ARTIFACT: false`** |
| Go / npm / lint caches | Cached on every PR branch (N copies of ~400MiB) | **Go:** PRs *restore* the `go.sum`-keyed cache saved on `main`; they do not save. golangci-lint restores on PRs and saves on `main`. **npm:** still cached on `main` only |
| Product `v*` release | Attached `docker save` tar.gz duplicates | **Removed** — use GHCR images + `image-digests-*.txt` ([self-host.md](./self-host.md)) |
| Control IDE release | Staging artifacts between matrix jobs | Still uploaded with `retention-days: 3` (required for `download-artifact` → GitHub Release) |
| Caches | Manual purge only | **Weekly** `.github/workflows/actions-storage-hygiene.yml` deletes caches unused for 14 days (`scripts/gh-actions-cache-expire.sh`). `scripts/gh-actions-quota-cleanup.sh --caches-only` still deletes *all* caches in an emergency |

### One-shot cleanup (run locally)

```bash
export GH_REPO=MajestaNet/one
DRY_RUN=1 ./scripts/gh-actions-cache-expire.sh           # drop caches unused >14d
./scripts/gh-actions-quota-cleanup.sh --dry-run
./scripts/gh-actions-quota-cleanup.sh                    # artifacts + caches + 3-day default
./scripts/gh-actions-quota-cleanup.sh --ghcr-prune-untagged  # drop untagged GHCR layers
```

The script:

1. Deletes **all** workflow artifacts (paginated).
2. Clears **all** Actions caches (`gh cache delete --all`). Prefer `./scripts/gh-actions-cache-expire.sh` (14-day unused) unless quota is already exhausted.
3. Sets repo artifact+log retention default to 3 days (`-F days=3` — typed integer).
4. Optionally prunes **untagged** GHCR versions for `one-api` / `one-worker`.

**Manual equivalents**

```bash
gh api --paginate repos/OWNER/REPO/actions/artifacts --jq '.artifacts[].id' \
  | xargs -I {} gh api -X DELETE repos/OWNER/REPO/actions/artifacts/{}
gh cache delete --all -R OWNER/REPO
gh api -X PUT repos/OWNER/REPO/actions/permissions/artifact-and-log-retention -F days=3
```

### GHCR (Packages) storage

Each `v*` tag pushes api + worker images to `ghcr.io/majestanet/*`. Old tags and untagged manifest revisions accumulate. In the GitHub UI: **Packages → one-api / one-worker → Package settings → retention** (enable delete untagged / keep last N versions). Or run `--ghcr-prune-untagged` above.

Release assets on the **GitHub Release** page (static binaries) are separate from GHCR but still count toward overall storage if large — we no longer attach redundant image tarballs.

### If you still hit the cap

1. Run the cleanup script + GHCR prune.
2. Confirm org billing shows which bucket is largest (artifacts vs caches vs packages).
3. Further knobs (trade CI time for storage): set `cache: false` on `release.yml` setup-go, skip `dist:linux` on PRs (only on `main`), raise `MAX_AGE_DAYS` on cache expiry, or reduce agent PR volume / use draft PRs without CI until ready.

```text
feature branch → PR → ci.yml (must green)
       ↓
 merge to main
       ↓
 git tag vX.Y.Z → release.yml
       ↓
 ONE artifact set (tag + image digests):
   ghcr.io/majestanet/one-api:X.Y.Z @ sha256:…
   ghcr.io/majestanet/one-worker:X.Y.Z @ sha256:…
   one-api, one-worker, one-migrate (linux host binaries)
   one (linux/amd64 compat alias) plus one-linux-amd64, one-linux-arm64,
     one-darwin-amd64, one-darwin-arm64, one-windows-amd64.exe
   image-digests-X.Y.Z.txt on the GitHub Release
       ↓
   Self-host / DOKS Helm / other K8s  (pin digests — docs/self-host.md)
   DO Marketplace 1-Click when BP-028 unblocked (same digests)
```

**Release publishes artifacts. It does not roll customer installs or Marketplace listings.** Operators (or a future listing) pin digests. Historical managed-subscription / AWS Marketplace channel language below is **not GA** — prefer [self-host.md](./self-host.md) and [BP-011](../backlog/BP-011-container-marketplace-fargate.md).

Customer-facing docs on `one.majesta.net` are published by a **separate CMS aggregator**, expected to pin the same **`v*`** as GHCR. This repo’s `release.yml` does **not** build or deploy that site. Pointer: [public-docs-site.md](./architecture/public-docs-site.md) ([BP-067](../backlog/BP-067-public-docs-site.md)).

## Versioning

- Semver tags: `v0.1.0`, `v0.2.0`, …
- Link-time inject: `-X github.com/MajestaNet/ide/internal/version.Version=X.Y.Z`
- Runtime / Deploy compatibility: set `PRODUCT_VERSION` on each install to the running product version
- Client wire compatibility (ADR-025 / BP-025): set `API_REVISION_CURRENT` and `API_REVISION_MIN` on each product image; publish a **revision changelog** when bumping `current` or raising `min` (sunset `min` only with ≥ one IDE/product release notice)
- Control IDE / `one` / `sdk/client` pin `One-API-Revision` — product tags and IDE tags stay separate; coupling is manifest + revision window policy
- Do **not** parse the pin from `PRODUCT_VERSION`. Early Majesta One may set `current ≈ product minor` as a convenience in release scripts only.
- Managed `core` package versions advance with product boot migrate ([BP-007](./adr/020-cdm-managed-packages.md)), not via Deploy promote
- Marketplace and managed subscription **must consume the same `vX.Y.Z` tag and image digests** — never diverge on `:latest` or a long-lived product branch

## What a release artifact contains

**Included**

- Go static binaries for `api`, `worker`, `migrate` (linux host / self-host)
- Customer DX CLI `one`: unadorned `one` is linux/amd64 (compat for existing CI samples); also `one-linux-amd64`, `one-linux-arm64`, `one-darwin-amd64`, `one-darwin-arm64`, `one-windows-amd64.exe`. Do not matrix `one-api` / `one-worker` / `one-migrate` for darwin/windows.
- Distroless container images built from `deploy/Dockerfile`
- Kernel SQL under `migrations/` (copied into the image)

**Excluded (explicitly)**

- `.customer-sandbox/**`
- `tools/**`, `scripts/**` (except as CI runners)
- `docs/**`, `backlog/**`
- Any customer metadata exports, fixtures with customer data, or customer Git checkouts

The Dockerfile already limits `COPY` to product paths; `.dockerignore`, `scripts/assert-product-boundary.sh`, and `scripts/assert-image-contents.sh` reinforce that. Optional community AWS Marketplace assets live under [`sdk/aws/deploy/marketplace/`](../sdk/aws/deploy/marketplace/) — not the vendor monorepo.

## Local release smoke

```bash
# Validate like CI
make ci
./scripts/assert-product-boundary.sh

# Build versioned images locally
VERSION=0.2.0
docker build -f deploy/Dockerfile --build-arg CMD=api --build-arg VERSION=$VERSION -t one-api:$VERSION .
docker build -f deploy/Dockerfile --build-arg CMD=worker --build-arg VERSION=$VERSION -t one-worker:$VERSION .
```

## Cutting a release

1. Ensure `main` is green on `ci.yml`.
2. Update release notes / changelog as needed.
3. Tag and push:

```bash
git tag -a v0.2.0 -m "Majesta One v0.2.0"
git push origin v0.2.0
```

4. `release.yml` builds and publishes artifacts (images + binaries + GitHub Release). Record the **image digests** from that run.
5. **Separately** promote that version into each commercial channel (Marketplace listing / self-managed rolls / managed cells). See [Channel promotion](#channel-promotion-marketplace--managed). Customer metadata stays in place across product upgrades.

## Channel promotion (Marketplace + managed)

Co-locating product source, Marketplace packaging, and managed-fleet *reference* TF in this monorepo is intentional ([monorepo.md](./monorepo.md), [ADR-001](./adr/001-dedicated-install.md)). The risk is **not** shared source — it is shared **promotion authority and credentials**.

### Hard rules

| Rule | Why |
|---|---|
| `release.yml` / `v*` tags **only publish** artifacts | A tag must never imply managed-prod or Marketplace subscriber rolls |
| Channel rolls pin **`PRODUCT_VERSION` + image digest** | Marketplace and managed stay on the same bits; no `:latest`, no rebuild-per-channel |
| **No managed-prod credentials** in product CI for PRs or unprotected pushes | Prevents merge → accidental fleet mutation |
| Managed rolls use a **separate** workflow / environment (or ops repo) with required reviewers | Canary → staging cohort → prod; never on tag alone |
| Product stays **trunk + semver tags** | Do **not** create a long-lived `managed-prod` product branch — that invites Marketplace/managed drift |
| `sdk/aws/deploy/managed/` is community ops reference | Not a product listing asset; not product-image contents ([managed-channel.md](../sdk/aws/docs/managed-channel.md)) |

```text
tag vX.Y.Z  →  publish digests D_api, D_worker
                 │
                 ├─► Marketplace: attach digests to listing / ECR fulfillment
                 │     (portal / gated publish; subscribers pull)
                 │
                 └─► Managed: consume same digests
                       canary cell install(s)
                         → approved staging cohort
                         → approved prod cohort
                       (FleetOps starts install-local SSM /ops roll; no secrets in CI)
```

### Same-version parity gate (before calling a version GA on either channel)

1. GitHub Release `vX.Y.Z` exists with published `one-api` / `one-worker` digests.
2. Digests are available to **both** Marketplace fulfillment and managed cell registries (or a shared ECR the vendor mirrors from).
3. Smoke: one Marketplace-shaped stack **and** one managed canary install on those digests (`PRODUCT_VERSION=X.Y.Z`).
4. Only then mark the version eligible for broader Marketplace publish and managed cohort rolls.

### What comes later (not a product fork)

- Protected GitHub Environments (`managed-canary`, `managed-prod`) or a private **ops** repo that only references released digests + per-cell state
- EventBridge / scheduler for Cognito quota metrics already documented under managed; still **not** an auto-prod-roll
- Fleet-wide version inventory ([BP-002](../backlog/BP-002-dedicated-install-fleet-ops.md))

Until that automation exists, managed and Marketplace rolls remain **manual or explicitly approved** after each release.

## Product upgrade vs customer promote

| Change type | Mechanism |
|---|---|
| Kernel, API, worker, managed packages | Product release → image/binary upgrade on each install |
| Customer objects/fields/rules/tests | Metadata API on a source install → Deploy API promote to peer installs |

Never ship a customer’s metadata by embedding it in the product image or “special casing” a customer in `internal/seed`.
