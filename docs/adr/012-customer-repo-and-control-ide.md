# ADR-012: Customer repo (`one/v1`) and Control IDE

## Status

Accepted

## Context

Customer implementation (metadata, tests, AgentSpecs) lives on install DBs and promotes via Deploy API. Customers and SIs need a cloneable, reviewable Git home for that implementation, plus a purpose-built Control IDE that orchestrates Connect → edit → pack → test → promote without forking Majesta One product source or embedding a second AuthZ system.

## Decision

### 1. One CodeCommit repo per `CUSTOMER_ID`

Each commercial customer gets an AWS CodeCommit repository `one-<CUSTOMER_ID>`, provisioned with the customer’s AWS stack (not per `INSTALL_ID`). All peer installs of that customer share the same `CUSTOMER_REPO_URL`. The normative on-disk format is **`one/v1`** ([customer-repo.md](../customer-repo.md)).

### 2. Pack/unpack is product Go; Git is not SoR for apply

- Pack YAML tree → `deploy.BundleArtifact` in `internal/customerrepo`.
- Apply/promote remains Deploy API (`ValidateBundleArtifact` / `ApplyBundleArtifact`).
- Install DB remains runtime SoR after apply; the repo is the reviewable source and CI input.

### 3. Control IDE is client-only under `tools/control-ide`

- Purpose-built Electron + Monaco (not an editor fork).
- Never ships in product images; never embeds API/provisioning logic.
- Auth: obtain Majesta One JWT (`/auth/v1/token/exchange` or client credentials) and pass `Authorization: Bearer` on family routes. Effective AuthZ stays install-local (ADR-006).

### 4. Planes unchanged

| Plane | Holds |
|---|---|
| Product | Go API, `internal/customerrepo`, Deploy pack/export routes, ECS + CodeCommit Terraform |
| Vendor | `docs/`, `tools/control-ide`, ADR index |
| Customer | CodeCommit repo + install DBs |

## Consequences

- Terraform writes `CUSTOMER_REPO_URL` / provider / region into install env.
- Deploy `GET /environment` exposes repo metadata for the IDE.
- `POST /deploy/v1/packages/pack` and `GET /deploy/v1/packages/export` round-trip `one/v1`.
- Mac / Windows / Linux installers via `electron-builder` (desktop installers — no hosted browser product). See [control-ide-build.md](../control-ide-build.md).

## Amendment — no customer IDE plugins (2026-07)

Control IDE remains a **fixed vendor React surface**. Customers customize via `one/v1` metadata rendered by first-party panels — not by shipping Electron/React components, iframes, or remote renderer code. Local dedicated installs do not change this trust boundary (session JWT + FS IPC + OS privileges). Domain-logic plugins, if ever added, follow BP-009 as sandboxed **server-side** executables on the Go install, not desktop UI bundles.

## Amendment — declarative CRM Canvas configure (2026-07)

Control IDE may host an agent-constructed **CRM Canvas** ([ADR-018](./018-crm-canvas-document.md)). Configuration and agent output are **declarative** (`CanvasDocument` / `CanvasSpec` YAML) rendered by vendor node kinds only. This does **not** reopen customer React plugins, iframes, remote renderer scripts, or an OSS browser Experience host as a second IDE. Automations remain server-side Deno ([ADR-014](./014-customer-code-automations.md)) and must not import canvas UI.

## Amendment — initialize from prod + managed baseline (2026-07)

- `POST /deploy/v1/packages/initialize-repo` (admin + `scope:deploy`) seeds remote `main` from the calling install using **pure-Go go-git** (`internal/deploy/gitremote`). No OS `git` in product images; Git is still not the apply SoR.
- Export/unpack may include `.one/baseline/` (managed object/field reference). Pack and promote **ignore** that tree.
- Control IDE Sync local: clone when empty, otherwise `git pull --ff-only origin/main`.
- See [customer-repo-init-build-plan.md](../architecture/customer-repo-init-build-plan.md) and [BP-031](../../backlog/BP-031-customer-repo-init-sync.md).

## Amendment — customer DX validate / deploy vs org (2026-07)

- Recommended multi-env path is **repo → org**: `POST /deploy/v1/packages/validate-local` (diff + `ValidateBundleArtifact`), then apply via promotions on the **connected** install. CLI: `one org validate|deploy|retrieve`. IDE: Ship **Validate vs org** / **Deploy to org**; Repo **Refresh from org**.
- Peer push (`POST /deploy/v1/bundles/{id}/push`) and inbound `{ artifact }` promote are **removed**. Multi-env = switch install and validate/deploy the same Git revision ([multi-env-deploy.md](../multi-env-deploy.md), [customer-dx-build-plan.md](../architecture/customer-dx-build-plan.md), [BP-032](../../backlog/BP-032-customer-dx-validate-deploy.md)).

## Amendment — Client Experience outside the IDE (2026-07)

OSS **Client Experience** apps (browser/mobile end-user UIs) are encouraged under [ADR-019](./019-client-experience-oss-kits.md): `sdk/client/` kits, Connected Apps with `client` scope only, customer-hosted SPAs. This does **not** reopen customer Electron plugins or load arbitrary code into Control IDE. Admin-IDE forks are unsupported; Deploy/Metadata authoring is MCP + `one` + family HTTP ([ADR-030](./030-install-agent-runtime.md)). Control IDE is an optional thin client of the same APIs.

## Amendment — Control IDE optional / frozen chrome (2026-08)

[ADR-030](./030-install-agent-runtime.md) keeps customer Git + pack/unpack as product Go. **Builder DX** is MCP + `one` CLI + MCP, not Control IDE Ship/Build panels. Control IDE remains a client-only tree under `tools/control-ide` and must not ship in product images; **new IDE chrome, graphs, license onboarding, and in-IDE agent hosts are frozen**. End-user UX stays Client Experience ([ADR-019](./019-client-experience-oss-kits.md)). Admin-IDE forks remain unsupported.

## Related

- [ADR-001](./001-dedicated-install.md) · [ADR-004](./004-three-api-families.md) · [ADR-005](./005-go-runtime.md) · [ADR-006](./006-jwt-auth.md) · [ADR-019](./019-client-experience-oss-kits.md) · [ADR-030](./030-install-agent-runtime.md)
- [customer-repo.md](../customer-repo.md) · [customer-customizations.md](../customer-customizations.md) · [monorepo.md](../monorepo.md)
- [customer-ide-ux.md](../customer-ide-ux.md) · [BP-009](../../backlog/BP-009-no-in-kernel-language.md) · [BP-031](../../backlog/BP-031-customer-repo-init-sync.md) · [BP-032](../../backlog/BP-032-customer-dx-validate-deploy.md) · [BP-039](./018-crm-canvas-document.md) · [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) · [ADR-018](./018-crm-canvas-document.md) · [ADR-019](./019-client-experience-oss-kits.md)
