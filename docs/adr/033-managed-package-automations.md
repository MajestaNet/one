# ADR-033: Managed package automations (install toggle + description)

## Status

Accepted (plan locked; implementation phased — see [package-automations-build-plan.md](../architecture/package-automations-build-plan.md))

## Context

Managed packages ship **objects/fields** (`ownership=managed`) and **platform actions** (product Go, [ADR-029](./029-platform-actions.md)). Customer process is guest TypeScript in the customer repo ([ADR-014](./014-customer-code-automations.md)). That split stays correct for integrity verbs (`lead.convert`, `quote.accept`) and for customer-specific mapping (`Region__c`).

Installs still need **default process** that ships with a pack (for example: setting Lead.Status to `Converted` actually runs convert; setting Quote.Status to `Accepted` actually runs `quote.accept`). Today those wraps do not exist. Customers must author them from scratch. Control IDE Automations can list/create **custom** automations and toggle `active`, but:

1. Metadata **rejects** mutating `ownership=managed` automations (there are none yet; PATCH `active` would 403).
2. `metadata_automations` has **no persisted functional description**.
3. Package enable does not seed automations. Default-on / customer-off is undefined.
4. The Automations tool treats every row as an editable customer file in the local repo.

Cloning templates to `ownership=custom` (the `agents_starter` pattern) is the wrong fit here: upgrades would not refresh product source/description, Deploy would try to promote package process, and “turn the package automation off” would be indistinguishable from deleting customer code.

## Decision

### 1. Nouns (do not conflate)

| Noun | Author | Ownership | How it lands | Customer may |
|---|---|---|---|---|
| **Platform action** | Product Go | Image registry (no Metadata row) | Package enable unhides Client `/actions` | Invoke; not toggle as metadata |
| **Managed package automation** | Product TypeScript (Deno guest) | `ownership=managed`, `package_name=<pack>` | Seeded on package enable / image migrate | **Toggle `active` only** |
| **Customer automation** | Customer TypeScript | `ownership=custom` | Metadata POST or `one org deploy` | Full definition CRUD + `active` |

Managed package automations are Metadata automations. They are **not** platform actions. They **call** platform actions via `ctx.invokeAction`. They must not reimplement convert / accept / merge in guest TypeScript.

They are **not** `agents_starter` clones. Clone-on-enable remains valid for example **customer** automations; it is not how package process ships.

### 2. Seed on enable; default on; preserve off

`packages.Module` gains `Automations []AutomationDef` next to `Actions`. On `POST /metadata/v1/packages/{name}/enable` and on boot migrate of an already-enabled pack:

1. **Insert** missing managed automation rows with `active=true` (requirement C).
2. **Update** product-owned columns from the image: `label`, `description`, `object_api_name`, `trigger_event`, `runtime`, `execution`, `entry_file`, `source`.
3. **Preserve** install `active`. A customer who disabled the automation keeps it disabled across product upgrades.
4. **New** defs added in a later image version insert `active=true` on that migrate (new process defaults on).
5. **Removed** defs are deactivated (`active=false`) and skipped by dispatch; rows are not hard-deleted in v1.

Soft-disable of the owning package (`POST /packages/{name}/disable`) **stops dispatch** even when `active=true`, matching platform-action `409 PACKAGE_NOT_ENABLED`. Re-enable restores objects/automations and keeps the stored `active` flags.

### 3. Metadata API: toggle is PATCH `active`

Family stays [ADR-004](./004-three-api-families.md) Metadata. Do **not** add a fourth family or a per-automation mux under `/packages`.

| Method | Path | Managed | Custom |
|---|---|---|---|
| `GET` | `/metadata/v1/automations` | List (includes `description`, `ownership`, `packageName`, `active`) | Same |
| `GET` | `/metadata/v1/automations/{apiName}` | Describe one | Same |
| `POST` | `/metadata/v1/automations` | **403** (cannot create managed) | Create |
| `PATCH` | `/metadata/v1/automations/{apiName}` | **`active` only** (`metadata.build` + admin) | Full definition (`metadata.build`) |
| `DELETE` | (none in v1 for managed) | Forbidden | Out of scope unless already supported |

`GET /metadata/v1/packages` and `GET /packages/{name}` list **declared** automations (`automationApiNames` plus per-row `label`, `description`, `active` when installed) the same way they list `actionApiNames` — including when the pack is disabled.

No dedicated `/automations/{apiName}/enable` route. On/off is `{ "active": true\|false }`.

### 4. Persisted description

`metadata_automations.description` is a short functional string (max **500** characters, Unicode). It explains **what the automation does**, not how to implement it.

- **Managed:** required in `AutomationDef`; product-owned; refreshed on migrate; not PATCH-able.
- **Custom:** optional; PATCH-able; recommended in customer YAML.

GET list and GET-by-name always return `description` (empty string when unset).

### 5. Guest runtime (same Deno path)

Managed automations use the existing ADR-014 guest: `runtime=code`, `one:automation` only, no npm. Source lives in the product image (`internal/seed` / `internal/seed/automations/`) and is copied onto the install row. Customers do not pack that source. Deploy **rejects** `ownership=managed` automations in customer bundles (same reject list as managed objects).

`apiName` is PascalCase, unique across the image catalog, never `__c`, never a platform-action dotted `noun.verb`. Customer POST that collides with a registry name is rejected.

### 6. Sync vs async

Each `AutomationDef` declares `execution`. Wraps of `syncSafe` platform actions **must** be `execution: sync` so the triggering write, `invokeAction`, and any standard-field follow-up share one Postgres transaction (fail → full rollback). Async remains valid for non-rollbackable process; it is not the default for integrity wraps.

Caps unchanged: 5s, depth 3, 50 mutations. Guests must no-op when the verb is already satisfied (`alreadyConverted`, Quote already `Accepted`) so nested writes do not loop.

### 7. Control IDE (optional client)

Uplift the **existing** Automations tool and Packages panel to consume these Metadata routes ([ADR-030](./030-install-agent-runtime.md): no new Electron-only chrome, no new mode/tile). The install API is the SoR; builders can do the same via MCP / `one` / curl.

## Consequences

- Enabling `lead_marketing` or `sales` can ship default process without forking Go verbs or customer Git.
- Customers who need `__c` mapping disable the managed wrap and author a custom automation that calls the same `invokeAction`.
- Product upgrades can improve managed source/description without turning disabled process back on.
- ADR-029’s non-goal “locked TS as **integrity verbs**” stands. This ADR adds **managed process automations** that wrap those verbs.

## Explicit non-goals (this ADR)

- Clone-on-enable of package automations to `ownership=custom`
- Customer-editable source of managed automations
- Per-verb `ctx.convertLead` / new Client routes
- Drag-and-drop automation builder as SoT
- Hard-uninstall of managed automation rows
- New Control IDE tiles, modes, or Electron-only APIs
- npm allowlists (still ADR-014)

## Related

- Build plan: [package-automations-build-plan.md](../architecture/package-automations-build-plan.md)
- [ADR-004](./004-three-api-families.md) · [ADR-011](./011-sales-service-managed-modules.md) · [ADR-014](./014-customer-code-automations.md) · [ADR-020](./020-cdm-managed-packages.md) · [ADR-029](./029-platform-actions.md) · [ADR-030](./030-install-agent-runtime.md)
- [BP-069](../../backlog/BP-069-managed-package-automations.md)
