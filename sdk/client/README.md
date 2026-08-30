# Majesta One Client Experience kits (`sdk/client/`)

Open-source auth and Client API helpers for **customer-hosted Client Experience** apps (browser/mobile end-user UIs).

**Status:** Phases 1–6 landed ([BP-040](../../backlog/BP-040-client-experience-oss-kits.md)). Remainder **R1** (kit wire + tests) is in this tree: query/sobjects match the install, both kits pin `One-API-Revision`, `OneAPIError` parses `{ error, message }`. Refresh helpers (R2), Experience HTTP tests (R3), `@one/react`, and partner certification stay deferred.

## Packages

| Path | npm name | Role |
|---|---|---|
| [`auth/`](./auth/) | `@one/auth` | PKCE, authorize URL (client-only scopes), token exchange with revision pin |
| [`client/`](./client/) | `@one/client` | Typed `/client/v1` fetch (`query`, `search`, sobjects CRUD, `describe`, `me`) |
| [`examples/list-view/`](./examples/list-view/) | (sample app) | React + Vite Account list (`query({ object: "Account", select: ["Name"] })`) |

`@one/react` hooks are deferred.

## Security defaults

- Connected Apps: **`client` scope only** for public/browser clients
- Do **not** call Metadata or Deploy from browser apps
- See [docs/client-experience-security.md](../../docs/client-experience-security.md)

## Build and test packages

```bash
cd sdk/client/auth && npm install && npm test
cd ../client && npm install && npm test
```

## Boundary

| In product image? | License |
|---|---|
| **No** | Apache-2.0 |

Distinct from optional Control IDE (`tools/control-ide`) and community cloud SDKs (`sdk/aws`, etc.).

## Related

- [ADR-019](../../docs/adr/019-client-experience-oss-kits.md) · [client-experience-build-plan.md](../../docs/architecture/client-experience-build-plan.md)
- [customer-connect.md](../../docs/customer-connect.md) · [client-experience-telephony.md](../../docs/client-experience-telephony.md)
- [BP-013](../../backlog/BP-013-jwt-unified-principals.md) · [BP-022](../../backlog/BP-022-client-access-ide-device.md) · [BP-040](../../backlog/BP-040-client-experience-oss-kits.md)
