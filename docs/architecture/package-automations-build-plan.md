# Managed package automations — build plan

Executable plan so a customer install can **seed, describe, and toggle** product-owned automations that ship with managed packages, and so the existing Control IDE Automations tool can operate those same Metadata APIs.

**ADR:** [ADR-033](../adr/033-managed-package-automations.md)  
**Backlog:** [BP-069](../../backlog/BP-069-managed-package-automations.md)  
**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-data-architecture.md](./agent-data-architecture.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md) · [agent-deploy.md](./agent-deploy.md) · [agent-control-ide.md](./agent-control-ide.md) (consume only; no new chrome)  
**Domain agents:** `api-families` then `db-backend-perf` then `worker-jobs` then `deploy-ops`; IDE phase is `control-ide` after HTTP is green  
**Related:** [ADR-014](../adr/014-customer-code-automations.md) · [ADR-029](../adr/029-platform-actions.md) · [customer-automations-build.md](./customer-automations-build.md) · [platform-actions-build-plan.md](./platform-actions-build-plan.md) · [customer-customizations.md](../customer-customizations.md)

---

## Thesis

> Packages ship **process wrappers** as `ownership=managed` Metadata automations: product Deno TypeScript that calls platform actions. Enable of the pack turns them **on**. The install may turn each one **off** with `PATCH /metadata/v1/automations/{apiName}` `{ "active": false }` without forking source. Every automation (managed and custom) persists a short **description**. Integrity verbs stay product Go. Control IDE is a JWT client of those routes, not a second SoR.

```text
Image registry (packages.Module.Automations)
        │
        ▼  enable / boot migrate
metadata_automations  ownership=managed  active=true (insert) | preserve active (upgrade)
        │
        ├─ PATCH {active}          Metadata (admin + metadata.build)
        ├─ GET list / GET by name  includes description
        └─ afterWrite dispatch     skip if !active OR owning pack soft-disabled
                 │
                 ▼
           Deno guest → ctx.invokeAction → platform action (same tx when execution=sync)
```

---

## Locked product decisions

| Topic | Choice |
|---|---|
| Artifact | Metadata automation row, `ownership=managed`, `package_name=<pack>` |
| Not | Platform action; not `agents_starter` clone; not customer Git |
| Default on enable | `active=true` on **insert** |
| Upgrade | Refresh product columns; **preserve `active`** |
| Off | `PATCH` `{ "active": false }` only; no `/enable` alias |
| Soft-disable pack | Do not fire, even if `active=true` |
| Description | `metadata_automations.description` text, max 500 chars; required on managed; optional on custom |
| Guest | Existing Deno + `one:automation`; source in product image |
| Integrity | `ctx.invokeAction` only; no reimplementation of `lead.convert` / `quote.accept` |
| Sync wraps | `execution: sync` when calling `syncSafe` actions |
| Family | Metadata definitions; Client still invokes runs |
| IDE | Existing Automations + Packages panels only ([ADR-030](../adr/030-install-agent-runtime.md)) |
| Deploy | Reject managed automations in customer packs |

### Error codes (stable)

| HTTP | Code | When |
|---|---|---|
| 403 | `FORBIDDEN` | PATCH managed fields other than `active`; POST `ownership=managed`; non-admin toggling managed `active` |
| 400 | `VALIDATION_ERROR` | Description > 500 chars; invalid `active`; customer `apiName` collides with registry |
| 404 | `NOT_FOUND` | Unknown automation apiName |
| 409 | `PACKAGE_NOT_ENABLED` | Dispatch/run of a managed automation whose pack is soft-disabled (Client run path); Metadata PATCH `active` on a row whose pack is disabled is still allowed so the install can pre-set intent |

Do not 404 a known managed automation because the pack is disabled — GET still returns the row with `active` and package state.

---

## Current state (baseline)

