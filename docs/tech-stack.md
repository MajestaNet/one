# Confirmed tech stack

Canonical stack for Majesta One as of the current platform foundation. Prefer this over informal chat history when choosing libraries or patterns.

## Platform

| Layer | Choice | Notes |
|---|---|---|
| Language (runtime) | **Go 1.25+** | Platform API + worker; required by the current security-patched dependency graph; see [ADR-005](./adr/005-go-runtime.md) |
| Module | `github.com/MajestaNet/ide` | `cmd/api`, `cmd/worker`, `cmd/migrate`, `internal/*` |
| Monorepo layout | `cmd/*`, `internal/*`, `migrations/`, `deploy/*`, `sdk/*`, `tools/*`, `scripts/*` | OSS product plane + optional/frozen IDE + community `sdk/`; see [monorepo.md](./monorepo.md) · [ADR-030](./adr/030-install-agent-runtime.md) |

## API & validation

| Layer | Choice | Notes |
|---|---|---|
| HTTP framework | `net/http` + stdlib mux | REST under `/client/v1`, `/metadata/v1`, `/deploy/v1`, `/ops/v1` |
| Schema / validation | Go structs + manual validation | Query AST in `internal/dataengine` |
| Rule expressions | JSONLogic (`diegoholiveira/jsonlogic`) | Metadata validation rules |
| Deploy engine | Go `internal/deploy` | Customer bundles; repo→org validate/apply; initialize-repo via go-git (`internal/deploy/gitremote`) |
| Customer Git remote seed | `github.com/go-git/go-git/v5` | Pure-Go only — no OS `git` in product images ([ADR-012](./adr/012-customer-repo-and-control-ide.md), [BP-031](../backlog/BP-031-customer-repo-init-sync.md)) |
| Ops upgrades | Go `internal/ops` + AWS SDK ECS | Product image rolls (`/ops/v1`); ADR-007 |
| API key scopes | `key`, `key+admin`, or `key:client+metadata+deploy+ops[+admin]` in `API_KEYS` | Family scopes enforced; `+admin` is explicit privilege; keys become bootstrap-only (ADR-006); `ops` = product upgrades (ADR-007) |
| AuthN | Majesta One JWT + **install claim / password** + **customer SSO** (ADR-015 / BP-037); optional Google/Apple; optional OIDC env exchange; optional Cognito sync | Claim + DB install auth; `golang.org/x/oauth2` + `keyfunc`/`golang-jwt`; Cognito not product default |
| HTTP limits | `REQUEST_BODY_LIMIT`, `RATE_LIMIT_PER_MINUTE`, `ADMISSION_CLIENT_RPM_SHARE` | Family admission lanes in `internal/httpapi` (Client reserved share; metadata/deploy/ops share the remainder). `/auth/v1` uses `AUTH_TOKEN_RATE_LIMIT_PER_MINUTE` only. |
| Deploy admission | `DEPLOY_SYNC_MAX_FILES` / `DEPLOY_SYNC_MAX_BYTES`, `DEPLOY_QUEUE_MAX`, `JOB_SLOTS_DEPLOY` | Tiny repo→org validate/apply stays sync; larger work returns `202` + `jobId` and runs as `deploy.validate` / `deploy.apply`. Queue full → `429 DEPLOY_BUSY`. |
| Install exposure | `EXPOSURE_RECONCILE`, `WAF_*` | Metadata `/install/exposure` → Memory roller (product default); AWS WAFv2 via community [`sdk/aws`](../sdk/aws/README.md) when opted in |

## Data

| Layer | Choice | Notes |
|---|---|---|
| Database | PostgreSQL 16+ | One DB per customer install |
| Migrations | Go pgx migrate | Kernel DDL in `migrations/`; applied on boot via `one_schema_migrations` |
| Driver | pgx v5 | |
| Flexible store | `records.data` JSONB + LIST partitions by `object_api_name` | Custom fields = metadata, not migrations; CRM-scale shared parent (ADR-013 Tier C) |
| High-volume store | `records_hv` (LIST/RANGE) | `storage_mode=high_volume` — see [ADR-013](./adr/013-high-volume-flexible-storage.md); no product Message object ([ADR-032](./adr/032-retire-messages-polymorphic-lookup.md)) |
| Query engine | SQL-native planner (`internal/dataengine`) | Filters/sorts/joins/keyset in Postgres; HV + flexible guardrails |
| Field projections | Partial expression indexes | Driven by metadata `indexed` + `field_projections` (on `records` or `records_hv`); worker uses `CREATE INDEX CONCURRENTLY` |
| Pool sizing | `DB_MAX_CONNS` / `DB_MIN_CONNS` | Defaults 10 / 1 per process (`internal/db`) |
| Retention | `RETENTION_*_DAYS` | jobs/outbox/audit_log purge via worker (hard-delete records; no soft-delete) |
| Test DB | PostgreSQL (`DATABASE_URL`) | Integration suites skip when unset |

