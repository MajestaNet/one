# Metadata API (`/metadata/v1`)

Shape **this install’s** model: objects, fields, rules, automations, permission-set definitions, packages, and related config.

**Scope:** `metadata`. Writes that change definitions also need `metadata.build` (or `authz.manage` / `identity.manage` / `govern.network` where noted).

**Does not:** mutate `ownership=managed` artifacts (403); promote to sibling installs (use [Deploy](./deploy.md)); CRUD business records (use [Client](./client.md)); roll product images (use [Ops](./ops.md)).

Writes are always local to the install that receives the request. Prefer `/metadata/v1`. Flat `/v1` aliases exist for core object/field verbs during transition.

New customer artifacts are tagged `ownership=custom`. Managed seed (`core` and enabled modules) stays product-owned.

## Objects, fields, rules

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/metadata/v1/objects` | `metadata` | List objects on this install | Substitute for public [objects](../objects.md) docs |
| `GET` | `/metadata/v1/objects/{apiName}` | `metadata` | One object + fields | |
| `POST` | `/metadata/v1/objects` | `metadata.build` | Create a **custom** object | Create or rename managed objects |
| `PATCH` | `/metadata/v1/objects/{apiName}` | `metadata.build` | Update a custom object | Alter managed definitions |
| `DELETE` | `/metadata/v1/objects/{apiName}` | `metadata.build` | Delete a custom object with no remaining fields/rules | Delete managed objects |
| `POST` | `/metadata/v1/fields` | `metadata.build` | Add a field (`fieldType` must be canonical) | Change managed field types |
| `PATCH` `DELETE` | `/metadata/v1/fields/{object}/{apiName}` | `metadata.build` | Update / delete a custom field | |
| `GET` | `/metadata/v1/field-types` | `metadata` | Allowed types + create rules | |
| `POST` | `/metadata/v1/validation-rules` | `metadata.build` | Create a JSONLogic rule | |

Object delete refuses when fields or validation rules still exist. Field delete removes the matching relationship row when present.

## Automations, permission sets, snapshot

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` `POST` `PATCH` | `/metadata/v1/automations` · `/{apiName}` | `metadata.build` on write | Automation **definitions** | Invoke a run (Client) |
| `GET` `POST` `PATCH` | `/metadata/v1/permissions/sets` · `/{apiName}` | `authz.manage` on write | Permission-set **definitions** (object + field + system) | Assign to a user (Client) |
| `GET` | `/metadata/v1/snapshot` | `metadata` | Export **customer-owned** metadata (includes customer fields on managed objects) | Include managed package internals |

## Packages (managed modules)

Defs ship in the product image. Enable is Metadata, not Deploy. Always-on `core` and `agents_starter` are not enable/disable targets. Catalog: [modules/README.md](../modules/README.md).

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/metadata/v1/packages` | `metadata` | Image catalog + install state | |
| `GET` | `/metadata/v1/packages/{name}` | `metadata` | Detail (version, deps, objects, enabled?) | |
| `POST` | `/metadata/v1/packages/{name}/enable` | `metadata.build` + admin | Idempotent install/migrate of managed defs | Promote via Deploy |
| `POST` | `/metadata/v1/packages/{name}/disable` | `metadata.build` + admin | Soft-disable: stop future upgrades; keep metadata/records | Hard-uninstall |

## Agents, tools, experiences

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/metadata/v1/agents/harnesses` | `metadata` | Job-class harness floors | Drop a floor via PATCH |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/agents/playbooks` · `/{apiName}` | `metadata.build` on write | AgentSpec **definitions** | Start a run (Client) |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/tools` · `/{apiName}` | `metadata.build` on write | ToolSpec definitions | |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/experiences` · `/{apiName}` | `metadata.build` on write | Customer-hosted Experience defs | Serve the Experience from this API as a product SPA |

## Webhooks, connectors, secrets, inference

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` `POST` | `/metadata/v1/webhooks` | `metadata.build` on write | Subscription **config** | Consume events (Client) |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/connectors` · `/{apiName}` | `metadata.build` on write | Outbound connector defs | Pay for x402 from this route |
| `GET` | `/metadata/v1/connectors/{apiName}/oauth/status` | `metadata` | OAuth connection status | |
| `DELETE` | `/metadata/v1/connectors/{apiName}/oauth/connection` | `metadata.build` | Disconnect OAuth | |
| `GET` `POST` `DELETE` | `/metadata/v1/secrets` · `/{apiName}` | `metadata.build` on write | Install secret refs | Echo ciphertext on list |
| `POST` | `/metadata/v1/secrets/{apiName}/rotate` | `metadata.build` | Rotate a secret ref | |
| `GET` `PATCH` | `/metadata/v1/inference/config` | `metadata.build` on write | Install inference routing | Call the model (Client agent runs) |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/inference/providers` · `/{apiName}` | `metadata.build` on write | BYO provider rows | |

Connector authorize/callback live on [Auth](./auth.md).

## Sharing, exposure, install auth, projections, egress

| Method | Path | Capability | What it does | What it does not |
|---|---|---|---|---|
| `GET` | `/metadata/v1/sharing/settings` | `metadata` | Sharing enabled? | |
| `POST` | `/metadata/v1/sharing/enable` | `metadata` | Turn sharing on for the install | |
| `GET` `PATCH` | `/metadata/v1/sharing/objects` · `/{apiName}` | `metadata` | Per-object OWD | |
| `GET` `POST` `PATCH` `DELETE` | `/metadata/v1/sharing/objects/{apiName}/rules` · `/{ruleApiName}` | `metadata` | Criteria sharing rules | |
| `GET` `PUT` | `/metadata/v1/install/exposure` | `govern.network` | Edge/WAF path policy | |
| `POST` | `/metadata/v1/install/exposure/apply` | `govern.network` | Apply exposure | |
| `GET` `PUT` | `/metadata/v1/install/auth` | `identity.manage` | Install auth / SSO config | Claim the install (Auth) |
| `GET` `POST` | `/metadata/v1/install/egress` · `/{hostPattern}` | `govern.network` on write | Egress allowlist | |
| `GET` | `/metadata/v1/projections/{object}` | `metadata` | Query index projections | |
| `POST` | `/metadata/v1/projections/{object}/build` | `metadata.build` | Rebuild projections for an object | |

## Related

- [API families overview](../api-families.md) · [Client](./client.md) · [Deploy](./deploy.md)
- [Objects](../objects.md) · [Managed modules](../modules/README.md) · [Customer customizations](../customer-customizations.md)
