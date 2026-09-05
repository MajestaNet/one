# Client API (`/client/v1`)

Work on **business data** in this install: records, query, search, bulk, agent runs, and identity assignment.

**Scope:** `client`. Object and field CRUD still require permission sets. Some routes also need a system capability (noted below).

**Does not:** create custom objects or fields; enable managed packages; promote customer metadata; roll product images.

Prefer `/client/v1`. Flat `/v1` is a deprecated alias for the same Client verbs (except where noted). Pin `One-API-Revision` from `GET /version`.

Object catalog for docs: [objects.md](../objects.md). Runtime schema for **this** install: `GET /describe` (authenticated). Do not treat describe as a public website catalog.

## Records and query

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` | `/client/v1/describe` | List objects the caller may see | Publish a public schema; bypass FLS |
| `GET` | `/client/v1/describe/{object}` | Fields, types, and layout hints for one object | Mutate metadata (use [Metadata](./metadata.md)) |
| `POST` | `/client/v1/sobjects/{object}` | Create a record | Upsert on external id (see below) |
| `GET` | `/client/v1/sobjects/{object}/{id}` | Read one record | Return fields the permission set denies |
| `PATCH` | `/client/v1/sobjects/{object}/{id}` | Update fields | Change managed field definitions |
| `DELETE` | `/client/v1/sobjects/{object}/{id}` | Delete one record | Soft-delete on high-volume objects that hard-delete |
| `POST` | `/client/v1/sobjects/{object}/upsert` | Create-or-update by external id | Match on a non-unique field |
| `GET` | `/client/v1/sobjects/{object}/{externalIdField}/{externalId}` | Read by external id | |
| `PATCH` | `/client/v1/sobjects/{object}/{externalIdField}/{externalId}` | Upsert by external id | |
| `DELETE` | `/client/v1/sobjects/{object}/{externalIdField}/{externalId}` | Delete by external id | |
| `POST` | `/client/v1/query` | SQL-native query with keyset pagination | Cross-object ranked find (use `search`) |
| `POST` | `/client/v1/search` | Ranked find on `searchable` fields | Substitute for `query` filters/sorts |
| `POST` | `/client/v1/composite` | Batch several Client ops in one request | Metadata or Deploy verbs |
| `POST` | `/client/v1/bulk/{object}` | Small synchronous create batch | Async ingest (use jobs below) |

User is identity, not a Client record object. There is no `/sobjects/User` CRUD; principals use the identity routes below. Describe may still return User metadata.

## Ingest jobs

Async insert / update / upsert / delete. Datapack apply uses ingest when a step has more than 500 rows; smaller steps stay per-row REST upsert.

| Method | Path | What it does |
|---|---|---|
| `POST` | `/client/v1/jobs/ingest` | Create a job (`object`, `operation`, optional `externalIdField`) |
| `GET` | `/client/v1/jobs/ingest/{id}` | Job status |
| `PUT` | `/client/v1/jobs/ingest/{id}/batches` | Upload a batch |
| `PATCH` | `/client/v1/jobs/ingest/{id}` | Close / complete the job |
| `DELETE` | `/client/v1/jobs/ingest/{id}` | Abort |
| `GET` | `/client/v1/jobs/ingest/{id}/successfulResults` | Per-row successes |
| `GET` | `/client/v1/jobs/ingest/{id}/failedResults` | Per-row failures |

## Events, activity, audit

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` | `/client/v1/events` | Read delivered/available events | Configure subscriptions (Metadata webhooks) |
| `GET` | `/client/v1/events/unpublished` | Events not yet published | |
| `PATCH` | `/client/v1/events/{id}/ack` | Acknowledge an event | |
| `GET` | `/client/v1/activity-feed` | Composed Task / Appointment / PhoneCall / Email for a parent (`parentType`, `parentId`) | Write those objects (use sobjects) |
| `GET` | `/client/v1/audit` | Audit log read | Non-admin callers (admin + `client`) |

## Agents, tools, automations, actions

