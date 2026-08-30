# Operations

## Dedicated install

1. Provision Postgres 16+ (Managed PostgreSQL on DO for Path A; RDS/external for Path B).
2. Set `DATABASE_URL`, bootstrap `API_KEYS` (scopes `key:client+metadata+deploy` and optional `+admin`), `APP_ENV=production`, JWT signing material when BP-013 ships (prefer Majesta One JWT over transitional `OIDC_*`), `FEATURE_FLAGS`, `CUSTOMER_ID`, `INSTALL_ID` / `INSTALL_ROLE` / `PRODUCT_VERSION`, `API_REVISION_CURRENT` / `API_REVISION_MIN`, optional `DEPLOY_PEER_MODE` / `DEPLOY_SHARE_SECRET`, `REQUEST_BODY_LIMIT` / `RATE_LIMIT_PER_MINUTE`. See [security.md](./security.md) and [ADR-006](./adr/006-jwt-auth.md).
3. Run kernel migrations: `make migrate` / `go run ./cmd/migrate` (API also auto-applies kernel DDL on boot).
4. Deploy API + worker ([self-host.md](./self-host.md)):
   - **Path A (default):** DigitalOcean App Platform — [deploy/digitalocean/](../deploy/digitalocean/), [ADR-005](./adr/005-go-runtime.md).
   - **Path B:** Compose (`deploy/docker-compose.yml`) or Helm (`deploy/helm/one`) on DOKS / EKS / AKS / GKE.
   - **Optional community AWS:** ECS Fargate under [`sdk/aws/deploy/ecs/`](../sdk/aws/deploy/ecs/) — not product GA.

Production config validation requires `DATABASE_URL` and rejects development API keys (`dev-*`). Both API and worker handle `SIGTERM`/`SIGINT` for graceful shutdown.

API families: `/client/v1` (data), `/metadata/v1` (model), `/deploy/v1` (customer promote), `/ops/v1` (product image upgrades). Flat `/v1` remains a deprecated alias for Client/Metadata only. A customer may run many installs under one `CUSTOMER_ID`; see [multi-env-deploy.md](./multi-env-deploy.md).

Helm/App Platform defaults support multiple API replicas: metadata cache coherence uses the DB `metadata_cache_epoch`. Workers should use unique worker IDs with `FOR UPDATE SKIP LOCKED` job/outbox claims.

## Upgrades

Product image rolls are **not** Deploy promotions. Prefer the guided path in [product-upgrades.md](./product-upgrades.md) ([ADR-007](./adr/007-platform-ops-upgrades.md)):

1. Confirm target image digests / `PRODUCT_VERSION` via App Spec update, `helm upgrade`, or (community AWS) SSM / **`POST /ops/v1/upgrades`** (scope `ops` + admin).
2. Roll services while preserving capacity. Prefer forward-compatible kernel DDL so old and new tasks/pods can coexist briefly.
3. Kernel migrations apply on boot (idempotent).
4. On API restart with `AUTO_SEED=1` (default), **managed package migrate** runs: `core` (Account, Contact) sync; additive only — customer-owned apiName collisions fail loudly.
5. Gate: `/healthz` + `/readyz`, then Deploy suites **`PlatformSmoke`** (product) and optional customer **`PostUpgradeSmoke`**.
6. Customer custom (customer) fields never require Majesta One DDL and are never overwritten by package migrate.

Manual fallback for community ECS: `terraform apply` with new image digests + `product_version` — see [sdk/aws/docs/marketplace.md](../sdk/aws/docs/marketplace.md).

## Observability

- Structured JSON logs via `log/slog` (`LOG_LEVEL`, request access lines with `x-request-id`).
- Optional OpenTelemetry OTLP traces/metrics when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (see [BP-008](../backlog/BP-008-production-packaging.md), [outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md)). Resource attrs include `PRODUCT_VERSION`, `CUSTOMER_ID`, `INSTALL_ID`. No-op when unset. Logs stay stdout JSON unless `OTEL_LOGS_EXPORTER=otlp` (still keeps stdout). The OTEL log handler drops `authorization` / token / ciphertext / cookie keys. No collector sidecar.

## Packaging pointers

- **Path A:** App Platform Spec — `deploy/digitalocean/`
- **Path B:** Compose — `deploy/docker-compose.yml`; Helm — `deploy/helm/one`
- **Community AWS (optional):** [`sdk/aws/`](../sdk/aws/README.md) — [aws-fargate.md](../sdk/aws/docs/aws-fargate.md), [marketplace.md](../sdk/aws/docs/marketplace.md)
- SKU/feature flags: `FEATURE_FLAGS` + `feature_flags` table
- Security: [security.md](./security.md)
