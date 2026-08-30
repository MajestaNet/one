# Customization AuthZ — principal parity + capabilities

How Majesta One enforces customer customizations for `user` | `service` | `agent` principals across Client / Metadata / Deploy.

See [BP-006](../../backlog/BP-006-agent-guardrails.md), [BP-003](../../backlog/BP-003-enterprise-auth.md), [ADR-006](../adr/006-jwt-auth.md), [ADR-009](../adr/009-record-audit-authz-packaging.md), [customer-customizations.md](../customer-customizations.md).

**This document is the AuthZ contract (shipped).** Hosted `/client/v1/agents/runs` **execution** of tools as the run actor is a different remaining item: [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md). Do not reopen principal parity or system capabilities here to land the loop.

## Product rules

1. Principals are distinct `users` rows (`principal_type`: user / service / agent).
2. Any principal may perform customer-allowed customizations **iff** Roles (family scopes) + permission sets / system capabilities allow it.
3. Go enforces **capability then ownership** on every mutate path.
4. One AuthZ model across Client / Metadata / Deploy — no agent-only parallel permission system.

## Enforce order (mutate)

```text
AuthN (API key | Majesta One JWT | OIDC)
  → family scope (client | metadata | deploy | ops)
  → system capability (unless admin)
  → ownership (AssertCustomerMutable / Deploy managed rejection)
  → handler
```

Admin (`users.is_admin` or Role `admin` scope) implies all system capabilities.

**Platform actions** ([ADR-029](../adr/029-platform-actions.md)) are Client data verbs, not Metadata customizations. v1 AuthZ is object/FLS/sharing of the caller (or automation run-as) on every record the action touches — there is no `actionAccess` catalog yet. Package enablement is a separate 409 gate (`PACKAGE_NOT_ENABLED`).

## Capability catalog

Stored on `permission_sets.system_permissions` (JSONB string array). Each set may enable **one or many** flags; assignees OR-union across sets. Full table: [system-capabilities.md](./system-capabilities.md).

| Capability | Gates |
|---|---|
| `identity.users` | User principals / user credentials / freeze |
| `identity.integrations` | Connected Apps, service/agent credentials, secret rotate/reveal |
| `authz.manage` | Define permission sets (Metadata) **and** Role/PS assignment (Client) |
| `metadata.build` | Customer customize + package enable/disable |
| `deploy.promote` | Deploy mutate paths |
| `govern.network` | Install exposure / WAF / `clientAccessMode` / `requireDeviceCert` |
| `govern.agents` | Agent run approve |
| `govern.audit` | Reserved for audit export |
| `debug.read` / `debug.trace` | Customer debug objects / TraceFlags |
| `ide.*` | Control IDE mode + tool chrome (see [system-capabilities.md](./system-capabilities.md)) |

Legacy aliases (`identity.manage`, `metadata.customize`, `metadata.packages`, `metadata.assignAuthz`, `metadata.network`, `agents.approve`) still accepted.

## Principal + credential admin (Client)

Identity admin lives on **Client** (`scope: client`) under the split identity / authz capabilities above.

1. `POST /client/v1/principals` — create principals (`identity.users`)
2. `POST /client/v1/roles/assign` + `POST /client/v1/permissions/assign` — `authz.manage`
3. `POST /client/v1/principals/{id}/credentials` — Majesta One `client_secret`
4. `GET|POST|PATCH|DELETE /client/v1/integrations` — Connected Apps (`identity.integrations`); optional `allowedCidrs`
5. Humans: Google/Apple social broker (ADR-015) or customer OIDC → Majesta One JWT (`azp`, typically `one.controlIde`); legacy Cognito exchange still supported as adapter
6. Machines: `POST /auth/v1/token` client credentials → Majesta One JWT
7. Devices: `POST /client/v1/devices/enroll` — required when `requireDeviceCert=true`

Also: `GET /client/v1/roles`, list/get/patch principals, list/revoke credentials. Managed OOTB integration: `one.controlIde` (public PKCE).

Permission-set **definitions** stay on Metadata:

| Method | Path | Notes |
|---|---|---|
| `GET` | `/metadata/v1/permissions/sets` | Headers; `?include=dataAccess` (also loads `automationAccess`) |
| `GET` | `/metadata/v1/permissions/sets/{apiName}` | Includes `dataAccess` + `automationAccess` |
| `POST` | `/metadata/v1/permissions/sets` | `authz.manage` |
| `PATCH` | `/metadata/v1/permissions/sets/{apiName}` | `authz.manage`; `systemPermissions` / `systemPermissionsAdd` / `systemPermissionsRemove` (live for assignees); `automationAccess`; system PS immutable |

### Data access section

Each permission set exposes a **data access** matrix:

```json
{
  "dataAccess": {
    "objectPermissions": [
      {"objectApiName": "Account", "canCreate": true, "canRead": true, "canUpdate": true, "canDelete": false, "viewAll": false, "modifyAll": false}
    ],
    "fieldPermissions": [
      {"objectApiName": "Account", "fieldApiName": "Name", "canRead": true, "canEdit": true, "configured": false}
    ]
  }
}
```

Flat `objectPermissions` / `fieldPermissions` keys are also returned for compatibility.

### Automation access section (ADR-014 Phase 1)

```json
{
  "automationAccess": {
    "allAutomations": false,
    "automations": [
      {"apiName": "CreateOpp_On_Account", "canRun": true}
    ]
  }
}
```