| Surface | Today | Gap |
|---|---|---|
| `metadata_automations` | `label`, `active`, `ownership`, `package_name`, code columns | No `description` |
| Metadata PATCH | `AssertCustomerMutable` → 403 on managed | Cannot toggle package process |
| Package enable | `SyncObjectManaged` + CanvasSpecs + AgentSpec **clone** | No `Module.Automations` |
| Dispatch | `active=true` only | No pack-enabled gate |
| GET `/packages` | `actionApiNames`, objects | No declared automations |
| Customer automations | Full CRUD + IDE Monaco | No description field; list ignores managed |
| IDE Automations | List, create custom, local file edit, `active` checkbox | Checkbox 403s on managed; no description; no managed vs custom |
| IDE Packages | Enable/disable pack, object names | No automation list/toggles |
| Deploy | Rejects managed `packageName` / ownership on objects | Must reject managed automations explicitly |
| ADR-029 / BP-061 | Non-goal “locked TS as integrity verbs” | Amend: process wraps allowed |

---

## Target types

`internal/packages`:

```go
type AutomationDef struct {
    APIName       string // PascalCase, unique across image, never __c, never noun.verb
    Label         string
    Description   string // ≤500 chars, functional
    ObjectAPIName string
    TriggerEvent  string // create | update | delete | write
    Runtime       string // code
    Execution     string // sync | async
    EntryFile     string // virtual product path, e.g. seed/automations/Lead_ConvertOnConvertedStatus.ts
    Source        string // TypeScript body (or loaded from embed)
}

type Module struct {
    // existing fields…
    Automations []AutomationDef
}
```

`internal/metadata`:

- `SyncAutomationManaged(ctx, def, packageName) error` — insert `active=true`; update product columns; never overwrite `active` on existing rows
- Deactivate registry-removed apiNames for that package
- `EnsureAutomationInAccessCatalog` on insert (same as customer create)

HTTP:

- `GET /metadata/v1/automations/{apiName}` (missing today; list-only)
- `PATCH` managed: allowlisted keys = `{active}`
- `PackageStatus.Automations []PackageAutomationStatus` with `apiName`, `label`, `description`, `active`, `installed`

---

## v1 proving automations

Ship two process wraps that already have platform actions. Bump `LeadMarketingPackageVersion` and `SalesPackageVersion`.

| apiName | Package | Trigger | Execution | Description (functional) |
|---|---|---|---|---|
| `Lead_ConvertOnConvertedStatus` | `lead_marketing` | Lead `update` | sync | When Lead.Status becomes Converted, runs `lead.convert` in the same transaction. Does not copy customer custom fields. |
| `Quote_AcceptOnStatusAccepted` | `sales` | Quote `update` | sync | When Quote.Status becomes Accepted, runs `quote.accept` in the same transaction. Does not copy customer custom fields. |

Guest rules:

1. No-op if already converted / already accepted (`alreadyConverted` or Status already terminal before this write).
2. `invokeAction` only; no duplicate Account/Contact/Order logic.
3. Optional `createOpportunity` / `createOrder` follow the action’s existing package options (do not invent defaults that contradict ADR-029 / ADR-031).
4. Unit harness tests under `internal/seed/automations` or `internal/automation` using mock `invokeAction`.

Customers who need `__c` mapping: disable the managed wrap, author a custom sync automation that calls the same action then `updateRecord`.

---

## Phases

Execute in order. Each phase is mergeable and test-gated.

### Phase 0 — Docs & contracts (this change set)

**Owner:** architecture / product  
**Status:** Done (this file + ADR-033 + BP-069 + index/playbook/module cross-links)

**Exit:** Agents can implement without re-litigating clone-vs-managed, PATCH-only toggle, description, or IDE chrome.

### Phase 1 — Description column + GET/PATCH for custom

**Packages:** `migrations/` (next kernel number after `0061`), `internal/httpapi`, `internal/customerrepo`, `internal/deploy` (snapshot field)  
**Agents:** `api-families`, `db-backend-perf`  
**Playbooks:** api-families + data-architecture

**Deliverables**

1. `ALTER TABLE metadata_automations ADD COLUMN description text NOT NULL DEFAULT ''` + check `char_length(description) <= 500` (or enforce in Go; prefer Go + comment if CHECK is painful with Unicode).
2. List, create, patch, and snapshot include `description`.
3. `GET /metadata/v1/automations/{apiName}`.
4. Customer YAML `metadata/automations/*.yaml` may include `description`.
5. Custom PATCH `description` still requires `AssertCustomerMutable`.

**Exit:** HTTP tests: POST custom with description; PATCH description; description >500 → 400; GET-by-name 200.

### Phase 2 — Registry + SyncAutomationManaged + enable default on

