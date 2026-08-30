# Customer connect paths

How callers authenticate to a Majesta One install and which surface they use. One AuthN contract: **Majesta One-issued JWT** (or bootstrap API key). One AuthZ model: Roles (family scopes) + permission sets. Three practical paths:

| Path | Principal | Typical caller | Surface |
|---|---|---|---|
| **A — Humans / UI** | `user` | Control IDE (optional) or a custom UI / Client Experience | Family HTTP APIs with user JWT |
| **B — Service accounts** | `service` | CI, Deploy bots, backend integrations, `one` | Family HTTP APIs with client-credentials JWT |
| **C — Agents** | `agent` (or `service`/`user` with grants) | MCP hosts, coding agents, CI | Product MCP gateway at `/mcp`, or Client/Metadata/Deploy HTTP |

See [ADR-006](./adr/006-jwt-auth.md), [ADR-010](./adr/010-customer-agentic-platform.md), [ADR-030](./adr/030-install-agent-runtime.md), [customer-agents.md](./customer-agents.md), [builder-connect.md](./builder-connect.md).

**MCP server ownership:** the install **is** the MCP server ([ADR-010](./adr/010-customer-agentic-platform.md)). Customers configure principals, credentials, and AgentSpecs — they do **not** author product MCP server code. Optional custom tools live in the vendor-plane TypeScript scaffold under [`tools/one-mcp`](../tools/one-mcp/) (customer-hosted; never in the product image).

**Not the same as code automations:** sandboxed Deno guest TypeScript ([ADR-014](./adr/014-customer-code-automations.md)) runs inside the worker with `one:automation` only — it is not an MCP server and cannot import npm packages.

---

## Path A — Humans / UI (JWT identity)

Path A splits into two supported surfaces:

| Surface | Typical caller | API families (default) |
|---|---|---|
| **Control IDE** (optional / frozen chrome) | Admins who prefer a desktop client | Client + Metadata + Deploy (+ Ops) per JWT scopes |
| **Client Experience** (OSS kits) | End users in browser/mobile | `/auth/v1` + `/client/v1` only |
| **MCP / CLI** (builder DX of record) | Admins, SIs, coding agents | Family HTTP + MCP; Ship via `one` |

See [ADR-019](./adr/019-client-experience-oss-kits.md) · [client-experience-build-plan.md](./architecture/client-experience-build-plan.md) · [BP-040](../backlog/BP-040-client-experience-oss-kits.md) for customer-hosted Experiences. Forking an admin IDE is unsupported; security defaults argue for Client-API-only browser apps.

1. Day-0: claim the install (`POST /auth/v1/install/claim`) or use break-glass `API_KEYS`.
2. Ensure install Auth is configured (`AUTH_JWT_SIGNING_KEY`; customer SSO via `/metadata/v1/install/auth` and/or optional social).
3. Humans sign in via:
   - **Password** (`grant_type=password`) when enabled.
   - **Control IDE:** Managed Connected App `one.controlIde` (PKCE) → Majesta One login page (SSO primary when configured).
   - **Client Experience:** Tenant-registered Connected App (PKCE, `client` scope only) → `@one/auth` kit ([ADR-019](./adr/019-client-experience-oss-kits.md)).
   - **Custom UI / exchange:** customer OIDC → `POST /auth/v1/token/exchange` ([auth-adapters.md](./auth-adapters.md)).
4. Call family APIs with `Authorization: Bearer <Majesta One JWT>`. Effective AuthZ is always resolved from the `sub` principal in Postgres. **Find** is Client `POST /client/v1/search` (ranked hits on `searchable` fields) — the same API Control IDE Operate uses for the top-bar combobox.

Control IDE persists the access JWT and opaque refresh token in an encrypted session file (OS keyring or AES-GCM fallback). Close/reopen silently calls `grant_type=refresh_token` ([refresh-token-session-build-plan.md](./architecture/refresh-token-session-build-plan.md) / [BP-063](../backlog/BP-063-refresh-token-sessions.md)). Sign out revokes the refresh family. Each install is its own issuer — a refresh token does not unlock peer environments.

Remainders: Slack exchange and optional ALB mTLS — [BP-013](../backlog/BP-013-jwt-unified-principals.md), [BP-022](../backlog/BP-022-client-access-ide-device.md).

