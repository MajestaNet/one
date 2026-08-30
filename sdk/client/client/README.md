# @one/client

Typed fetch helpers for Majesta One **Client API** (`/client/v1`).

Pass `apiRevision` (or rely on `PREFERRED_API_REVISION` / `CLIENT_PREFERRED_API_REVISION`, both `1`) so every family request sends `One-API-Revision`. The install advertises `{min,current,recommended}` on unauthenticated `GET /version` (`probeVersion`); pins outside that window return `400` / `API_REVISION_UNSUPPORTED` as `OneAPIError` (read `cta` for the operator hint). Optional path form: `/client/v1/r{N}/…` (same semantics). See [ADR-025](../../../docs/adr/025-api-revision-versioning.md).

Inject `fetch` on `OneClientConfig` for tests; default is `globalThis.fetch`.

**Live wire (revision 1):**

| Method | HTTP |
|---|---|
| `query({ object, select?, filters?, sort?, limit?, cursor?, includeDeleted?, mode? })` | `POST /client/v1/query` |
| `search({ q, objects?, limit? })` | `POST /client/v1/search` |
| `getRecord` / `createRecord` / `updateRecord` / `deleteRecord` | `GET\|POST\|PATCH\|DELETE /client/v1/sobjects/{object}[/{id}]` |
| `describe` / `describeObject` | `GET /client/v1/describe[/{object}]` |
| `me` | `GET /client/v1/me` |

There is no `/client/v1/records` path. Query uses `object` / `select` / `filters[]` / `cursor` — not `objectApiName` / `fields` / `filter` / `offset`.

**Scope:** Client family only. Metadata/Deploy belong in Control IDE or confidential service principals.

```bash
npm test   # tsc + node --test (mocked fetch; no live install)
npm run build
```

See [docs/client-experience-security.md](../../../docs/client-experience-security.md).
