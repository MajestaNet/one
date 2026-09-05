# API families

Every Majesta One **install** exposes versioned HTTP families. Pick the family that matches the work, then call it with a credential that carries that scope.

| Family | Path | Scope | Audience | Mutates |
|---|---|---|---|---|
| [Client](./api/client.md) | `/client/v1` | `client` | Apps, integrations, agents | Business records, queries, bulk/search, agent **runs**, identity assignment |
| [Metadata](./api/metadata.md) | `/metadata/v1` | `metadata` | Builders on **this** install | Objects, fields, rules, automations, permission-set **definitions**, packages |
| [Deploy](./api/deploy.md) | `/deploy/v1` | `deploy` | CI / `one org deploy` | Customer-owned bundles, tests, promote onto **this** install |
| [Ops](./api/ops.md) | `/ops/v1` | `ops` | Install operators | Product **image** confirm / roll / rollback on this install |
| [Auth](./api/auth.md) | `/auth/v1` | (mint / claim) | Humans, services, agents | Tokens and install claim — not business data |

Design: [ADR-004](./adr/004-three-api-families.md). Connect recipes: [builder-connect.md](./builder-connect.md) · [customer-connect.md](./customer-connect.md).

## What each family does not do

- **Client** does not create custom objects or promote environments.
- **Metadata** writes apply only to the install that receives the request. It does not copy changes to siblings.
- **Deploy** promotes **customer-owned** metadata and tests. It does not ship product images or managed `core` internals.
- **Ops** rolls the product image on this install. It does not promote customer metadata.
- **Auth** mints and revokes credentials. It is not a fourth data API.

Admin privilege does **not** fill in a missing family scope.

## Product vs customer implementation

| Concern | Lives where | How it changes |
|---|---|---|
| Majesta One product (API, worker, kernel schema, images) | This monorepo / GHCR | Product `v*` release → App Platform / Compose / Helm on each install |
| Managed metadata (`core`, optional modules) | Seeded with the image | Product upgrade + package migrate |
| Customer implementation | That customer’s installs (+ their Git) | Metadata API + [customer repo](./customer-repo.md); Deploy **repo→org** onto each install |
| Business records | That install’s database | Client API. Not promoted by default |

Customers buy and install Majesta One. Their customization unit is **metadata + tests**, not a fork of this repo.

## How to call the families

1. Discover `{min,current}` from unauthenticated `GET /version` (`apiRevision.recommended` is an alias of `current`).
2. Send `Authorization: Bearer <Majesta One JWT>` (or a bootstrap API key while claiming).
3. Send `One-API-Revision: N` on family routes (and `/mcp`). Optional path form: `/client/v1/r{N}/…`.
4. Prefer `/client/v1`, `/metadata/v1`, `/deploy/v1`, `/ops/v1`. Flat `/v1` is a deprecated alias for Client and Metadata only.

MCP (`POST /mcp`) is an adapter over family HTTP, not a fifth family. See [Client](./api/client.md#mcp-adapter).

## Objects

Managed objects and fields: [objects.md](./objects.md) plus per-package tables under [modules/](./modules/README.md). Runtime schema on an install is `GET /client/v1/describe` (authenticated). That endpoint is **not** the public catalog.

## Related

- [Client](./api/client.md) · [Metadata](./api/metadata.md) · [Deploy](./api/deploy.md) · [Ops](./api/ops.md) · [Auth](./api/auth.md)
- [Self-host](./self-host.md) · [Multi-env deploy](./multi-env-deploy.md) · [Product upgrades](./product-upgrades.md)
- [ADR-004](./adr/004-three-api-families.md) · [ADR-006](./adr/006-jwt-auth.md) · [ADR-007](./adr/007-platform-ops-upgrades.md) · [ADR-025](./adr/025-api-revision-versioning.md)
