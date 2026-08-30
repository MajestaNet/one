# Auth adapters — external IdP setup

How to connect **customer-run** identity providers to Majesta One. Product AuthN remains Majesta One JWT; AuthZ remains Postgres Roles + permission sets ([ADR-015](../adr/015-idp-agnostic-social-login.md)).

**Day-0:** claim the install with `INSTALL_CLAIM_TOKEN` (`POST /auth/v1/install/claim`) — [install-claim-sso-build-plan.md](./architecture/install-claim-sso-build-plan.md).

**Preferred human login:** customer SSO via Metadata `PUT /metadata/v1/install/auth` (Control IDE Govern → Install auth), then login page **Continue with {IdP}** or `POST /auth/v1/token/exchange`.

**Optional:** Google / Apple when the customer enables `socialProviders` (plus deploy secrets). Env `AUTH_LOGIN_PROVIDERS` remains a lab override.

**This doc:** adapters that end at `POST /auth/v1/token/exchange` (or Control IDE “Exchange ID token”), aligning env `OIDC_*` with DB install auth, and SCIM UserCustom attribute mapping for Okta / Entra.

## Common contract

1. Create or SCIM-provision a Majesta One `users` row with ≥1 Role (or enable auto-provision carefully).
2. Ensure an `identity_links` row for `(provider, issuer, subject)` — either by prior admin link or first successful exchange with auto-provision.
3. Obtain an **OIDC ID token** from the customer IdP (not an opaque access token). Entra/Okta/Keycloak ID tokens typically omit Cognito `token_use`; Majesta One accepts a missing `token_use` and rejects `token_use=access`.
4. Exchange:

```http
POST /auth/v1/token/exchange
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ietf:params:oauth:grant-type:token-exchange
&subject_token=<ID_TOKEN>
&subject_token_type=urn:ietf:params:oauth:token-type:id_token
```

5. Call family APIs with `Authorization: Bearer <Majesta One JWT>`.

Install env for verification:

```bash
OIDC_ISSUER=https://<idp-issuer>
OIDC_AUDIENCE=<one-client-id-at-idp>
# OIDC_JWKS_URI=   # optional; if unset, fetched from OpenID discovery jwks_uri
#                  # (never guessed as ${OIDC_ISSUER}/.well-known/jwks.json when discovery is available)
OIDC_AUTO_PROVISION_USERS=0   # prefer pre-provision for production
```

**Never** map IdP groups to Majesta One Roles or permission sets.

---

## Okta

1. Create an OIDC app (Web or Native) with Authorization Code + PKCE.
2. Sign-in redirect URIs include Control IDE callback or your admin UI.
3. Set `OIDC_ISSUER` to the Okta org issuer (e.g. `https://dev-XXXX.okta.com/oauth2/default`).
4. Set `OIDC_AUDIENCE` to the Okta app client id.
5. Pre-create Majesta One users (SCIM or Client principals) or enable auto-provision for lab only.
6. Exchange the Okta ID token as above (Control IDE: paste ID token → Exchange).

## Microsoft Entra ID

1. App registration → Authentication → platform with redirect URIs.
2. Expose an API / ID token audience = Application (client) ID.
3. `OIDC_ISSUER=https://login.microsoftonline.com/<customer-id>/v2.0`
4. `OIDC_AUDIENCE=<application-client-id>`
5. Same exchange path. Prefer setting `OIDC_JWKS_URI=https://login.microsoftonline.com/<tid>/discovery/v2.0/keys` **or** leave it unset so the install uses discovery `jwks_uri`. Do not rely on a guessed `/.well-known/jwks.json` path.

## Keycloak (customer-operated)

Majesta One does **not** ship or embed Keycloak.

1. Operator runs Keycloak (or equivalent) for the customer.
2. Create a confidential or public client; enable Standard flow.
3. `OIDC_ISSUER=https://<keycloak>/realms/<realm>`
4. `OIDC_AUDIENCE=<client-id>`
5. Exchange Keycloak ID tokens into Majesta One JWTs.

## Cognito Hosted UI (optional AWS community SDK)

1. Optional: community Cognito TF under [`sdk/aws/deploy/ecs`](../sdk/aws/deploy/ecs) — Path B extension only; **not** product GA.
2. Set `OIDC_ISSUER` / `OIDC_AUDIENCE` to the pool and app client.
3. Optional write-through: `IDENTITY_SYNC=cognito` + pool IAM — **not** required for Path A / Path B GA.
4. Prefer migrating Control IDE to Majesta One `/auth/v1/authorize` (Google/Apple) instead of Cognito Hosted UI.

## SCIM directory (Okta / Entra)

Human (and service) provisioning uses `/scim/v2` against the same `users` table as Client principals. Product AuthZ stays Roles + permission sets — **never** IdP groups.

Full protocol notes: [scim-provisioning.md](./architecture/scim-provisioning.md). Customer User fields and install `provisioning`: [user-identity-extension-build-plan.md](./architecture/user-identity-extension-build-plan.md) ([BP-058](../backlog/BP-058-user-identity-extension.md) mitigated).

### Shared recipe

