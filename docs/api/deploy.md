# Deploy API (`/deploy/v1`)

Treat **customer implementation** as a releasable artifact for **this customer’s** installs: pack, validate, test, and apply customer-owned metadata onto the connected install.

**Scope:** `deploy`. Cloud bind / scale / provision also require **admin**.

**Does not:** promote managed `core` / module internals; copy business records by default; roll the product image (that is [Ops](./ops.md)); push artifacts install→install (repo→org only).

There is no flat `/v1` alias for Deploy. Typical DX: `one org validate` / `one org deploy` against each install URL from the same Git SHA — [customer-repo.md](../customer-repo.md) · [multi-env-deploy.md](../multi-env-deploy.md).

## Environment

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` | `/deploy/v1/environment` | Product version, `customerId`, install id, role, peer mode, `cloudHost`, capabilities, customer repo URL | Echo cloud credentials |

## Packages, bundles, promotions

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `POST` | `/deploy/v1/packages/pack` | Upload a `one/v1` zip/tar → create a bundle | Pack managed seed |
| `POST` | `/deploy/v1/packages/validate-local` | Dry-run the connected install (`one org validate`) | Apply |
| `GET` | `/deploy/v1/packages/export` | Current customer snapshot (+ tests + managed **baseline**) as `one/v1` zip | |
| `POST` | `/deploy/v1/packages/initialize-repo` | Admin+deploy: seed remote `main` from this install (`CUSTOMER_REPO_URL`) | Overwrite an unrelated Git remote |
| `POST` | `/deploy/v1/bundles` | Create a bundle from the current customer snapshot (or uploaded package) | Include managed packages, records, API keys, or audit history |
| `GET` | `/deploy/v1/bundles` · `/bundles/{id}` | List / get | |
| `GET` | `/deploy/v1/bundles/{id}/artifact` | Download the signed artifact | |
| `POST` | `/deploy/v1/bundles/{id}/validate` | Validate against **this** install | Apply |
| `POST` | `/deploy/v1/promotions` | Apply `{ bundleId }` to this install | Accept an inbound `{ artifact }` from a peer |
| `GET` | `/deploy/v1/promotions/{id}` | Status, logs, rollback marker | |
| `GET` | `/deploy/v1/work/{jobId}` | Async Deploy job status | |

**Safety:** refuse apply if target `productVersion` is outside the bundle range; refuse clobber/delete of managed apiNames; transactional apply per artifact group; Deploy keys are environment-bound.

## Customer tests

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `POST` | `/deploy/v1/tests` | Register/update a customer test suite | Run product `PlatformSmoke` as a customer suite |
| `GET` | `/deploy/v1/tests` · `/tests/{apiName}` | List / get definitions | |
| `POST` | `/deploy/v1/tests/runs` | Start a run (worker job) | |
| `GET` | `/deploy/v1/tests/runs` · `/tests/runs/{id}` | List / results | |

CI example: [ci-customer-tests.md](../ci-customer-tests.md). Product image rolls reuse the same runner for `PlatformSmoke` (and optional `PostUpgradeSmoke`) — [product-upgrades.md](../product-upgrades.md).

## Peers

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` `POST` | `/deploy/v1/peers` | Registry of sibling installs (topology) | Push a bundle to a peer |

## Cloud (day-2 on this install)

Host-agnostic verbs. DigitalOcean is the product adapter; `/cloud/digitalocean/*` remains a compatibility alias (legacy `appId` / size slugs accepted).

| Method | Path | Admin? | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/deploy/v1/cloud/status` | | Adapter configured? binding; reachability | Echo credentials |
| `PUT` | `/deploy/v1/cloud/binding` | yes | Bind opaque `appResourceId` / `databaseResourceId` | |
| `GET` | `/deploy/v1/cloud/app` | | Live app summary for the bound app | |
| `PATCH` | `/deploy/v1/cloud/app/scale` | yes | Scale api/worker instances or `sizeClass` | |
| `PATCH` | `/deploy/v1/cloud/database/resize` | yes | Resize managed Postgres size class / `numNodes` | |
| `POST` | `/deploy/v1/cloud/environments` | yes | Provision a peer app + DB + peer row (shared `CUSTOMER_ID`, new `INSTALL_ID`; unique `installId`, `apiKeys`, `authJwtSigningKey`) | Fork this product repo |
| `GET` | `/deploy/v1/cloud/environments` | | Peers + provision audit runs | |
| `POST` | `/deploy/v1/cloud/app/redeploy` | yes | Temporary digest redeploy helper | Replace [Ops](./ops.md) long-term |
| `GET` `PUT` | `/deploy/v1/cloud/inference` | PUT yes | Native inference binding when the adapter supports it | BYO provider rows (Metadata) |

## Related

- [API families overview](../api-families.md) · [Metadata](./metadata.md) · [Ops](./ops.md)
- [Multi-env deploy](../multi-env-deploy.md) · [Customer repo](../customer-repo.md) · [Self-host](../self-host.md)