## Async & events

| Layer | Choice | Notes |
|---|---|---|
| Jobs | Postgres `jobs` table (poll loop) | `FOR UPDATE SKIP LOCKED` + lease reclaim |
| Events | Transactional outbox (`outbox_events`) | Same claim/lease pattern; `webhook_deliveries` for idempotent POSTs; system intents `install.claimed` / `principal.*` ([BP-038](../backlog/BP-038-no-product-mailer-byo-alerts.md)) — **no product mailer** |
| Metadata cache | DB epoch token + short TTL | `metadata_cache_epoch` keeps multi-replica API describe coherent |
| Customer code automations | Deno **2.9.3** guest (ADR-014) | Frozen SDK + unit harness; Deploy steps `automationUnitPass` / `automationContract`; `DEPLOY_REQUIRED_TEST_SUITES` promote gate; `invokeAction` for platform verbs ([ADR-029](./adr/029-platform-actions.md)); see [automation-sdk.md](./automation-sdk.md) |
| Platform actions | Go `internal/actions` | Client `GET/POST /client/v1/actions/{apiName}`; package-gated; first verb `lead.convert` ([BP-061](../backlog/BP-061-platform-actions.md)) |

## Auth (current vs planned)

| Concern | Current | Planned |
|---|---|---|
| AuthN | Majesta One JWT + social broker (Google/Apple); optional `OIDC_*` exchange; optional `IDENTITY_SYNC=cognito` | Majesta One JWT = API bearer; social = OOTB human login ([ADR-015](./adr/015-idp-agnostic-social-login.md)); Cognito optional AWS adapter |
| AuthZ | Permission sets (object CRUD); admin via `+admin`; Roles → scopes; deny-by-default FLS; sharing (ADR-016); kernel User + `users.data` + SCIM UserCustom ([BP-058](../backlog/BP-058-user-identity-extension.md) mitigated) | — |
| API scopes | `client` \| `metadata` \| `deploy` \| `ops` on keys **and** JWT claims/groups | Same families; scopes assigned via Roles (ADR-004; BP-010 mitigated; ADR-007 for `ops`) |

## Deploy & ops

| Layer | Choice | Notes |
|---|---|---|
| Path A (default) | **DigitalOcean App Platform** | **Only** product managed PaaS path + Managed PostgreSQL; [BP-029](../backlog/BP-029-app-platform-install.md), [self-host.md](./self-host.md) |
| Path B — Compose | Docker Compose | `deploy/docker-compose.yml` (local / simple) |
| Path B — Kubernetes | Helm chart | `deploy/helm/one` — DOKS / EKS / AKS / GKE / on-prem |
| Marketplace | DO App Platform–first (+ optional K8s 1-Click) | Publish **deferred** — [BP-028](../backlog/BP-028-digitalocean-marketplace-listing.md). Plan: [digitalocean-distribution-build-plan.md](./architecture/digitalocean-distribution-build-plan.md) |
| Deploy cloud day-2 | Host-agnostic verbs; DO adapter in product | [deploy-cloud-capability-contract.md](./architecture/deploy-cloud-capability-contract.md); routes `/deploy/v1/cloud/digitalocean/*` ([BP-030](../backlog/BP-030-deploy-api-digitalocean-apps.md)) |
| Community cloud SDKs | `sdk/aws`, `sdk/azure`, `sdk/gcp` | Optional Path B extensions; **not** product GA; **not** a second Path A ([sdk/README.md](../sdk/README.md)) |
| Optional AWS community | ECS Fargate (**power**) + managed PaaS profile docs | [`sdk/aws/deploy/ecs/`](../sdk/aws/deploy/ecs/); [managed-paas-profile.md](../sdk/aws/docs/managed-paas-profile.md) — opinionated ECS Fargate api+worker; not center of gravity |
| Human / machine AuthN | Majesta One JWT; install claim + password; customer SSO; optional Google/Apple; API keys break-glass | External IdP via install auth / `/auth/v1/token/exchange` (ADR-015 / BP-037) |
| Install identity | `CUSTOMER_ID`, `INSTALL_ID`, `INSTALL_ROLE` | N envs per customer |
| Peer registry | `POST/GET /deploy/v1/peers` | IDE env topology; not a promote channel |