**Packages:** `internal/packages`, `internal/seed`, `internal/metadata`, `internal/db` (catalog stubs)  
**Agents:** `db-backend-perf`  
**Playbook:** data-architecture

**Deliverables**

1. `AutomationDef` + `Module.Automations`; uniqueness test across modules (and vs `ActionDef` apiNames).
2. `syncModuleDefs` pass: `SyncAutomationManaged`.
3. Enable `lead_marketing` with an empty Automations slice still works; tests use a fixture module **or** the proving defs in Phase 4 — if proving defs land here, keep guests no-op until Phase 4.
4. First insert `active=true`; second sync preserves `active=false`.
5. Registry-removed apiName → `active=false`.
6. Customer POST colliding with a registry apiName → 400.

**Exit:** `go test` seed: enable pack inserts managed row active; disable-via-SQL-then-sync keeps `active=false` while source/description refresh.

### Phase 3 — Metadata toggle of managed `active` + package catalog

**Packages:** `internal/httpapi`, `internal/metadata`, `internal/seed` (`PackageStatus`)  
**Agents:** `api-families`, `authz-security`  
**Playbooks:** api-families + authz

**Deliverables**

1. PATCH managed `{ "active": bool }` allowed; any other key → 403.
2. Capability: `metadata.build` **and** admin (same bar as package enable). Non-admin → 403.
3. `PackageStatus` includes declared automations + install `active` when the row exists.
4. GET automation still works when pack is soft-disabled.

**Exit:** HTTP: admin PATCH active false → 200; admin PATCH label → 403; non-admin PATCH active → 403; GET `/packages/sales` lists automation apiNames after enable.

### Phase 4 — Dispatch + proving guests

**Packages:** `internal/dataengine`, `internal/automation`, `internal/seed/module_lead_marketing.go`, `internal/seed/module_sales.go`, `internal/seed/automations/` (embed), `internal/worker` (async path already skips `active=false` via job payload — still skip pack-disabled)  
**Agents:** `worker-jobs`, `db-backend-perf`  
**Playbooks:** worker + data-architecture

**Deliverables**

1. `dispatchAutomations`: skip `!active`; skip managed rows whose `package_name` is not `package_installs.enabled`.
2. Client `POST /automations/{apiName}/runs` on inactive → not found/inactive (existing); on managed + pack disabled → 409 `PACKAGE_NOT_ENABLED`.
3. Embed `Lead_ConvertOnConvertedStatus` and `Quote_AcceptOnStatusAccepted`; bump package versions.
4. Integration: Lead update Status=Converted with managed auto **on** → converted Account/Contact (and Opportunity only if `sales` on and input says so — default follow ADR-029 `createOpportunity` false unless the wrap explicitly passes true; **lock: wrap does not pass `createOpportunity` unless Lead already has enough party data and `sales` is enabled — prefer `false` in v1**).
5. Same test with `active=false` → Status stays Converted **without** Account create (field-only write). Document that inconsistency: that is why the wrap exists and why custom wraps replace it.
6. Fail-after-convert in a **custom** sync wrap still rolls back (existing test). Managed wrap itself is syncSafe-only.

**Exit:** `TestSyncInvokeActionConvertAndRollback` remains green; new tests for enable-default-on, disable-skip, pack-soft-disable-skip, alreadyConverted no-op.

### Phase 5 — Deploy / snapshot / baseline

**Packages:** `internal/deploy`, `internal/customerrepo`  
**Agents:** `deploy-ops`  
**Playbook:** agent-deploy

**Deliverables**

1. Validate/apply reject `ownership=managed` automations in customer packs (same pattern as managed objects).
2. Snapshot export omits managed automations (already customer-owned snapshot — confirm).
3. `.one/baseline` may list managed automation **references** as read-only; packing them is rejected.
4. Customer description round-trips in pack YAML.

**Exit:** Validate fixture with a managed automation → issue; custom description pack/unpack round-trip.

### Phase 6 — Control IDE Automations + Packages consume

**Packages:** `tools/control-ide/**` only  
**Agents:** `control-ide`  
**Playbooks:** agent-control-ide · [ide-demo-client-uplift-build-plan.md](./ide-demo-client-uplift-build-plan.md)  
**Depends:** Phases 1–3 HTTP (Phase 4 proving rows make the panel useful)

