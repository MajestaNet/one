# Building a Client Experience — security guide

How to build customer-hosted browser or mobile apps against a Majesta One install using OSS [`sdk/client/`](../sdk/client/) kits. See [ADR-019](./adr/019-client-experience-oss-kits.md) and [client-experience-build-plan.md](./architecture/client-experience-build-plan.md).

**Thesis:** Effective AuthZ stays on the install. Your Experience app is untrusted client code; narrow scopes and Connected App registration are the trust root.

---

## Surface comparison

| Surface | Who | Default API families | Connected App kind |
|---|---|---|---|
| **Client Experience** | End users in browser/mobile | `/auth/v1` + `/client/v1` only | `public` + PKCE |
| **Control IDE** (licensed) | Admins, builders, operators | Client + Metadata + Deploy (+ Ops) | Managed `one.controlIde` |
| **Service principal** | CI, bots, server integrations | Per assigned Role | `confidential` |

Forking Control IDE or building an alternate admin console is **unsupported**. Use optional Control IDE for Deploy/Metadata authoring.

---

## Connected App checklist

1. **Register** via `POST /client/v1/integrations` (or Control IDE Govern → Integrations).
2. Set **`clientKind`: `public`** for browser SPAs.
3. Set **`oauthFlows`: [`authorization_code`]** only — public clients cannot use `client_credentials`.
4. Set **`pkceRequired`: true** (default for public clients).
5. Register **exact redirect URIs** (dev + prod); no wildcards.
6. Set **`allowedScopesHint`: [`client`]** (platform default when omitted for public clients).
7. Assign **`roleApiNames`: [`StandardUser`]** only for Experience apps (default when omitted).
8. Promote **Experience metadata** (`metadata/experiences/*.yaml`) via Deploy when using the customer repo ([Phase 4](./architecture/client-experience-build-plan.md)).

---

## Token handling

| Do | Don't |
|---|---|
| Use PKCE authorization code flow | Embed break-glass `API_KEYS` in bundles |
| Keep access tokens in memory when possible | Store client secrets in SPA localStorage |
| Use short-lived JWTs from `/auth/v1` | Ship long-lived refresh tokens in query strings or SPA `localStorage` |
| Refresh via BFF, or `grant_type=refresh_token` only with `offline_access` | Copy Control IDE’s on-disk session into the browser |
| Redact tokens in logs | Log tokens to analytics or error reporters |

Prefer customer SSO (`/auth/v1/token/exchange`) when users already authenticate via corporate IdP ([auth-adapters.md](./auth-adapters.md)).

---

## Scope fence

**Supported in browser Experiences:**

- `POST /auth/v1/token`, `/auth/v1/token/exchange`, `/auth/v1/authorize` (via `@one/auth`)
- `GET/POST/PATCH/DELETE /client/v1/*` under the user's Majesta One JWT

**Unsupported / security smell in browser apps:**

- `/metadata/v1/*` — use Control IDE or a confidential server integration
- `/deploy/v1/*` — use Control IDE Ship or CI service principal
- `/ops/v1/*` — operator tooling only

The platform **rejects** `metadata`, `deploy`, `ops`, and `admin` in `allowedScopesHint` for `public` Connected Apps ([BP-040](../backlog/BP-040-client-experience-oss-kits.md) Phase 3).

---

## Hosting and CSP

- Host the built SPA on **customer infra** (CDN, App Platform static site, S3+CloudFront, etc.).
- Majesta One does **not** webpack customer UI into the product image.
- Set **Content-Security-Policy** on your static host: restrict `script-src` to your origin; avoid `unsafe-inline` in production.
- Register production **origins** in Experience metadata `allowedOrigins` when promoted via Deploy.
- CORS: call the Majesta One install API from your SPA origin; configure install exposure policy per [security.md](./security.md).

---

## Anti-patterns

| Anti-pattern | Why it fails |
|---|---|
| Metadata API from browser | Over-scoped token; bypasses IDE trust boundary |
| Deploy promote from SPA | Same; use repo → org via IDE/CI |
| API key in `VITE_*` env baked into bundle | Key is public to anyone who loads the app |
| Embedding Experience in Control IDE Electron | ADR-012 plugin ban |
| Telephony secrets in browser | Use connectors + server automations ([telephony guide](./client-experience-telephony.md)) |

---

## Related

- [customer-connect.md](./customer-connect.md) · [security.md](./security.md)
- [client-experience-telephony.md](./client-experience-telephony.md)
- [BP-013](../backlog/BP-013-jwt-unified-principals.md) · [BP-022](../backlog/BP-022-client-access-ide-device.md) · [BP-040](../backlog/BP-040-client-experience-oss-kits.md)