| Method | Path | What it does | What it does not |
|---|---|---|---|
| `GET` | `/client/v1/agents/playbooks` | Safe summaries of **active** AgentSpecs this caller may run | Create/edit definitions (Metadata) |
| `POST` | `/client/v1/agents/runs` | Start a hosted run (`stream` SSE optional) | Execute Metadata upserts or Deploy `org_*` in v1 |
| `GET` | `/client/v1/agents/runs/{id}` | Run status | |
| `GET` | `/client/v1/agents/runs/{id}/stream` | SSE for an existing run | |
| `POST` | `/client/v1/agents/runs/{id}/approve` | Continue a `require_approval` run (`govern.agents`) | Also enqueue a second `agent.run` on SSE approve |
| `GET` | `/client/v1/automations` | Customer automations this caller may invoke | List Metadata definitions for edit |
| `POST` | `/client/v1/automations/{apiName}/runs` | Invoke as the **caller** | Run with a different principal |
| `GET` | `/client/v1/automations/runs/{id}` | Invoke status | |
| `GET` | `/client/v1/tools` · `/tools/{apiName}` | ToolSpec summaries for Client | Create ToolSpecs (Metadata) |
| `GET` | `/client/v1/actions` | Platform actions enabled on this install | |
| `GET` | `/client/v1/actions/{apiName}` | One action contract | |
| `POST` | `/client/v1/actions/{apiName}` | Invoke a product verb (`lead.convert`, `quote.accept`, …) | Invent a per-verb URL (`/convertLead`) |

Hosted runs execute a v1 subset of MCP names (Client read/write + invoke). Builder Metadata/Deploy tools stay on MCP / family HTTP. Connect: [builder-connect.md](../builder-connect.md).

Agent conversation threads (`/client/v1/agents/conversations`) store principal chat audit. They are not business records and not a public object.

## Identity (assignment on this install)

All of these require `client` plus an identity / AuthZ capability. They do not replace Metadata permission-set **definitions**.

| Method | Path | Capability | What it does |
|---|---|---|---|
| `GET` | `/client/v1/me` | (caller) | Current principal |
| `POST` | `/client/v1/me/password` | (caller) | Change own password |
| `POST` `GET` `PATCH` | `/client/v1/principals` · `/{id}` | `identity.users` | Users, services, agents |
| `POST` | `/client/v1/principals/{id}/freeze` · `/unfreeze` | `identity.users` | Freeze / unfreeze |
| `POST` `GET` | `/client/v1/principals/{id}/credentials` | `identity.users` | Issue / list credentials |
| `POST` | `/client/v1/principals/{id}/credentials/{credId}/revoke` | `identity.users` | Revoke a credential |
| `POST` | `/client/v1/principals/{id}/password` | `identity.users` | Admin password set |
| `GET` `POST` `PATCH` `DELETE` | `/client/v1/roles` · `/{apiName}` | `authz.manage` | Role definitions (family scopes) |
| `POST` | `/client/v1/roles/assign` · `/unassign` | `authz.manage` | Bind Role to principal |
| `POST` | `/client/v1/permissions/assign` · `/unassign` | `authz.manage` | Bind permission set to user |
| `GET` `POST` `PATCH` `DELETE` | `/client/v1/data-roles` · `/{apiName}` | `authz.manage` | Sharing data-role hierarchy |
| `GET` `POST` `PATCH` `DELETE` | `/client/v1/directory-tags` · `/{apiName}` | `identity.users` | Directory tags (SCIM groups-as-tags) |
| `POST` | `/client/v1/directory-tags/assign` · `/unassign` | `identity.users` | Tag membership |
| `GET` `POST` `PATCH` `DELETE` | `/client/v1/integrations` · `/{apiName}` | `identity.integrations` | Connected Apps |
| `POST` | `/client/v1/integrations/{apiName}/secrets/rotate` · `/reveal` | `identity.integrations` | Client secrets |

SCIM 2.0 (`/scim/v2/Users`, …) is a connector adapter over the same identity store — [auth.md](./auth.md#scim).

## MCP adapter

Not a fourth family. `POST /mcp` (and `GET /mcp/tools`) projects Client/Metadata tools for external agent hosts when `FEATURE_FLAGS` includes `agents`. AuthZ is the same as the mapped HTTP path. MCP invents no capabilities. Ops **mutate** is out of MCP v1.

## Related

- [API families overview](../api-families.md) · [Metadata](./metadata.md) · [Auth](./auth.md)
- [Objects](../objects.md) · [Managed modules](../modules/README.md)
- [Builder connect](../builder-connect.md) · [Customer agents](../customer-agents.md)