| Observability | Structured JSON logs (`log/slog`) + OpenTelemetry OTLP traces/metrics (`internal/otel`); optional logs via `otlploghttp` + `otelslog` | `OTEL_EXPORTER_OTLP_ENDPOINT` optional; `OTEL_LOGS_EXPORTER` defaults to **none** (set `otlp` to fan-out logs; stdout JSON always remains). See [BP-008](../backlog/BP-008-production-packaging.md), [outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md) |
| Process runner | **Go static binary** | Distroless image |
| Environment | `APP_ENV` (legacy `NODE_ENV` / `ENV` still accepted) | `production` enforces DB + non-dev API keys |

## Tooling

| Layer | Choice | Notes |
|---|---|---|
| Tests | `go test` | `make test`, `make cover`, `make ci`; shared harness `internal/testutil`; PostgreSQL-backed aggregate statement coverage has a 35% CI regression floor |
| CLI credentials | OS keychain (`zalando/go-keyring`) + file `0600` fallback | `cmd/one` only (`ONE_CREDENTIAL_STORE=auto\|file\|keychain`; [BP-048](../backlog/BP-048-one-cli.md) / [BP-064](../backlog/BP-064-install-agent-runtime.md)). Direct `go.mod` require, so `go run ./cmd/migrate` still downloads it. `al.essio.dev/pkg/shellescape` is an indirect Linux helper of go-keyring. |
| Lint | golangci-lint | `.golangci.yml` |
| CI | GitHub Actions | `.github/workflows/ci.yml` (validate) + `release.yml` (tag publish) |
| Cloud agents | `.cursor/environment.json` + Dockerfile | Go 1.25 image |
| License | Entire repository **Apache-2.0** (including Control IDE) | Root `LICENSE` / `NOTICE` (covers Control IDE) |

## Vendor plane — Control IDE

Not product runtime. Lives under `tools/control-ide` only ([ADR-012](./adr/012-customer-repo-and-control-ide.md), [control-ide-build.md](./control-ide-build.md)).

| Layer | Choice | Notes |
|---|---|---|
| Shell | Electron 43 | Desktop installers via electron-builder + `@electron/fuses` |
| UI | React 19 + **own CSS tokens** + `ui/` primitives | Monaco for YAML; self-hosted IBM Plex; no third-party design-system kit — [control-ide-design.md](./control-ide-design.md), [BP-016](./adr/030-install-agent-runtime.md) |
| Chat UI | `@assistant-ui/react` + Majesta One external-store runtime | Headless Thread/Composer; runs stay on Go `/client/v1/agents/runs` |
| Operate graph layout | `@dagrejs/dagre` + owned SVG | Explorer tool only — no cytoscape/vis in Operate v1 |
| Run Tool renderers | `@base-ui/react` · `@tanstack/react-table` + `@tanstack/react-virtual` · `@dnd-kit/*` · `@xyflow/react` · `react-hook-form` + `zod` (+ `@hookform/resolvers`) · `recharts` · `react-markdown` | Headless engines painted with Majesta One tokens under `tools/control-ide/src/renderer/run/` — [ADR-021](./adr/021-run-mode-toolspec.md) |
| Bundler | Vite 8 + `vite-plugin-electron` | Rolldown/Oxc; browser `npm run dev` is vendor-local only |
| Language | TypeScript 5.7 | Renderer + main/preload |
| Tests | Vitest 4 + Testing Library + jsdom 30 | `make test-ide`; live-API: `make test-ide-integration`; Node ≥22.22.2 |
| Updates | `electron-updater` (generic feed) | Gated on packaged app + `UPDATE_FEED_URL`; private CDN E2E frozen — [ADR-030](./adr/030-install-agent-runtime.md) |
| Auth transport | Majesta One JWT Bearer | Effective AuthZ on the install; session via `safeStorage` |

## Vendor plane — Majesta One MCP scaffold

Not product runtime. Lives under `tools/one-mcp` ([customer-connect.md](./customer-connect.md), [ADR-010](./adr/010-customer-agentic-platform.md)).