1. Create customer User fields via Metadata (`POST /metadata/v1/fields`, `objectApiName=User`, e.g. `CostCenter__c`). Do not add kernel columns.
2. Confirm discovery: `GET /scim/v2/Schemas` lists `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom` with those apiNames.
3. Create a Connected App / service principal with Role scope `client`. Grant:
   - `identity.users` — provision human Users
   - `authz.manage` — only if the connector sets `roleApiNames` / `permissionSetApiNames` / `dataRoleApiName`
4. Mint a Majesta One JWT: `POST /auth/v1/token` (client credentials). Prefer install `clientAccessMode` of `registered_clients` or `open`.
5. In the IdP SCIM app, Base URL `https://<install>/scim/v2`, Authorization Bearer = that JWT. Content-Type `application/scim+json`.
6. Map directory attributes onto UserCustom apiNames (see vendor notes below). `enterprise.employeeNumber` maps to `users.employee_number` — do **not** alias it onto `externalId`.
7. Optional install defaults: `PUT /metadata/v1/install/auth` `provisioning.scimDefaultRoleApiName` / `scimDefaultPermissionSetApiNames` / `scimDefaultDataRoleApiName` so creates without the Majesta One Principal extension still get a Role (default `StandardUser`).
8. JIT login (not SCIM) can map IdP claims on **first create only** via `provisioning.claimMappings`.

**Never** map IdP groups to Majesta One Roles or permission sets. If a connector must pick a Role, map a directory attribute into `urn:ietf:params:scim:schemas:extension:one:2.0:Principal:roleApiNames`.

Enable **Push Groups** against `/scim/v2/Groups`. That creates **directory tags** (Client `/client/v1/directory-tags`) and memberships only — tags are not Roles, permission sets, or data roles. `User.groups` on GET is read-only; membership is managed on `/Groups`. Grant `identity.users` for human tag/Group ops and `identity.integrations` to tag `service` / `agent` principals. Do **not** grant `authz.manage` for tagging.

### Okta

1. Add a SCIM 2.0 application (or enable Provisioning on the OIDC app).
2. Provisioning → To App: Create / Update / Deactivate Users. Enable Group Push to `/scim/v2/Groups` (directory tags). Do not treat Group Push as AuthZ.
3. Attribute mappings: add unmapped attributes with external namespace  
   `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom`  
   and external name equal to the Majesta One field apiName (`CostCenter__c`). Okta PATCH `add` on  
   `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom:CostCenter__c` is supported.
4. Map Okta `employeeNumber` to SCIM `enterprise.employeeNumber` (not `externalId`). Use a stable federation id for `externalId`.

### Microsoft Entra ID

1. Enterprise application → Provisioning → target `customappsso` / SCIM.
2. Customer URL `https://<install>/scim/v2`, Secret token = Majesta One JWT.
3. Mappings → Provision Microsoft Entra ID Users: add a custom attribute whose SCIM path is  
   `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom:CostCenter__c`.
4. Same employeeNumber / externalId split as Okta. Push Groups to `/scim/v2/Groups` as directory tags — not as Roles or permission sets.

## Slack (Sign in with Slack OpenID)

Slack **user identity** via OpenID Connect — not bot tokens, Incoming Webhooks, or the Events API (those are connectors).

1. Create a Slack app → **OpenID Connect** / Sign in with Slack.
2. Redirect URI: `https://<install>/auth/v1/callback/slack`.
3. Lab env (broker secrets, same pattern as Google/Apple):

```bash
AUTH_SLACK_CLIENT_ID=<slack-client-id>
AUTH_SLACK_CLIENT_SECRET=<slack-client-secret>
AUTH_LOGIN_PROVIDERS=slack
```

4. Enable on the install: Metadata `PUT /metadata/v1/install/auth` with `socialProviders: ["slack"]` (DB list wins when non-empty). Hosted login shows **Continue with Slack** only when the install is claimed and Slack is enabled.
5. PKCE: `GET /auth/v1/authorize?provider=slack&…` → Slack → callback → Majesta One JWT. `identity_links.provider` is the exact string `slack`.
6. Token exchange: Slack **OIDC ID tokens** (`iss=https://slack.com`) with `subject_token_type=urn:ietf:params:oauth:token-type:id_token`. Bot tokens (`xoxb-*`) are rejected (`401 INVALID_TOKEN`). Unknown workspace issuers are `401`. Disabled Slack is omitted from `GET /auth/v1/login/providers` and authorize returns `PROVIDER_DISABLED`.
7. First provision requires a verified Slack email (`email` + `email_verified`), same JIT/domain allowlist as other brokers.

Family APIs still accept **Majesta One JWT only**. Slack access tokens stay `401`.

## Security notes

- Verify ID tokens server-side (signature, `iss`, `aud`, `exp`); Majesta One already does this for configured `OIDC_*` (JWKS from `OIDC_JWKS_URI` or OpenID discovery `jwks_uri`).
- Prefer PKCE for public clients; never embed IdP client secrets in Control IDE.
- Keep `OIDC_AUTO_PROVISION_USERS=0` in production unless domain allowlists and monitoring are in place.
- Family routes reject Google/Apple/Okta access tokens — only Majesta One JWTs.