**Password recovery (no product mailer):** admins set/rotate passwords with `POST /client/v1/principals/{id}/password` or Control IDE Users → Set password. Authenticated users change their own via `POST /client/v1/me/password`. There is no forgot-password email ([BP-038](../backlog/BP-038-no-product-mailer-byo-alerts.md)). Prefer customer SSO for IdP-owned invites/MFA/reset.

---

## Path B — Service accounts

1. Create a service principal: `POST /client/v1/principals` with `principalType=service` (`identity.manage`).
2. Assign ≥1 Role (scopes) and permission sets as needed.
3. Issue a credential: `POST /client/v1/principals/{id}/credentials`.
4. Mint a JWT:

```http
POST /auth/v1/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=<principal_credential_id>
&client_secret=<secret>
```

5. Call `/client/v1`, `/metadata/v1`, `/deploy/v1`, and/or `/ops/v1` with the bearer token.

**Connected Apps** (`/client/v1/integrations`) wrap OAuth client configs for confidential or public clients (including a linked service principal). Bootstrap `API_KEYS` remain break-glass only.

Directory / SCIM remainders: [BP-017](../backlog/BP-017-identity-directory-productionization.md).

---

## Path C — Agents via MCP (or HTTP)

### What customers configure (not “define a server”)

1. Set `FEATURE_FLAGS` to include `agents` (MCP stays dark without it; production often omits until hosted tool execution is complete — [BP-006](../backlog/BP-006-agent-guardrails.md)).
2. Starter AgentSpecs are cloned automatically when `AUTO_SEED` runs (`agents_starter`). Create additional AgentSpecs via Metadata anytime.
3. Create an agent principal (`principalType=agent`), assign Roles / permission sets, issue a credential (same Client identity admin as Path B).
4. Point an external MCP client at `https://<install>/mcp` with that bearer (or call Client/Metadata HTTP directly).

AgentSpecs (`/metadata/v1/agents/playbooks`) hold instructions and allowlists. The **tool catalog** on the product gateway is fixed in Go and maps 1:1 to existing family HTTP paths — MCP invents no capabilities. v1 ships Client/Metadata (+ agent runs) plus builder tools: `invoke_action`, `invoke_skill`, Metadata upsert/list, `org_validate` / `org_deploy` / `pack` / `org_retrieve`, and `install_version` ([BP-064](../backlog/BP-064-install-agent-runtime.md)). Ops **mutate** stays out of MCP.

### Product MCP gateway

| Item | Value |
|---|---|
| Endpoint | `POST /mcp` (Streamable HTTP, **stateless JSON** responses) |
| Catalog helper | `GET /mcp/tools` (authenticated convenience list) |
| SSE listen (`GET /mcp`) | `405` — not offered in v1 |
| Sessions (`DELETE /mcp`) | `405` — stateless; no `Mcp-Session-Id` |
| Auth | Same as family APIs: Majesta One JWT or bootstrap API key; send `One-API-Revision` (inherits the service client pin, [BP-025](../backlog/BP-025-ide-api-version-compatibility.md)) |
| Flag | `FEATURE_FLAGS` must include `agents` |

Supported JSON-RPC methods: `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`.

### Product MCP tools (adapter over family HTTP)

| MCP tool | Maps to | Scope |
|---|---|---|
| `describe_global` / `describe_object` / `query` / `search` / `get_record` / `create_record` / `update_record` | Client describe, query, search, sobjects | `client` |
| `create_agent_run` / `get_agent_run` | Client `/agents/runs` | `client` |
| `invoke_action` | `POST /client/v1/actions/{apiName}` | `client` |
| `invoke_skill` | `POST /client/v1/automations/{apiName}/runs` | `client` + PS `automationAccess` / `canRun` (AgentSpec `allowedSkills` when a playbook is in context) |
| `get_object_metadata` / `list_objects_metadata` | Metadata object GET/list | `metadata` |
| `list_agent_specs` | Metadata AgentSpecs list | `metadata` |
| `upsert_object` / `upsert_field` | Metadata object/field POST+PATCH | `metadata` + `metadata.build` |
| `org_validate` | `POST /deploy/v1/packages/validate-local` | `deploy` + `deploy.promote` |
| `org_deploy` | `POST /deploy/v1/promotions` | `deploy` + `deploy.promote` |
| `pack` | `POST /deploy/v1/packages/pack` (JSON artifact) | `deploy` + `deploy.promote` |
| `org_retrieve` | `GET /deploy/v1/packages/export` SoR (snapshot bundle) | `deploy` |
| `install_version` | `GET /version` | authenticated (not Ops mutate) |

