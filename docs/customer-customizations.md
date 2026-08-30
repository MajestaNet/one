# Developing and testing customer customizations

Majesta One allows rich per-customer customization (objects, fields, validation, automations, customer tests). Those customizations are **customer implementation**, not product source. This document is the working agreement so vendors and agents never accidentally include a customer’s customization in a Majesta One release.

## Golden rules

1. **One product, many installs** — every customer runs the same Majesta One binaries from this monorepo.
2. **Customization lives on the install** — create/change via Metadata API (`/metadata/v1`) on a customer environment.
3. **Promote with Deploy, not Git product PRs** — move customer-owned artifacts between same-`CUSTOMER_ID` installs with `/deploy/v1`.
4. **Customer Git is auto-provisioned** — one AWS CodeCommit repo per `CUSTOMER_ID` in `one/v1` format ([customer-repo.md](./customer-repo.md)); never commit that tree into this monorepo.
5. **Never commit customer metadata into product paths** — not under `cmd/`, `internal/`, `migrations/`, or `internal/seed`.
6. **Local scratch is disposable** — use `.customer-sandbox/` (gitignored) or a throwaway Compose install; do not open a product PR that adds customer fixtures “just for Acme”.

## Allowed customization surfaces

| Surface | Where it lives | How it ships to another env |
|---|---|---|
| Custom objects / fields / rules | Install DB (`ownership=custom`) | Deploy bundle → promote |
| Automations, permission sets (customer) | Install DB | Deploy bundle → promote |
| AgentSpecs (customer playbooks) | Install DB (`agent_playbooks`) | Deploy bundle → promote |
| Customer test suites | Install DB (`/deploy/v1/tests`) | Included in customer promote / CI gate |
| Managed core definitions (`core` / `platform`) | Product seed (`ownership=managed`) | Product image upgrade only |
| Optional managed modules | Product image + `package_installs` | Admin enable via `/metadata/v1/packages`; product upgrades enabled modules |
| Platform actions (`lead.convert`, …) | Product Go catalog ([ADR-029](./adr/029-platform-actions.md)) | Image upgrade; **gated** by enabled packages; customers wrap via `ctx.invokeAction`, they do not own the verb |
| `agents_starter` templates | Product seed templates | Always-on clone → customer AgentSpecs ([customer-agents.md](./customer-agents.md)) |

Deploy **rejects** managed package internals in customer bundles. Metadata API **rejects** mutating managed definitions. See [api-families.md](./api-families.md) and [multi-env-deploy.md](./multi-env-deploy.md).

## Recommended workflow (customer or SI)

```text
1. Install Majesta One product vX on test (+ optional staging/prod); provision CUSTOMER_REPO_URL (CodeCommit or operator Git)
2. As admin on prod: POST /deploy/v1/packages/initialize-repo (or `one` / Metadata — Control IDE Repo is optional)
3. Clone the customer Git repo locally (any editor). Dual-write YAML under metadata/ (not .one/baseline)
4. Pack via POST /deploy/v1/packages/pack or `one pack`
5. `one org validate` + customer tests on test until green
6. CI gate (optional): docs/ci-customer-tests.md against test
7. `one org deploy` (or Deploy API) to staging/prod
8. Product upgrades (vX → vY) are a separate image roll; re-run customer tests after
```

Customer Git is **outside this monorepo**. Export/sync with `GET /deploy/v1/packages/export` (air-gap) or initialize-repo. Do not vendor customer trees into MajestaNet product source. Control IDE under `tools/control-ide/` is an optional frozen client ([ADR-030](./adr/030-install-agent-runtime.md)). Builder connect: [builder-connect.md](./builder-connect.md).

## Vendor / agent workflow (safe local testing)

When platform engineers need to exercise customization paths while building Majesta One:

### Preferred: ephemeral local install

```bash
docker compose -f deploy/docker-compose.yml up --build -d
# Point Metadata/Deploy clients at localhost:8080
# Create throwaway objects (e.g. SandboxDemo__c), run tests, tear down
docker compose -f deploy/docker-compose.yml down -v
```

Nothing from that DB is in Git. Nothing from that DB enters `docker build`.

### Optional: `.customer-sandbox/` scratch

```bash
mkdir -p .customer-sandbox/exports
# drop temporary bundle JSON / curl payloads here — entire tree is gitignored
```

Use this only for files you might curl into a local API. CI fails if any path under `.customer-sandbox/` is tracked.

### Forbidden patterns

| Pattern | Why it fails the boundary |
|---|---|
| Adding `internal/seed` objects named for a customer | Becomes product for every install |
| Checking in Deploy bundle JSON under `testdata/` with real customer schema | Leaks customer impl into product repo; risks shipping via copy-paste |
| Branching `apps/acme/` or forking platform per customer | Violates ADR product≠customer; forks diverge |
| Copying sandbox exports into `migrations/` | Kernel migrations are product-wide |
| “Temporary” customer field in managed `core` package | Pollutes managed ownership; upgrade hell |

## CI protections

On every PR / push, `scripts/assert-product-boundary.sh` verifies:

1. No tracked files under `.customer-sandbox/` or `customer-sandbox/`.
2. `deploy/Dockerfile` still only `COPY`s product paths (`cmd`, `internal`, `migrations`, module files).
3. No tracked `**/customer-exports/**` or `**/*.customer-bundle.json` paths.
4. `.dockerignore` excludes vendor/agent plane (`docs`, `backlog`, `.cursor`, `*.md`, `tools`, `scripts`).

After the Docker image smoke build, `scripts/assert-image-contents.sh` confirms `/one` + `/migrations` and rejects vendor-plane paths inside the image. Gitleaks runs on every PR and on product / Control IDE releases.

Product tests may use **synthetic** customer-owned metadata created at test time in Postgres (ephemeral). That is fine. Checked-in **customer** snapshots are not.

## Quick decision guide

| I want to… | Do this |
|---|---|
| Ship a platform bugfix | PR in this monorepo → CI → release tag → image roll |
| Add Invoice.Discount__c for Acme | Metadata API on Acme test → Deploy promote |
| Add a new standard core field for all customers | Product change in managed seed + package migrate |
| Gate Acme’s promote on tests | Deploy test suites on Acme test + [ci-customer-tests.md](./ci-customer-tests.md) |
| Experiment locally with a weird object | Compose install or `.customer-sandbox/` + API; delete when done |

## Related

- [Core data model](./data-model.md)
- [Monorepo structure](./monorepo.md)
- [Release CI/CD](./release-cicd.md)
- [CI customer tests](./ci-customer-tests.md)
- [Multi-env deploy](./multi-env-deploy.md)
- [Customer repo format](./customer-repo.md)
- [Customer developer workflow](./customer-developer-workflow.md) — best-practice DX loop
- [Customer DX build plan](./architecture/customer-dx-build-plan.md)
- [ADR-001](./adr/001-dedicated-install.md) · [ADR-004](./adr/004-three-api-families.md) · [ADR-008](./adr/008-core-data-model.md) · [ADR-012](./adr/012-customer-repo-and-control-ide.md)
