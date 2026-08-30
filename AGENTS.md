# Majesta One — agent notes

Dedicated install, metadata-driven enterprise platform (API-first). Product runtime has **no embedded UI**. The install is the **agent runtime** ([ADR-030](docs/adr/030-install-agent-runtime.md)): builders use MCP + `one`; Control IDE under `tools/control-ide` is an optional JWT client (ADR-012) — **refactor it when that cleans the install** ([BP-065](backlog/BP-065-ide-backend-coupling.md)).

## Starting instructions (do this first)

Before implementing or proposing architecture changes, get oriented:

1. Read [docs/glossary.md](docs/glossary.md), [docs/architecture.md](docs/architecture.md) and [docs/tech-stack.md](docs/tech-stack.md) for product vocabulary, model, and **confirmed** stack.
2. Read [docs/architecture/module-map.md](docs/architecture/module-map.md) and [docs/architecture/agent-routing.md](docs/architecture/agent-routing.md) — pick the domain, packages, and domain agent **before** broad exploration.
3. Open **one** domain playbook (see table below). For agent runtime / MCP / harness: [agent-runtime-build-plan.md](docs/architecture/agent-runtime-build-plan.md). For hosted `/agents/runs` tool execution: [hosted-agent-tool-loop-build-plan.md](docs/architecture/hosted-agent-tool-loop-build-plan.md). For Control IDE or install-coupling lockstep ([BP-065](backlog/BP-065-ide-backend-coupling.md)): the IDE playbook **plus** the matching backend playbook. For data model / seed / storage / query: also [docs/data-model.md](docs/data-model.md) and ADRs 008 / 002 / 003.
4. Read [docs/monorepo.md](docs/monorepo.md) and [docs/customer-customizations.md](docs/customer-customizations.md) — Apache-2.0 monorepo + optional Control IDE + community `sdk/`; never commit customer customizations into product paths. Agent runtime direction: [ADR-030](docs/adr/030-install-agent-runtime.md).
5. Read [docs/api-families.md](docs/api-families.md) and [ADR-004](docs/adr/004-three-api-families.md) when the change touches HTTP or family ownership (BP-010 mitigated).
6. Read [backlog/README.md](backlog/README.md) for foreseeable problems; open relevant `BP-*.md` items that touch your task. Treat each BP `Area:` as the preferred code scope (resolve via the module map).
7. Prefer solutions that reduce backlog risk (especially High severity) when the request is open-ended.
8. When you close or materially de-risk a backlog item, update its status in the item file and the table in `backlog/README.md`.

Do not invent a parallel stack. If a library choice conflicts with `docs/tech-stack.md`, justify the change and update that doc in the same change set.

### Task → playbook (focus here)

| Task concerns… | Playbook | Domain agent |
|---|---|---|
| Data model, seed, JSONB, query | [agent-data-architecture.md](docs/architecture/agent-data-architecture.md) | `db-backend-perf` |
| JWT, keys, Roles, scopes, principals | [agent-authz.md](docs/architecture/agent-authz.md) ([refresh tokens](docs/architecture/refresh-token-session-build-plan.md), [BP-063](backlog/BP-063-refresh-token-sessions.md)) | `authz-security` |
| HTTP routes / API family ownership | [agent-api-families.md](docs/architecture/agent-api-families.md) | `api-families` |
| Platform actions / Lead convert | [platform-actions-build-plan.md](docs/architecture/platform-actions-build-plan.md) (+ api-families + data) | `api-families` then `db-backend-perf` |
| Promote, peers, Ops rolls, packaging | [agent-deploy.md](docs/architecture/agent-deploy.md) | `deploy-ops` |
| Jobs, outbox, worker concurrency | [agent-worker.md](docs/architecture/agent-worker.md) | `worker-jobs` |
| Agent runtime (harness, MCP catalog, builder DX) | [agent-runtime-build-plan.md](docs/architecture/agent-runtime-build-plan.md) (+ api-families + worker) | `api-families` then `worker-jobs` (not `control-ide`) |
| Hosted agent tool loop (`/agents/runs` executes tools) | [hosted-agent-tool-loop-build-plan.md](docs/architecture/hosted-agent-tool-loop-build-plan.md) (+ api-families + worker) | `api-families` then `worker-jobs` (not `control-ide`) |
| Control IDE (Electron / React / Vitest) — optional client; **lockstep refactor OK** when it cleans the install ([BP-065](backlog/BP-065-ide-backend-coupling.md)); demo-client honesty ([BP-066](backlog/BP-066-ide-demo-client-fidelity.md)); no new Electron-only product chrome | [agent-control-ide.md](docs/architecture/agent-control-ide.md) · [ide-demo-client-uplift-build-plan.md](docs/architecture/ide-demo-client-uplift-build-plan.md) | `control-ide` |
| Public docs site (`one.majesta.net`) — allowlisted markdown, Astro publisher, merge-event docs updates ([BP-067](backlog/BP-067-public-docs-site.md)) | [agent-public-docs.md](docs/architecture/agent-public-docs.md) · [public-docs-site-build-plan.md](docs/architecture/public-docs-site-build-plan.md) | `docs-publisher` |

Architecture index: [docs/architecture/README.md](docs/architecture/README.md). ADR catalog: [docs/adr/README.md](docs/adr/README.md).

### Scope fence (IDE vs backend vs docs)

