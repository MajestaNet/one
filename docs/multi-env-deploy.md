# Multi-environment deploy (Phase E)

**Start with one Prod.** The default install / Marketplace story is a **single** Prod release — one public API URL for Control IDE ([install-ide-connect-build-plan.md](./architecture/install-ide-connect-build-plan.md)). Multi-env is **opt-in**: add sibling installs only when the customer wants them.

Customers may run **any number** of Majesta One installs under one commercial customer (e.g. `test`, `test-eu`, `staging`, `prod`). There is **no cap** on how many test (or other) instances a customer may run.

**Multi-env change flow is repo → org only:** pack from customer Git, validate and deploy against each install URL from the **same Git SHA**. There is no install→install peer push or inbound artifact promote.

## Identity

| Variable | Meaning |
|---|---|
| `CUSTOMER_ID` | Customer identity shared by **all** installs of that customer |
| `INSTALL_ID` | Unique per environment |
| `INSTALL_ROLE` | Free-form label (`test`, `test-2`, `staging`, `prod`, …) |
| `DEPLOY_PEER_MODE` | Optional legacy label (`customer` / `allowlist`); **does not** gate promotions after inbound promote removal |
| `DEPLOY_SHARE_SECRET` | Optional; when set, local bundles may store an HMAC signature (not used for cross-install promote) |
| `CUSTOMER_REPO_URL` | HTTPS clone URL for the customer Git repo |
| `CUSTOMER_REPO_PROVIDER` | e.g. `codecommit`, `github`, `gitlab`, … |
| `CUSTOMER_REPO_REGION` | Region when applicable (e.g. CodeCommit) |

Isolation between **customers** remains dedicated install deploy (separate accounts / clusters). `CUSTOMER_ID` correlates sibling environments of the same customer for IDE topology (`POST/GET /deploy/v1/peers`).

**Customer Git:** one repository per `CUSTOMER_ID` (not per `INSTALL_ID`). Format: [customer-repo.md](./customer-repo.md).

## Peer registry (IDE / topology)

Register sibling installs so Control IDE can list env switcher targets:

```http
POST /deploy/v1/peers
{ "installId": "acme-test-eu", "installRole": "test", "label": "EU Test", "baseUrl": "https://..." }
```

Peers are **not** a promote channel. Apply always happens via pack + `POST /deploy/v1/promotions` `{ "bundleId" }` on the **connected** install (or `one org deploy`).

## Removed: install→install transfer

| Former API | Status |
|---|---|
| `POST /deploy/v1/bundles/:id/push` | **Removed** |
| `POST /deploy/v1/promotions` with `{ artifact, checksum, signature }` | **Removed** (use `{ bundleId }` only) |

## Topology example (optional siblings)

Default day-0 topology is **only** `acme-prod`. The tree below is what a customer may grow into later — not what Helm / Marketplace provisions by default.

```
CUSTOMER_ID=acme-corp
├── INSTALL_ID=acme-test-us   role=test      (optional)
├── INSTALL_ID=acme-test-eu   role=test-eu   (optional)
├── INSTALL_ID=acme-staging   role=staging   (optional)
└── INSTALL_ID=acme-prod      role=prod      (default)
```

All sibling installs share product upgrades via image rolls (Helm digest upgrade).

## Recommended DX path (repo → org)

Prefer [customer-developer-workflow.md](./customer-developer-workflow.md): pack from customer Git, **`org validate`** then **`org deploy`** against each install URL (test → staging → prod) from the **same Git SHA**. Control IDE Ship mirrors this (**Validate vs org** → **Deploy to org**).

See [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md) and [customer-dx-build-plan.md](./architecture/customer-dx-build-plan.md).

**Business data** is not Deploy-promoted. For ordered multi-object seed/refresh between installs, use external-ID **data packs** applied with Client credentials to each connected org — [external-id-upsert-bulk-build-plan.md](./architecture/external-id-upsert-bulk-build-plan.md) ([BP-041](../backlog/BP-041-record-external-id-upsert-bulk.md)).

### Control IDE when you add siblings

1. Register each sibling via `POST /deploy/v1/peers` (feeds IDE env switcher). `baseUrl` must be a non-loopback origin (HTTPS in production). Loopback URLs are rejected as an SSRF guard — paste the URL in Control IDE **Add environment…** instead.
2. In Control IDE, use **Add environment…** or a peer’s **Connect…** chip (URL prefilled when `baseUrl` is set).
3. Sign in **once per install** — each install has its own JWT issuer. Cross-install “one login unlocks all” is **not** supported; see [install-ide-connect-build-plan.md](./architecture/install-ide-connect-build-plan.md) Phase 3.
4. Multi-env release: switch the env switcher to the target org, checkout the same Git revision, Validate vs org → Deploy to org.

Local two-install Compose lab: [customer-rollout-test-run.md](./customer-rollout-test-run.md) (`deploy/docker-compose.multi-env.yml`). Three-env (dev / test / prod) simulation: [customer-install-simulation-test-run.md](./customer-install-simulation-test-run.md) (`deploy/docker-compose.dev-test-prod.yml`).
