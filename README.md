# Majesta One

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0-informational.svg)](#status)

Dedicated-install, metadata-driven enterprise platform. Each customer runs their own instance: one API, one Postgres database, one JWT issuer. **The API is the product.** The platform runtime is **Go**. The **entire repository is Apache-2.0**, including Control IDE.

Majesta One is a metadata platform with optional Sales, Service, and Billing modules you enable on an install.

## Status

**Alpha — Go platform `0.1.0`, Control IDE `0.1.0`.** This is the public initialization. Contracts, metadata, and packaging can still change in breaking ways. Pin image digests if you run it. Do not treat `0.1.0` as a stability promise.

## Install

**DigitalOcean App Platform** is the first targeted managed path. Compose and Helm cover local and self-install from image. Other cloud providers are expected later through community SDKs under [`sdk/`](sdk/README.md) — not product GA today.

| Path | What it is |
|---|---|
| **A — DigitalOcean App Platform** | First targeted managed install |
| **B — Self-install from image** | Compose (local/dev) or Helm on Kubernetes |

Details: [docs/self-host.md](docs/self-host.md).

**Control IDE** (`tools/control-ide`, `0.1.0`) is an optional desktop JWT client ([ADR-012](docs/adr/012-customer-repo-and-control-ide.md)). It is not in the product image and is not required to install, ship metadata, or run agents. Builders use MCP + `one` ([docs/builder-connect.md](docs/builder-connect.md)).

## Architecture

See [docs/glossary.md](docs/glossary.md), [docs/architecture.md](docs/architecture.md), [docs/tech-stack.md](docs/tech-stack.md), [docs/monorepo.md](docs/monorepo.md), [docs/api-families.md](docs/api-families.md), and ADRs under [docs/adr](docs/adr). Foreseeable risks live in [backlog/](backlog/).

- **Postgres** for all state
- **Go** platform API + worker
- **Metadata kernel** describes objects, fields, and rules
- **Flexible JSONB record store** so custom fields do not require production DDL
- **Three API families** — Client (data work), Metadata (shape this install), Deployment (promote customer implementation across same-customer environments)
- **Core data model** (User / Account / Contact) as managed package `core`
- **Worker** for automations, outbox/webhooks, and projections

## Quick start

### Prerequisites

- Go 1.25+
- Docker (for Postgres + full stack) *or* any Postgres 16+ URL

### Install

```bash
go mod download
cp .env.example .env
```

### Database

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
make migrate   # go run ./cmd/migrate
```

API boot also applies kernel migrations and optional core package seed (`AUTO_SEED=1`).

### Full stack (Compose)

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Production installs: [docs/self-host.md](docs/self-host.md).

### Run API / worker

```bash
make api       # go run ./cmd/api (auto-loads .env)
make worker    # go run ./cmd/worker
```

Health: `GET http://localhost:8080/healthz` (up after `one-api listening`; `/readyz` waits for seed)
Me: `GET http://localhost:8080/client/v1/me` with `Authorization: Bearer dev-admin-key`
Describe: `GET http://localhost:8080/client/v1/describe`

### Tests

```bash
make test          # go test ./...
make test-ide      # Control IDE Vitest
make ci            # boundary + lint + race+cover + build (local product CI)
```

### Control IDE (desktop)

```bash
cd tools/control-ide
npm ci && npm test && npm run electron:dev
```

Mac step-by-step (Postgres + API + JWT + IDE): [docs/local-development-mac.md](docs/local-development-mac.md).

## Monorepo layout

One Apache-2.0 monorepo for the **product** you publish/install, plus CI/CD and vendor tooling. Customer customizations are developed on installs (or a local gitignored sandbox), never shipped inside product images.

```
cmd/api               Platform API
cmd/worker            Worker
cmd/migrate           Kernel migrate CLI
internal/             Go packages (authz, metadata, dataengine, deploy, worker, …)
migrations/           Kernel SQL + journal
deploy/               Compose + Helm + DigitalOcean App Platform spec
sdk/                  Community SDKs (not product GA; other clouds later)
tools/                Vendor helpers + Control IDE (Apache-2.0; not in product image)
scripts/              Boundary checks + release helpers
.github/workflows/    CI (validate) + Release (tag → GHCR + artifacts)
.customer-sandbox/    Local-only scratch (gitignored)
```

| Concern | Doc |
|---|---|
| Monorepo planes & boundaries | [docs/monorepo.md](docs/monorepo.md) |
| Self-host (App Platform / Compose / Helm) | [docs/self-host.md](docs/self-host.md) |
| Version / CI/CD publish | [docs/release-cicd.md](docs/release-cicd.md) |
| Public docs (`one.majesta.net`) | Separate CMS aggregator — [docs/architecture/public-docs-site.md](docs/architecture/public-docs-site.md) |
| Customer customizations (safe workflow) | [docs/customer-customizations.md](docs/customer-customizations.md) |
| Mac local + Control IDE | [docs/local-development-mac.md](docs/local-development-mac.md) |
| Control IDE build / installers | [docs/control-ide-build.md](docs/control-ide-build.md) |

## License

This repository is licensed under the [Apache License 2.0](LICENSE), including the product plane (API, worker, migrations, packaging), Control IDE (`tools/control-ide`), community SDKs, and documentation. See [NOTICE](NOTICE).