| Agent track | Edit | Do not edit |
|---|---|---|
| **IDE** (`control-ide`) | `tools/control-ide/**` (+ IDE docs in the playbook) | `cmd/`, `internal/`, `migrations/`, `deploy/` unless the task is explicitly cross-plane and a backend playbook is attached |
| **Backend** (Go domain agents) | `cmd/`, `internal/`, `migrations/`, `deploy/` as mapped | `tools/control-ide/**` unless the task is install-coupling cleanup ([BP-065](backlog/BP-065-ide-backend-coupling.md)) |
| **Docs** (`docs-publisher`) | Allowlisted public `docs/` + `tools/one-docs/**` (Phase 1–2: impact script / `docs-impact.yml` / `netlify.toml` per the playbook) | `cmd/`, `internal/` product logic, `tools/control-ide/**` |

Cross-plane work must cite both playbooks: API logic stays in Go; the IDE remains a JWT client. **BP-065 may edit the IDE to delete Go chrome routes** — that is preferred over keeping unused kernel APIs.

## Setup

```bash
go mod download
cp .env.example .env   # or use Cloud Secrets for DATABASE_URL / API_KEYS
```

Integration tests need Postgres (`DATABASE_URL`). Unit tests that skip without it still pass.

Mac developers running API + Control IDE: [docs/local-development-mac.md](docs/local-development-mac.md).

```bash
make test          # go test ./...
make test-race     # with -race when DATABASE_URL is set in CI
make test-ide      # Control IDE Vitest (under tools/control-ide)
```

## Commands

| Command | Purpose |
|---|---|
| `make test` / `go test ./...` | Go unit + integration tests |
| `make test-ide` | Control IDE unit + component tests |
| `make test-ide-integration` | Control IDE live-API contracts (API must be up) |
| `make cover` | Coverage profile summary |
| `make lint` | golangci-lint |
| `make ci` | lint + race tests with coverage + build (product / Go; one test pass) |
| `make api` / `go run ./cmd/api` | API server |
| `make worker` / `go run ./cmd/worker` | Worker |
| `make migrate` / `go run ./cmd/migrate` | Kernel SQL migrate |
| `make build` | Static binaries under `bin/` |

## Architecture constraints

- No multi-tenant SaaS `tenant_id` on business rows — one deploy = one customer DB (ADR-001)
- Custom fields are metadata + JSONB (`records.data`), never customer DDL
- Agents (when present) must call the **Client** API under service credentials (`scope: client`)
- **API scopes** — keys may be `name` (all scopes) or `name:client+metadata+deploy+ops`; add `+admin` for admin privilege (no substring matching)
- **Product vs customer implementation** — do not fork Majesta One per customer; Deploy API promotes customer-owned metadata/tests between that customer's installs only
- **Licensing** — the entire repository is **Apache-2.0** (root `LICENSE` / `NOTICE`), including Control IDE under `tools/control-ide`. See [digitalocean-distribution-build-plan.md](docs/architecture/digitalocean-distribution-build-plan.md).
- **Distribution** — **dual-path only:** **Path A** DigitalOcean **App Platform** is the **only** product managed PaaS path ([BP-029](backlog/BP-029-app-platform-install.md), [docs/self-host.md](docs/self-host.md)); **Path B** self-install from image (Compose + Helm). **Day-2 cloud ops:** host-agnostic Deploy verbs ([deploy-cloud-capability-contract.md](docs/architecture/deploy-cloud-capability-contract.md)); DO adapter first ([BP-030](backlog/BP-030-deploy-api-digitalocean-apps.md); [active plan](docs/architecture/do-app-platform-deploy-api-build-plan.md)). Community cloud SDKs under [`sdk/`](sdk/README.md) (e.g. [`sdk/aws`](sdk/aws/README.md) managed PaaS profile / Fargate power path) are optional Path B extensions — **not** product GA and **not** a second Path A. Marketplace publish deferred ([BP-028](backlog/BP-028-digitalocean-marketplace-listing.md)). Control IDE DO Govern UI is **frozen** ([ADR-030](docs/adr/030-install-agent-runtime.md)). No AMI/EC2/Droplet 1-Click. No managed subscription fleet.
- **Multi-environment Deploy** — shared `CUSTOMER_ID` + unique `INSTALL_ID`; free-form `INSTALL_ROLE`. Default `DEPLOY_PEER_MODE=customer`. See [docs/multi-env-deploy.md](docs/multi-env-deploy.md).
- Prefer `/client/v1`, `/metadata/v1`, `/deploy/v1`; keep `/v1` aliases for compatibility only
- **Product runtime language is Go only** (ADR-005). That does **not** forbid vendor TypeScript under `tools/` (Control IDE). Do not ship Node/TypeScript as platform runtime or reintroduce a Python sidecar.
- Control IDE is an optional client-only tree under `tools/control-ide` (ADR-012 / [ADR-030](docs/adr/030-install-agent-runtime.md)); never embed it in product images. **Refactor the IDE when that removes install coupling** ([BP-065](backlog/BP-065-ide-backend-coupling.md) · [ide-backend-coupling-review.md](docs/architecture/ide-backend-coupling-review.md)). Do not add Electron-only product chrome — builders use MCP + `one` ([builder-connect.md](docs/builder-connect.md)).
- Prefer `internal/testutil` for new Go integration tests that need DB + HTTP harness
- Confirmed stack lives in [docs/tech-stack.md](docs/tech-stack.md)
- Risk backlog lives in [backlog/](backlog/)
- Vendor/agent plane (`docs/`, `backlog/`, `.cursor/`, `tools/`, this file) and community `sdk/` do **not** ship in product images
- Do not edit plan files under `.cursor/plans/` unless asked
