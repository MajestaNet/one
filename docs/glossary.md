# Glossary

Canonical product vocabulary. Use these nouns in docs, env vars, JSON, and code. Do not reintroduce `tenant` or `Lattice` as product names.

The GitHub repo path remains `github.com/MajestaNet/ide` until it is moved to `one`.

## Product

| Term | Meaning |
|---|---|
| **Majesta One** | Product display name. Company/domain: Majesta / [majesta.net](https://majesta.net). |
| **One** | Short identifier in binaries, images, CLI, Helm chart, package format (`one/v1`), JWT audience (`aud=one`). |
| **Product** | Vendor binaries and managed seed. Never a customer’s YAML or metadata. |

## Runtime topology

| Term | Identifier | Meaning |
|---|---|---|
| **Customer** | `CUSTOMER_ID` | Commercial owner. Shared by every sibling install (test/staging/prod). One customization Git repo. |
| **Install** | `INSTALL_ID` | One running instance: one API, one Postgres database, one JWT issuer. |
| **Org** | CLI alias | DX name for the **connected install** (`one org validate` / `one org deploy`). Not the Account object. |
| **Custom** | `ownership=custom` | Customer-owned metadata (objects, automations, tests). Contrasts with `managed`. |
| **Managed** | `ownership=managed` | Product seed / kernel definitions. Upgraded with the image, never Deploy-promoted. |

Example: customer `acme-corp` runs installs `acme-test` and `acme-prod`. Custom objects live in the customer Git repo (`one/v1`) and deploy to each install. Account and Contact stay `managed`.

## Do not confuse

| Avoid | Use instead |
|---|---|
| `tenant` as the everyday noun | **customer** (commercial owner) or **install** (running instance) |
| `ownership=tenant` or `ownership=customer` | `ownership=custom` |
| `TENANT_ID` | `CUSTOMER_ID` |
| `lattice-tenant` CLI | `one` |
| `lattice-tenant/v1` | `one/v1` |
| Multi-tenant SaaS `tenant_id` on business rows | Out of scope (ADR-001). Isolation is one database per install. |

Account as an organization is unrelated to the CLI `org` command.

## CLI and packages

| Surface | Name |
|---|---|
| Customer DX CLI | `one` (`cmd/one`) |
| Customer repo format | `one/v1` (`one.yaml` manifest) |
| Pack/unpack package | `internal/customerrepo` |
| Guest automation module | `one:automation` |
| Control IDE npm | `@one/control-ide` |
| Client Experience kits | `@one/auth`, `@one/client` |
| Helm chart | `deploy/helm/one` |
| Images | `one-api`, `one-worker` |

## Architecture

- [ADR-001 Dedicated install](./adr/001-dedicated-install.md)
- [Multi-env deploy](./multi-env-deploy.md)
- [Customer customizations](./customer-customizations.md)
- [Customer repo format](./customer-repo.md)
