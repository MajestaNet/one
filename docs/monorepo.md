# Majesta One monorepo structure

Majesta One is developed as **one monorepo**. The **entire repository** (product plane, Control IDE, community SDKs, and docs) is **open source (Apache-2.0)**. **Control IDE** under `tools/control-ide/` is an **optional frozen client** ([ADR-030](./adr/030-install-agent-runtime.md)) — not required to install, Ship, or run agents. **Community cloud SDKs** live under `sdk/` (not product GA). The repo also holds CI/CD and vendor/agent docs that do **not** ship in product images. It does **not** hold customer-specific customizations that should ride Deploy between a customer’s installs.

## Design goals

1. **One product codebase** — a single Majesta One application customers install (OSS). Builders use MCP + `one`; Control IDE is optional.
2. **Versioned release path** — every change can move through CI and become a publishable install artifact (GHCR images + digests on `v*` tags).
3. **Hard product ↔ customer boundary** — customer customizations are developed and tested against installs (or a local sandbox), never baked into product images or release tarballs.

See also: [glossary](./glossary.md), [release CI/CD](./release-cicd.md), [self-host](./self-host.md) (Path A App Platform / Path B Compose+Helm), [customer customizations](./customer-customizations.md), [API families](./api-families.md), [family reference](./api/).

## Top-level layout

```text
/
├── LICENSE, NOTICE          Apache-2.0 for the entire repository
├── SECURITY.md              Vulnerability disclosure
├── AGENTS.md                Agent entry (vendor plane; not in image)
├── cmd/                     Product binaries (api, worker, migrate, one CLI)
├── internal/                Product Go packages
├── migrations/              Kernel SQL (product schema only)
├── deploy/                  Install / publish packaging
│   ├── Dockerfile           Product images (cmd + internal + migrations only)
│   ├── docker-compose.yml   Local/dev stack (Path B; one install)
│   ├── docker-compose.multi-env.yml  Rollout lab: prod + test siblings
│   ├── helm/one         Kubernetes chart (Path B; + values-doks.yaml)
│   └── digitalocean/        App Platform Spec (Path A default)
├── sdk/                     Community SDKs (Apache-2.0; not in product image)
│   ├── client/              OSS auth + Client API kits (ADR-019; planned)
│   ├── aws/                 AWS identity / ops / edge / deploy + docs
│   ├── azure/               Stub layout
│   └── gcp/                 Stub layout
├── .github/workflows/       CI (validate) + release (GHCR + GitHub Release)
├── .cursor/agents/          domain agent definitions (vendor plane)
├── docs/                    Product + ops + architecture agent docs
├── backlog/                 Foreseeable product risks
├── tools/                   Vendor-only helpers; Control IDE (Apache-2.0)
├── scripts/                 Repo automation (boundary checks, release helpers)
└── .customer-sandbox/         Local-only customization scratch (fully gitignored)
```

| Path | Ships in product image? | Role |
|---|---|---|
| `cmd/`, `internal/`, `migrations/` | **Yes** | Majesta One application (**product plane**) |
| `deploy/` | Packaging inputs | How customers install / run the app (Path A + Path B) |
| `sdk/` | **No** | Community SDKs (Apache-2.0): `sdk/client/` Client Experience kits ([ADR-019](./adr/019-client-experience-oss-kits.md)) + cloud helpers — **not** product GA |
| `.github/workflows/` | No | CI/CD for product versions |
| `docs/`, `backlog/`, `.cursor/`, `AGENTS.md` | No | **Vendor/agent plane** — architecture, backlog, subagent routing |
| `tools/`, `scripts/` | No | Vendor automation |
| `.customer-sandbox/` | **Never** | Ephemeral local customer experiments (`mkdir` locally; not in Git) |

The vendor/agent plane stays in the monorepo for humans and coding agents. It is excluded from product image build context (`.dockerignore`) and never `COPY`ed into `deploy/Dockerfile`. Do not move agent guidance under `cmd/`, `internal/`, or `migrations/`, and do not `go:embed` those docs into binaries. Community `sdk/` is likewise excluded from product images.

## Three planes