`tools/call` uses the caller JWT. Missing family scope or capability is `401`/`403`. Ops roll/confirm/rollback is not in this catalog.

### Local / generic MCP clients

**Remote HTTP (preferred when the client supports Streamable HTTP):**

```json
{
  "mcpServers": {
    "one": {
      "url": "https://<install>/mcp",
      "headers": {
        "Authorization": "Bearer <one_jwt_or_api_key>",
        "One-API-Revision": "1"
      }
    }
  }
}
```

Mint `<one_jwt_or_api_key>` via Path B (`client_credentials`) for an `agent` principal, or use a short-lived exchanged user JWT for interactive demos.

**Stdio via TypeScript scaffold** (desktop hosts that only speak stdio):

```json
{
  "mcpServers": {
    "one": {
      "command": "npx",
      "args": ["tsx", "src/stdio.ts"],
      "cwd": "/path/to/tools/one-mcp",
      "env": {
        "ONE_BASE_URL": "https://<install>",
        "ONE_CLIENT_ID": "<credential_id>",
        "ONE_CLIENT_SECRET": "<secret>",
        "ONE_PROXY_PRODUCT_TOOLS": "1"
      }
    }
  }
}
```

See [`tools/one-mcp/README.md`](../tools/one-mcp/README.md) for custom vertical tools and optional proxy to the product gateway.

### Protocol check

```bash
# initialize
curl -sS -X POST "https://<install>/mcp" \
  -H "Authorization: Bearer $TOKEN" \
  -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'

# tools/list
curl -sS -X POST "https://<install>/mcp" \
  -H "Authorization: Bearer $TOKEN" \
  -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

---

## Backlog map

| Concern | Item |
|---|---|
| JWT / principals / Connected Apps | [BP-013](../backlog/BP-013-jwt-unified-principals.md) |
| Client Experience OSS kits | [BP-040](../backlog/BP-040-client-experience-oss-kits.md) · [ADR-019](./adr/019-client-experience-oss-kits.md) |
| IDE PKCE / client access | [BP-022](../backlog/BP-022-client-access-ide-device.md) |
| External MCP + hosted agent tool loop | [BP-006](../backlog/BP-006-agent-guardrails.md) · loop spec [hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md) |
| Job-class harness + builder MCP catalog | [BP-064](../backlog/BP-064-install-agent-runtime.md) · [ADR-030](./adr/030-install-agent-runtime.md) |
| Builder MCP + CLI | [builder-connect.md](./builder-connect.md) |
| BYO LLM outbound (hosted agents) | [BP-014](../backlog/BP-014-agent-outbound-integrations.md) — **not** required for Path C |
| Client-callable automations + connector OAuth | [BP-047](../backlog/BP-047-integrations-callable-oauth.md) · [integrations-build-plan.md](./architecture/integrations-build-plan.md) |
| Directory / SCIM | [BP-017](../backlog/BP-017-identity-directory-productionization.md) |
| System alerts / no product mailer | [BP-038](../backlog/BP-038-no-product-mailer-byo-alerts.md) — webhooks + connectors |

---

## System notification intents (BYO email / Slack)

Majesta One does **not** send email. Subscribe Metadata webhooks to system outbox events and forward to your provider:

| Event | Typical use |
|---|---|
| `install.claimed` | Alert ops that day-0 admin exists |
| `principal.created` | Welcome / provisioning follow-up |
| `principal.password_changed` | Security notice |

### Webhook → Slack Incoming Webhook

```http
POST /metadata/v1/webhooks
Authorization: Bearer <token with metadata>
Content-Type: application/json

