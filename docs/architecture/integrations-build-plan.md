# Integrations — callable automations + outbound OAuth

**Active plan** for [BP-047](../../backlog/BP-047-integrations-callable-oauth.md): expose customer automations on the **Client** API so Connected Apps / service principals can invoke them under Majesta One JWT AuthZ, and extend outbound connectors with **install-scoped OAuth** flow specs + token lifecycle so customers do not re-implement OAuth in guest code.

**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md) · [agent-data-architecture.md](./agent-data-architecture.md) · [agent-deploy.md](./agent-deploy.md)  
**Domain agents:** `api-families`, `authz-security`, `worker-jobs`, `db-backend-perf`, `deploy-ops`  
**Related:** [ADR-004](../adr/004-three-api-families.md) · [ADR-006](../adr/006-jwt-auth.md) · [ADR-014](../adr/014-customer-code-automations.md) · [customer-automations-build.md](./customer-automations-build.md) · [outbound-otel-build-plan.md](./outbound-otel-build-plan.md) · [BP-009](../../backlog/BP-009-no-in-kernel-language.md) · [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) · [BP-033](../../backlog/BP-033-customer-runtime-isolation.md)

---

## Thesis

> Integrations authenticate to Majesta One with platform auth (Connected App / service principal → JWT). They **invoke** customer automations on `/client/v1` as the calling principal. Async automations call external APIs through **Metadata-owned connectors** whose credentials are either static secret refs or **platform-managed OAuth** (authorize, store, refresh) — guest Deno never sees tokens or implements OAuth.

```text
External integration
  → /auth/v1 (client_credentials | PKCE)
  → POST /client/v1/automations/{apiName}/runs
  → jobs.automation.run as caller (PS automationAccess)
  → Deno guest (optional ctx.connector)
       → Go host
       → static Bearer | OAuth access token
       → HTTPS (egress allowlist)
```

---

## Locked decisions

| Decision | Choice |
|---|---|
| Invoke ownership | **Client** runtime; definitions stay **Metadata** (ADR-004) |
| Auth for invoke | Majesta One JWT + scope `client` + PS `automationAccess` / `allAutomations` / admin; **run-as = caller** (ADR-014) |
| Invoke routes | `GET /client/v1/automations` (callable catalog); `POST /client/v1/automations/{apiName}/runs`; `GET /client/v1/automations/runs/{id}` |
| Sync vs async | Async → `202` + job id; sync → inline result with sync caps; sync **forbids** outbound |
| Trigger | `action=manual` on API invoke; record triggers unchanged; `runAsPrincipalId` only for **schedule** |
| Outbound OAuth scope | **Install-scoped** only (v1); per-user OAuth deferred |
| Auth types | `static_bearer` (existing), `oauth2_client_credentials`, `oauth2_authorization_code` |
| OAuth config | Metadata connector + `oauth_flow` JSON (URLs, client id, scopes, PKCE); secrets/tokens install-local via `secretcrypt` |
| Callbacks | `POST /auth/v1/connectors/{apiName}/authorize` (authenticated); `GET /auth/v1/connectors/callback` (public) |
| Token injection | Host sets `Authorization` unconditionally for authenticated connectors; guest cannot override |
| Deploy | Promote connector defs + OAuth **non-secret** fields + secret **ref names**; never ciphertext/tokens; re-bind + re-consent per install |
| Debug | Thin job status now; project into BP-033 `ExecutionRun` when that lands |

---

## Phases

### Phase 0 — Docs + backlog alignment

This file, BP-047, API-family / connect / module-map cross-links, BP-009/014 pointers.

**Status:** Done

**Exit:** Agents can implement without re-litigating ownership.

### Phase 1 — Client automation invoke

**Packages:** `internal/httpapi`, `internal/worker`, `internal/authz`, `internal/testutil`  
**Agents:** `api-families`, `authz-security`, `worker-jobs`  
**Status:** Done

1. Callable catalog (active defs the actor may run; no `source`)
2. `POST .../runs` enqueues `automation.run` with `action=manual`, `actorId=caller`, optional `input` → `ctx.trigger.data`
3. `GET .../runs/{id}` returns job status for caller-owned (or admin) runs
4. Worker: merge manual `input`; apply `runAsPrincipalId` **only** when `action=schedule`
5. Integration tests: AuthZ matrix, scope, inactive/missing, cross-principal GET deny

**Exit:** Connected App / service JWT can invoke an automation and observe completion under caller AuthZ.

### Phase 2 — Connector auth model

**Packages:** `migrations/`, `internal/db`, `internal/httpapi/outbound_routes.go`  
**Agents:** `db-backend-perf`, `api-families`, `authz-security`  
**Status:** Done

1. Migration: `auth_type`, `oauth_flow` on `install_connectors`; `install_connector_oauth_tokens`; `install_connector_oauth_states`
2. Metadata CRUD accepts/returns auth type + secret-free OAuth flow; token status endpoints
3. Default existing rows to `static_bearer`

**Exit:** Customers can configure OAuth flow specs without storing tokens in Git.

### Phase 3 — Platform OAuth runtime

**Packages:** `internal/connectoroauth`, `internal/httpapi` (auth), `internal/automation/outbound.go`  
**Agents:** `authz-security`, `worker-jobs`, `api-families`  
**Status:** Done

1. Authorize start (state + optional PKCE) + provider callback → encrypted token upsert
2. Client-credentials: mint/refresh on demand in host
3. Host `ConnectorCall` selects auth type; refresh-on-expiry; fail closed when disconnected
4. Token URL + authorize URL HTTPS + egress allowlist; redirect-disabled HTTP client

**Exit:** Async automation `ctx.connector` works with OAuth connectors without guest OAuth code.

### Phase 4 — Deploy + DX closeout

**Packages:** `internal/deploy`, `internal/metadata`, `internal/customerrepo`, docs  
**Agents:** `deploy-ops`  
**Status:** Done (including Govern catalog follow-through)

1. Snapshot/pack/apply connector defs (refs + OAuth flow; no secrets/tokens)
2. On OAuth client/token URL change at target: drop tokens → require reconnect
3. Docs recipes (generic OAuth2; provider-shaped examples)
4. Govern → Integrations includes a catalog-driven connector wizard, secret-ref binding, required egress hosts, OAuth connect/status, and installed-connector management

**Exit:** Promote between installs preserves defs; operators re-bind secrets and re-consent OAuth.

---

## Security checklist

| Front | Control |
|---|---|
| Invoke AuthZ | Client scope + `AssertCanRunAutomation`; run-as = caller |
| Catalog leak | No source / entry internals on Client list |
| Run isolation | GET run filtered by `actorId` (admin exempt) |
| SSRF | HTTPS; no redirects; private IP block; egress allowlist |
| Secrets | `enc:v1`; Metadata `hasSecret` / token status only |
| Guest escape | Deno deny-net; no token in `ctx` |
| OAuth CSRF | Hashed single-use state; PKCE when enabled; config hash bind |
| Deploy | Never promote ciphertext or OAuth tokens |

---

## Non-goals

- Per-user / end-user OAuth connections
- BYO LLM / AgentSpec model keys (BP-006 / BP-014 remainder)
- Sync automation outbound (ADR-014)
- Customer `fetch` / npm inside Deno
- Inbound provider-webhook signature framework
- Full BP-033 `ExecutionRun` projection (thin job status until then)

---

## Verification

- `go test` Client invoke AuthZ matrix + worker manual input / schedule run-as
- OAuth: authorize → callback → encrypted store; refresh; allowlist fail-closed; Deploy excludes tokens
- Docs: `customer-connect.md` recipe for Connected App → automation run