| Event | Effect |
|---|---|
| New automation | Every permission set gets an `automation_permissions` stub — Admin / `allAutomations` = `canRun` true; others deny |
| New permission set | Stubs backfilled for all existing automations |
| Runtime | `AutomationAuthz.ActorCanRunAutomation` — OR-union of `canRun` across assigned PSs; `IsAdmin` or any PS `allAutomations` grants all |

Run-as for execution remains the starter principal (Phase 4+); Phase 1 only ships the grant catalog + enforce helper.

**Catalog sync (automatic):**

| Event | Effect |
|---|---|
| New object (customer create or managed module enable) | Every permission set gets an `object_permissions` stub — Admin = full CRUD; others = deny |
| New field | Every permission set gets a `field_permissions` stub — Admin = read+edit; others = deny |
| New permission set | Object + field stubs backfilled for all existing objects/fields |
| Metadata GET | Field matrix is stored rows (`configured: true`); missing rows (should not happen after freeze) default deny |

Field FLS enforcement is **deny-by-default** with OR-union across assigned sets ([authz-ide-fls-build-plan.md](./authz-ide-fls-build-plan.md)). A one-shot freeze migration materializes prior allow-if-absent effective access as explicit grants.

## Install exposure (network opening without AWS admin)

Non-AWS customer admins (including managed installs) set desired edge posture via Metadata:

- `GET|PUT /metadata/v1/install/exposure`
- `POST /metadata/v1/install/exposure/apply`

Modes per family (`client` / `auth` / `metadata` / `deploy` / `ops`): `public` | `allowlist` | `blocked` (+ CIDRs). Product edge reconcile uses the in-process Memory roller (`local`). AWS WAFv2 reconcile is a community option under [`sdk/aws/edge`](../../sdk/aws/README.md).

## Seeded Roles / permission sets

| Role | Scopes |
|---|---|
| `SystemAdmin` | client, metadata, deploy, ops, admin |
| `StandardUser` | client |
| `MetadataDeveloper` | client, metadata |
| `DeployBot` | deploy |

| Permission set | `system_permissions` |
|---|---|
| `Admin` | all canonical capabilities |
| `ManageUsers` | `identity.users` |
| `ManageIntegrations` | `identity.integrations` |
| `ManagePermissions` | `authz.manage` |
| `Build` | `metadata.build` |
| `Deploy` | `deploy.promote` |
| `Govern` | `govern.network`, `govern.agents` |
| `Operate` | `[]` (object/field grants only) |
| `MetadataCustomize` (legacy) | `metadata.build` |
| `DeployPromote` (legacy) | `deploy.promote` |
| `AgentsApprove` (legacy) | `govern.agents` |
| `IdentityManage` (legacy) | `identity.users`, `identity.integrations` |

Assignment path: create principal → assign Role(s) → assign permission set(s) → issue credential / use env API key → call APIs as that `sub`.

## Env API keys → distinct service principals

Each `API_KEYS` entry name binds to a `users` row with `api_key_name` (migration `0015`). Resolve/mint loads that service user (not a shared `DEFAULT_OWNER_ID` collapse). Grants are synced from key scopes (`+admin` → SystemAdmin + Admin PS; metadata → MetadataDeveloper + MetadataCustomize; deploy → DeployBot + DeployPromote).

## AgentSpecs + runs + MCP

- AgentSpecs (`agent_playbooks`) hold `instructions`, allowlists, ownership; mutate only when `ownership=custom` (`metadata.build`).
- Create run stores `actor_id` from JWT/`sub` and copies playbook `instructions` into run input when absent.
- Approve reloads playbook `allowed_tools` / `object_scopes` (does not reset allowlists).
- Worker validates known tools; empty `objectScopes` means all objects until tools execute real Client writes (then Client object/FLS AuthZ applies as the run actor).
- Approvers need `govern.agents` (or admin).
- MCP (`POST /mcp`, Streamable HTTP / stateless JSON) uses the same enforce order as mapped Client/Metadata tools ([ADR-010](../adr/010-customer-agentic-platform.md), [customer-agents.md](../customer-agents.md), [customer-connect.md](../customer-connect.md)).

## Code map

| Concern | Package |
|---|---|
| Capability check | `internal/authz/system_perms.go` |
| PS system_permissions load | `internal/db/system_perms.go` |
| Metadata / Client approve / Deploy gates | `internal/httpapi/metadata_routes.go`, `deploy_routes.go` |
| Principal / credential admin (Client) | `internal/httpapi/principal_routes.go`, `internal/db/users.go`, `credentials.go` |
| Social login broker / IdP exchange | `internal/authlogin` (planned), `auth_routes.go`, `identity_links` |
| Optional Cognito write-through | `internal/identity`, `identity_links` |
| Token exchange | `internal/httpapi/auth_routes.go` |
| Install exposure / WAF | `internal/httpapi/exposure_routes.go`, `internal/edge` |
| API key principals | `internal/authz/apikey.go`, `internal/db/users.go` |
| Agent worker allowlists | `internal/worker/process.go` |
| Hosted agent tool loop | `internal/agentloop` — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md); executor is `mcp.CallTool` as reconstructed run Actor |
| MCP adapter | `internal/mcp`, `internal/httpapi/mcp_routes.go` |
| Platform actions | `internal/actions` + Client `/client/v1/actions` ([ADR-029](../adr/029-platform-actions.md) · [BP-061](../../backlog/BP-061-platform-actions.md)) |