**Deliverables** (existing tools only — **no new tile/mode**)

1. **Automations panel**
   - List shows `description`, `ownership` badge (`managed` / `custom`), `packageName`, `active`.
   - Filter: All / Package / Custom.
   - Managed select: description + trigger + execution + **read-only** source (from GET); hide Save file / Open YAML as write; Active checkbox calls PATCH `{active}`.
   - Custom: keep Monaco + local repo edit; add description on create form and YAML mirror.
   - Run now: still Client invoke; surface 409 pack-disabled.
2. **Packages panel**
   - Expanded pack lists declared automations (label + description + active) when enabled; declared catalog when disabled.
   - Toggle active without leaving Packages (same PATCH).
3. Vitest: managed row toggle fires PATCH; managed editor is not writable; description visible; Packages list includes automations.

**Exit:** `make test-ide` green. Do not add license/Operate-CRM/update-CDN chrome.

### Phase 7 — Module docs + builder MCP honesty

**Packages:** `docs/modules/*`, `docs/api/metadata.md`, `internal/mcp` only if customize catalog lacks Metadata PATCH  
**Agents:** `api-families` (MCP) + docs in the same change set as HTTP if MCP already proxies family HTTP

**Deliverables**

1. Module pages (`lead-marketing.md`, `sales.md`, [modules/README.md](../modules/README.md)) list packaged automations + how to disable.
2. Metadata catalog documents PATCH-active-on-managed exception to “does not mutate managed”.
3. Builder customize skill / MCP: PATCH automation `active` is an existing family path — add a tool **only** if the gateway does not already allow Metadata PATCH. Do not invent MCP-only verbs.

**Exit:** Docs match HTTP. MCP test only if a new tool is added.

---

## Cross-cutting enforcement checklist

| Control | Where |
|---|---|
| Managed source immutable | PATCH allowlist `active` only |
| Default on | `SyncAutomationManaged` insert |
| Upgrade preserves off | never UPDATE `active` on existing managed rows |
| Pack off ⇒ no fire | dispatch + Client run |
| Description length | validate ≤500 |
| No integrity in TS | proving guests call `invokeAction` only; code review + tests |
| Deploy boundary | reject managed automations in customer packs |
| AuthZ run | existing PS `automationAccess` / `allAutomations`; Admin stub on insert |
| IDE | JWT client of Metadata/Client; no Electron-only API |

---

## Suggested tests (minimum)

| Test | Asserts |
|---|---|
| `TestAutomationDescriptionRoundTrip` | POST/PATCH/GET custom description |
| `TestSyncAutomationManagedPreservesActive` | insert true; customer false survives source refresh |
| `TestPatchManagedAutomationActiveOnly` | active 200; label 403; non-admin 403 |
| `TestManagedAutomationSkippedWhenInactive` | Lead Status=Converted does not convert |
| `TestManagedAutomationSkippedWhenPackageDisabled` | row exists; dispatch skip |
| `TestLeadConvertOnConvertedStatusHappy` | Status→Converted runs convert (sync rollback of trigger if convert fails) |
| `TestQuoteAcceptOnStatusAcceptedIdempotent` | second update no-ops |
| `TestDeployRejectsManagedAutomation` | validate issue |
| IDE Vitest | toggle PATCH; read-only managed; description in list |

---

## Non-goals (plan scope)

- Shipping a large library of industry-pack automations in v1 (registry is ready; proving two is enough)
- Clone-to-custom package automations
- `actionAccess` on permission sets (still ADR-029 v1)
- Changing default guest execution for **customer** automations (still async)
- New IDE modes or in-IDE agent host
- Auto-copy of `__c` fields on convert/accept

---

## Implementation order for agents

1. Phase 1+2+3 in one backend track if small enough; do not mix IDE.
2. Phase 4 proving guests after toggle HTTP is green (otherwise tests cannot disable).
3. Phase 5 Deploy with Phase 2 uniqueness.
4. Phase 6 IDE after GET-by-name + PATCH-active-on-managed exist.
5. Phase 7 docs can land with Phase 4 module version bumps.

Paste-ready prompts should cite this file, ADR-033, allowed packages in [module-map.md](./module-map.md), and forbid `tools/control-ide` until Phase 6.
