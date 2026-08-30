# Product upgrades

How a Majesta One **product** version lands on a customer install (Path A App Platform, Path B Compose/Helm, or optional community ECS). Customer metadata promote is separate — see [multi-env-deploy.md](./multi-env-deploy.md) and [api-families.md](./api-families.md). Operator guide: [self-host.md](./self-host.md).

Canonical decision: [ADR-007](./adr/007-platform-ops-upgrades.md).

## Product vs customer

| Change | Mechanism |
|---|---|
| Kernel DDL, API/worker binaries, managed `core` package | New container image digests → App Spec / Compose / Helm (or community ECS task definition) roll; set matching `PRODUCT_VERSION` **and** `API_REVISION_CURRENT` / `API_REVISION_MIN` |
| Customer objects/fields/rules/tests between installs | `/deploy/v1` bundles + promotions |

Never ship customer metadata inside the product image. Never use `/deploy/v1/promotions` to roll Majesta One images.

## Path A — App Platform (default)

Operator-native roll until an Ops App Platform roller exists. Product confirm/rollback stay **`/ops/v1/upgrades`** ([ADR-007](./adr/007-platform-ops-upgrades.md)). `POST /deploy/v1/cloud/app/redeploy` is a **temporary helper** that pushes digests to the **bound** app only — do not treat it as the product upgrade API.

```bash
# 1) Digest file from the GitHub Release
cp deploy/digitalocean/app.yaml /tmp/one-app.yaml
./scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt /tmp/one-app.yaml
go run ./scripts/validate-do-app-spec.go -strict-digest /tmp/one-app.yaml

# 2) Roll
doctl apps update <app-id> --spec /tmp/one-app.yaml

# 3) Health
curl -fsS "$PLATFORM_PUBLIC_URL/healthz"
curl -fsS "$PLATFORM_PUBLIC_URL/readyz"

# 4) PlatformSmoke
curl -sS -X POST "$PLATFORM_PUBLIC_URL/deploy/v1/tests/runs" \
  -H "Authorization: Bearer $DEPLOY_KEY" \
  -H "Content-Type: application/json" \
  -d '{"suiteApiName":"PlatformSmoke"}'
```

`apply-do-app-digests.sh` sets image `tag` and `PRODUCT_VERSION` from the filename semver. Set `API_REVISION_CURRENT` / `API_REVISION_MIN` for the wire window when the release notes ask for it. Same boot-migrate rules as Helm. Rollback = previous digest file through the same sequence.

## Path B — Helm / Compose

1. Read digests from the GitHub Release `image-digests-X.Y.Z.txt`.
2. `helm upgrade` with `--set image.digest=…` / `--set worker.image.digest=…` and `--set install.productVersion=X.Y.Z` plus `--set install.apiRevisionCurrent=…` / `--set install.apiRevisionMin=…` (or bump Compose image pins).
3. API pods/containers apply kernel migrations on boot. Prefer additive migrations so old and new replicas can coexist briefly.

## Optional community AWS ECS notes

The community stack ([`sdk/aws/deploy/ecs/`](../sdk/aws/deploy/ecs/)) places API tasks in **two AZs** behind an ALB, with `desired_count ≥ 2` and rolling deploy percents (`minimumHealthyPercent=50`, `maximumPercent=200`). That supports **zero-downtime rolling replace** when health checks pass. **Not product GA.**

**Do not** treat “upgrade AZ-A, then AZ-B” as the canary model:

- ECS schedules **tasks**, not AZ slices, as the deployment unit.
- All tasks share **one RDS**. Kernel migrations and managed-package migrate run **on boot** against that DB.
- A canary therefore validates **new binary + shared schema**, not an isolated AZ database.

### Admin confirm UX (community AWS)

#### Phase 1 — SSM Automation (AWS Console)

Ship with the community Terraform stack (`sdk/aws/deploy/ecs/upgrade_automation.tf`):

1. AWS admin opens **Systems Manager → Automation**.
2. Runs `One-ProductUpgrade` with target `api_image`, `worker_image`, `product_version`.
3. Document registers task definitions, updates ECS services, waits for stability, hits `/healthz` + `/readyz`, runs Deploy test suites (`PlatformSmoke`, and `PostUpgradeSmoke` when present), and rolls back task definitions on failure.

IAM is scoped to that install’s cluster/services/task roles. No Majesta One SaaS control plane.

#### Phase 2 — Ops API (`/ops/v1/upgrades`)

Install-local API (scope `ops`; confirm/rollback also require admin):

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/ops/v1/upgrades/available` | Current `PRODUCT_VERSION` + configured ECS targets / suggested next version |
| `POST` | `/ops/v1/upgrades` | Confirm upgrade (target images + version) → create run, drive ECS when configured |
| `GET` | `/ops/v1/upgrades` | List recent runs |
| `GET` | `/ops/v1/upgrades/{id}` | Status, ECS revision refs, test run ids, errors |
| `POST` | `/ops/v1/upgrades/{id}/rollback` | Force previous task definition revision |

When ECS env vars are unset (local/Compose/App Platform), the API still records runs and executes the **test gate** so CI and dry environments can exercise the flow.

## Shared-DB migration rules

1. Prefer **additive, forward-compatible** kernel migrations so old and new tasks can coexist briefly during a roll.
2. Avoid irreversible destructive DDL in the same release that first ships the code that requires it.
3. Rollback = previous revision / previous digests. Restore DB from snapshot only if a migration is incompatible (rare).

## Post-roll test gate

After the service is stable (or immediately in local mode):

1. `GET /healthz`, `GET /readyz`
2. Product suite **`PlatformSmoke`** (seeded, `ownership=managed`) via `POST /deploy/v1/tests/runs`
3. Optional customer suite **`PostUpgradeSmoke`** if registered on the install
4. Failure → orchestrator rolls back and surfaces the test run JSON / Automation failure output

Customers should keep `PostUpgradeSmoke` green on prod before relying on automated product rolls.

## ECS circuit breaker (community)

Reference services enable:

```hcl
deployment_circuit_breaker {
  enable   = true
  rollback = true
}
```

Failed unhealthy rolls revert without waiting for an operator when ECS detects the deployment is broken. Automation / Ops API still run **application** smoke and customer tests after ECS reports stable — circuit breaker alone does not replace those gates.

## Related

- [ops.md](./ops.md) — day-2 operations
- [sdk/aws/docs/marketplace.md](../sdk/aws/docs/marketplace.md) — optional AWS Marketplace upgrade notes
- [ci-customer-tests.md](./ci-customer-tests.md) — Deploy test step types
- [sdk/aws/docs/aws-fargate.md](../sdk/aws/docs/aws-fargate.md) — community topology
- [BP-002](../backlog/BP-002-dedicated-install-fleet-ops.md)