{
  "apiName": "SystemAlertsSlack",
  "url": "https://hooks.slack.com/services/T.../B.../...",
  "eventTypes": ["install.claimed", "principal.password_changed", "principal.created"],
  "active": true
}
```

Map the Majesta One outbox JSON body in a thin relay if Slack expects `{ "text": "..." }` — or point the webhook at your own HTTPS function that formats the message.

### Webhook → SES / SendGrid (via your relay)

1. Create an allowlisted HTTPS endpoint you control (Lambda / DO Function / Cloudflare Worker).
2. Register it as a Majesta One webhook with the event types above.
3. In the relay, call SES `SendEmail` or SendGrid `/v3/mail/send` with customer-owned credentials.

Do **not** put SMTP or provider API keys into Majesta One env as a product mailer — store them as install secrets / connectors ([BP-014](../backlog/BP-014-agent-outbound-integrations.md)).

### Call a customer automation (Connected App / service principal)

Integrations should invoke automations on the **Client** API under the caller's permission sets — not by faking record writes ([BP-047](../backlog/BP-047-integrations-callable-oauth.md)):

1. Connected App or service principal with Role scope `client` and a PS that grants `automationAccess` for the target apiName (or `allAutomations`).
2. Exchange credentials for a Majesta One JWT (`/auth/v1/token`).
3. Invoke:

```http
POST /client/v1/automations/CreateOpportunity_From_Account/runs
Authorization: Bearer <one-jwt>
Content-Type: application/json

{ "input": { "accountId": "…" } }
```

Async definitions return `202` with a job id; poll `GET /client/v1/automations/runs/{id}`. Run-as is always the invoking principal. Catalog: `GET /client/v1/automations` (callable metadata only — no source).

Outbound OAuth for connectors (authorize / token refresh) is Metadata + `/auth/v1/connectors/*` — see [integrations-build-plan.md](./architecture/integrations-build-plan.md).

### Outbound OAuth connector (install-scoped)

Configure the provider once on Metadata; Majesta One owns authorize/callback/refresh ([BP-047](../backlog/BP-047-integrations-callable-oauth.md)):

1. Put the OAuth client secret in `POST /metadata/v1/secrets` (install-local).
2. Allowlist the authorize and token hosts: `POST /metadata/v1/install/egress`.
3. Create a connector with `authType` `oauth2_authorization_code` or `oauth2_client_credentials` and an `oauthFlow` (tokenUrl, clientId, scopes, optional authorizationUrl + `pkce`).
4. For authorization code: `POST /auth/v1/connectors/{apiName}/authorize` (Metadata + `metadata.build`) → open `authorizationUrl` → provider hits `GET /auth/v1/connectors/callback`.
5. Automations call `ctx.connector` as usual; the host injects/refreshes Bearer tokens. Guest code never sees secrets.

Check status: `GET /metadata/v1/connectors/{apiName}/oauth/status`. Disconnect: `DELETE .../oauth/connection`.

Promote connector **defs + secret ref names** via Deploy; re-bind secrets and re-consent OAuth on each install.

### Automation send via connector

Packages and Deno automations send transactional contact with `ctx.http` / `ctx.connector` against an allowlisted provider host (same SSRF rules as webhooks). Example shape:

1. `POST /metadata/v1/secrets` — store SendGrid API key as `enc:v1:…`
2. `POST /metadata/v1/connectors` — `baseUrl` `https://api.sendgrid.com`, secret ref
3. Automation: `await ctx.connector("SendGrid").fetch("/v3/mail/send", { method: "POST", body: … })`

CRM inbound/outbound email (Message SoR) remains [BP-024](./adr/030-install-agent-runtime.md) Phase C — not this path.

## Related

- [customer-agents.md](./customer-agents.md) — AgentSpec day-one path
- [builder-connect.md](./builder-connect.md) — builder MCP + CLI
- [architecture/agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md)
- [system-alerts-byo-build-plan.md](./architecture/system-alerts-byo-build-plan.md) — BP-038 plan
- [api-families.md](./api-families.md) — Client / Metadata / Deploy / Ops
- [auth-adapters.md](./auth-adapters.md) — Okta / Entra / Keycloak exchange
- [tools/one-mcp](../tools/one-mcp/) — optional customer-hosted TypeScript MCP scaffold