| Layer | Choice | Notes |
|---|---|---|
| MCP SDK | `@modelcontextprotocol/sdk` | Stdio server; optional proxy to product `POST /mcp` |
| Language | TypeScript 5.7+ | Node ≥20 |
| Auth | Majesta One JWT via `client_credentials` | Same AuthZ as family APIs |

## Vendor plane — public docs site

Not product runtime and **not this repository**. Customer pages on `one.majesta.net` are published by a separate Majesta CMS aggregator (Netlify, per-product sites). Pointer: [public-docs-site.md](./architecture/public-docs-site.md) ([BP-067](../backlog/BP-067-public-docs-site.md)). Do not add Astro, `tools/one-docs`, or `netlify.toml` here. Not Path A.

| Layer | Choice | Notes |
|---|---|---|
| Host | External CMS aggregator (Netlify) | `one.majesta.net`; this repo has no deploy dependency |
| Source markdown | GitHub `docs/` in this repo | Operators who clone the product still read install/API docs in-tree |
| Writer (public site) | CMS-repo agent | Not a product CI job; no `NETLIFY_*` in this repo |

## Vendor plane — IDE entitlement issuer

Not product runtime. Lives outside `cmd/api`. IDE entitlement chrome is frozen ([ADR-030](./adr/030-install-agent-runtime.md)).

| Layer | Choice | Notes |
|---|---|---|
| Money SoR | **Stripe Billing** + Customer Portal | IDE seats only; not DO infra |
| Mint | Small issuer: Stripe webhooks → signed JWS | Private key in KMS; `customer_id` + quantity → claims |
| Verify | Product install, offline | Public key in `one-api`; `PUT /metadata/v1/install/entitlement` |

## Explicit non-goals (v1)

- Multi-tenant shared-schema SaaS / `customer_id` routing (cross-customer isolation stays infra: VPC/DB/cluster)
- A **managed subscription fleet** or embedding a fleet control plane inside the product binary (`cmd/api` / `cmd/worker`); install-local Ops upgrades stay per install. **Stripe Billing for Control IDE seats** is vendor-plane (issuer + portal), not this fleet.
- Treating community `sdk/` trees as product GA or a third install path
- Forking Majesta One product source per customer (customer implementation = metadata/tests, not a private platform fork)
- GraphQL
- Embedding Control IDE in the product image (client lives under `tools/control-ide`; Mac/Linux/Windows installers are a separate packaging track — [control-ide-build.md](./control-ide-build.md))
- Admin / classic enterprise admin UI clone (Control IDE is the supported API consumer shell)
- Custom proprietary in-kernel scripting language (customer **code automations** are sandboxed Deno TypeScript per [ADR-014](./adr/014-customer-code-automations.md) — not an in-kernel language; no third-party imports in v1)
- Shipping Deno/Node as **platform** runtime (guest Deno is install-side executor for customer automation only; product binaries stay Go)
- Using Deployment API to ship kernel/DDL or managed package internals
- AMI / EC2 / Droplet Marketplace fulfillment (containers + Helm; preferred DO Kubernetes 1-Click)
- Shipping Node/TypeScript or a Python sidecar as **platform runtime** (vendor TypeScript under `tools/control-ide` and `tools/one-mcp` is allowed; MCP product gateway remains Go; public docs Starlight is not in this repo)

## Related docs

- [Architecture](./architecture.md)
- [Core data model](./data-model.md)
- [ADR-005: Go runtime](./adr/005-go-runtime.md)
- [ADR-006: Majesta One JWT auth](./adr/006-jwt-auth.md)
- [ADR-008: Core data model](./adr/008-core-data-model.md)
- [Community AWS Fargate notes](../sdk/aws/docs/aws-fargate.md)
- [Security](./security.md)
- [API families](./api-families.md)
- [Multi-env deploy](./multi-env-deploy.md)
- [Monorepo structure](./monorepo.md)
- [Release CI/CD](./release-cicd.md)
- [Public docs pointer (`one.majesta.net`)](./architecture/public-docs-site.md)
- [Control IDE build plan](./control-ide-build.md)
- [Control IDE design (tokens + updates)](./control-ide-design.md)
- [Mac local development](./local-development-mac.md)
- [Customer customizations](./customer-customizations.md)
- [DigitalOcean distribution build plan](./architecture/digitalocean-distribution-build-plan.md)
- [Backlog](../backlog/README.md)