```text
┌─────────────────────────────────────────────────────────────┐
│  PRODUCT PLANE (this monorepo)                              │
│  cmd + internal + migrations → versioned images/binaries    │
│  Released via CI/CD → App Platform / Compose / Helm         │
└─────────────────────────────────────────────────────────────┘
                          │ product upgrade (image roll)
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  INSTALL PLANE (per customer environment)                   │
│  Same product binary + that install’s DB                    │
│  CUSTOMER_ID shared; INSTALL_ID unique                        │
└─────────────────────────────────────────────────────────────┘
                          │ Deploy API (customer-owned only)
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  CUSTOMER IMPLEMENTATION PLANE                                │
│  Metadata, automations, customer tests — live in install DBs  │
│  Auto-provisioned customer Git repo (one/v1)       │
│  Never committed into this monorepo as product source       │
└─────────────────────────────────────────────────────────────┘
```

## What belongs in the monorepo

**In (product):**

- Platform API, worker, migrate CLI
- Kernel migrations and managed seed package (`core`)
- Deploy engine that promotes **customer-owned** artifacts between peer installs
- Install packaging (Dockerfile, Compose, Helm, DigitalOcean App Spec)
- Product CI/CD and documentation

**In (community, not product GA):**

- Cloud SDKs under `sdk/` (AWS / Azure / GCP) — optional Path B helpers

**Out (not product source):**

- A customer’s custom objects/fields/rules/tests as “their fork of Majesta One”
- Per-customer Go forks or privately patched platform trees
- Production business data
- Secrets, API keys, JWT signing material

## Boundary enforcement

| Control | Mechanism |
|---|---|
| Image contents | `deploy/Dockerfile` copies only `cmd/`, `internal/`, `migrations/` |
| Build context | `.dockerignore` excludes sandbox, tools, docs, backlog, `.cursor`, `sdk`, `*.md`, git metadata |
| Git hygiene | `.customer-sandbox/` is gitignored; CI fails if it becomes tracked |
| Image audit | `scripts/assert-image-contents.sh` after Docker build — requires `/one` + `/migrations`; rejects vendor-plane paths |
| IDE artifacts | `scripts/assert-ide-artifacts.sh` — no `.map` / `.env` in packaged dirs |
| Secrets | Gitleaks on every PR and on `v*` / `control-ide-v*` releases |
| Runtime promote | Deploy API excludes managed package internals; only customer-owned metadata |
| Process | [customer-customizations.md](./customer-customizations.md) — develop on installs, not in product paths |

`scripts/assert-product-boundary.sh` is the automated check for the git/image boundary (Dockerfile allowlist **and** required `.dockerignore` vendor-plane excludes including `tools`/`scripts`/`sdk`). Run it in CI on every PR.

### Customer install package vs vendor monorepo

| Ships to customers | Stays vendor-only / community |
|---|---|
| Distroless `api` / `worker` images (+ `migrate` binary on product releases) | `docs/`, `backlog/`, `.cursor/`, `AGENTS.md` |
| App Platform Spec + Helm/Compose packaging | Full monorepo, agent playbooks, domain agents |
| Control IDE desktop installers (`control-ide-v*` — separate channel) | Vite browser `dev` mode, IDE source maps |
| Community AWS helpers under [`sdk/aws/`](../sdk/aws/) (optional; not GA) | Managed **prod credentials** and auto-roll pipelines |

Community AWS Marketplace / managed-channel notes under `sdk/aws/` document optional or historical shapes; they are **not** a product install path and **not** a managed subscription GA. Prefer Path A / Path B in [self-host.md](./self-host.md).

See [security.md](./security.md) (IP / distribution) and [sdk/aws/docs/marketplace.md](../sdk/aws/docs/marketplace.md).

## Scaling the monorepo later

Keep a **single Go module** for the product until a real second shippable binary appears. Prefer:

- `cmd/<binary>` for new product processes
- `internal/<domain>` for product packages
- `tools/<name>` for vendor CLIs / clients that must not ship (e.g. `tools/control-ide`). Do **not** add a public-docs publisher here — `one.majesta.net` is a separate CMS aggregator ([public-docs-site.md](./architecture/public-docs-site.md)).
- `sdk/<cloud>/` for community cloud helpers that must not ship in product images

Do **not** introduce `apps/customer-*` trees. Customer implementation stays on installs + Deploy + the auto-provisioned customer Git repo ([customer-repo.md](./customer-repo.md)).

Do **not** fork the product onto a long-lived `managed-prod` branch for channel isolation — use trunk + `v*` tags and separate roll environments instead.
