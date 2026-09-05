# Family API reference

Customer HTTP on a Majesta One **install**. Each family has its own path prefix, credential scope, and job.

| Family | Path | Scope | Page |
|---|---|---|---|
| Overview | — | — | [API families](../api-families.md) |
| Client | `/client/v1` | `client` | [client.md](./client.md) |
| Metadata | `/metadata/v1` | `metadata` | [metadata.md](./metadata.md) |
| Deploy | `/deploy/v1` | `deploy` | [deploy.md](./deploy.md) |
| Ops | `/ops/v1` | `ops` | [ops.md](./ops.md) |
| Auth | `/auth/v1` | (token / claim) | [auth.md](./auth.md) |

Objects on an install: [objects.md](../objects.md) (static catalog). Runtime `GET /client/v1/describe` is authenticated schema for that install — not the public docs source.

These pages are the GitHub source the public host should ingest. Do not copy [api-families.md](../api-families.md) onto each family route.
