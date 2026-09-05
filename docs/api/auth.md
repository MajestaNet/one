# Auth (`/auth/v1`)

Mint, refresh, and revoke **Majesta One JWTs**, claim a fresh install, and (when configured) broker human login. Auth is not a data family: after you have a token, call [Client](./client.md) / [Metadata](./metadata.md) / [Deploy](./deploy.md) / [Ops](./ops.md) with the matching scope.

Bootstrap API keys (`API_KEYS`, optional `name:client+metadata+deploy+ops` plus `+admin`) are accepted while claiming and as a break-glass. Day-2 work should use a principal + JWT. Admin privilege does **not** fill in a missing family scope.

Unauthenticated discovery: `GET /version` (API revision `{min,current}`), `GET /healthz`, `GET /readyz`.

Connect recipes: [customer-connect.md](../customer-connect.md) · [builder-connect.md](../builder-connect.md).

## Tokens

| Method | Path | Auth | What it does | What it does not |
|---|---|---|---|---|
| `POST` | `/auth/v1/token` | client credentials, authorization code, password, or `refresh_token` | Issue a Majesta One access JWT (`token_type=Bearer`) | Bypass Role scopes encoded on the principal |
| `POST` | `/auth/v1/token/exchange` | configured IdP assertion | Exchange an external token for a Majesta One JWT | Become a multi-tenant SaaS IdP |
| `POST` | `/auth/v1/revoke` | refresh token | Revoke an opaque refresh token | Revoke every credential on the principal (use Client credentials revoke) |
| `GET` | `/auth/v1/.well-known/openid-configuration` | public | OIDC discovery for this install | |

Interactive human grants may return an opaque `refresh_token`. Replay it with `grant_type=refresh_token`. Public clients that need refresh must request `offline_access`.

Send the access token as `Authorization: Bearer <jwt>` and pin `One-API-Revision` on family routes.

## Install claim

| Method | Path | Auth | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/auth/v1/install/status` | public | Whether this install is still unclaimed | |
| `POST` | `/auth/v1/install/claim` | claim secret / first-run | Bind the first admin and disable anonymous claim | Repeat after the install is claimed |

SSO / JIT config after claim is Metadata `GET|PUT /metadata/v1/install/auth` (`identity.manage`).

## Human login (when providers are configured)

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` | `/auth/v1/login` | Hosted login page | A product CRM UI |
| `GET` | `/auth/v1/login/providers` | Configured social/IdP providers | |
| `GET` | `/auth/v1/authorize` | Authorization-code start | |
| `GET` | `/auth/v1/callback/{provider}` | Provider callback | |

## Connector OAuth

| Method | Path | What it does |
|---|---|---|
| `POST` | `/auth/v1/connectors/{apiName}/authorize` | Start outbound connector OAuth (`metadata` + build) |
| `GET` | `/auth/v1/connectors/callback` | Provider callback |

Connector **definitions** stay on [Metadata](./metadata.md).

## SCIM

`/scim/v2` is a connector adapter over Client identity (Users; groups as directory tags). It is not a fifth commercial family. Requires a Client identity principal. See operator notes in [auth-adapters.md](../auth-adapters.md).

## Related

- [API families overview](../api-families.md) · [Client identity](./client.md#identity-assignment-on-this-install)
- [Customer connect](../customer-connect.md) · [Security](../security.md)
