# Platform actions — build plan

Executable plan for package-gated **platform actions**: product Go verbs on the Client API, callable from customer TypeScript via one SDK method.

**ADR:** [ADR-029](../adr/029-platform-actions.md)  
**Backlog:** [BP-061](../../backlog/BP-061-platform-actions.md)  
**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-data-architecture.md](./agent-data-architecture.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md) · [agent-control-ide.md](./agent-control-ide.md) (IDE consume only, later)  
**Domain agents:** `api-families`, `db-backend-perf`, `authz-security`, `worker-jobs`  
**Related:** [ADR-004](../adr/004-three-api-families.md) · [ADR-014](../adr/014-customer-code-automations.md) · [ADR-020](../adr/020-cdm-managed-packages.md) · [customer-automations-build.md](./customer-automations-build.md) · [api-families.md](../api-families.md) · [automation-sdk.md](../automation-sdk.md) · [BP-049](../../backlog/BP-049-cdm-managed-packages.md) convert follow-up · [BP-046](../../backlog/BP-046-record-merge-dedupe.md) · [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md)

---

## Thesis

> Integrity verbs (Lead convert, merge, later Quote accept) are **product Go**, not customer TypeScript and not locked package TS. They share **one** Client catalog (`/client/v1/actions/{apiName}`) so each new verb is a registry entry, not a new mux family. Customer automations call `ctx.invokeAction({ apiName, input })` as the run-as principal. Availability is the enabled-package set: `lead.convert` exists only when `lead_marketing` is on; `createOpportunity` requires `sales`.

```text
Client / IDE / iPaaS          Customer automation (Deno)
        │                              │
        ▼                              ▼
POST /client/v1/actions/{apiName}    ctx.invokeAction({ apiName, input })
        │                              │
        └──────────► internal/actions.Invoke (Go)
                           │
                           ├─ required packages enabled?
                           ├─ AuthZ object/FLS/sharing as caller / run-as
                           └─ DataEngine tx (syncSafe) or jobs (async)
```

---

## Locked product decisions

| Topic | Choice |
|---|---|
| Family | **Client** only (`scope: client`). Not Metadata, not Deploy, not a fourth family |
| HTTP | `GET/POST /client/v1/actions` + `GET/POST /client/v1/actions/{apiName}` — **no** per-verb routes |
| Registry | Compile-time `ActionDef` on `packages.Module`; no definitions table |
| Gate | All `RequiresPackages` must be `package_installs.enabled`; optional packs gate options |
| apiName | Dotted `noun.verb` (`lead.convert`); not the package name; never `__c` |
| Guest SDK | **One** new frozen method: `invokeAction`. No `ctx.convertLead` |
| AuthZ v1 | Object + field + sharing of caller/run-as on every touched record. No PS `actionAccess` |
| Sync | Convert is sync + `syncSafe` (same Postgres tx as triggering write when called from sync automation) |
| Custom fields | Product copies managed standard fields only; customer wrapping TS copies `__c` |
| Pricing / CPQ | Not platform actions; stay customer automations or future `cpq` |
| Starter clones | Still OK for **example** customer automations; not a substitute for this catalog |

### Error codes (stable)

| HTTP | Code | When |
|---|---|---|
| 404 | `ACTION_NOT_FOUND` | Unknown `apiName` |
| 409 | `PACKAGE_NOT_ENABLED` | Required pack off, or optional pack off for a requested option (`packageName` + optional `option` in details) |
| 403 | `FORBIDDEN` | Missing object/FLS/sharing on a touched record |
| 400 | `VALIDATION_FAILED` | Schema / Lead.Company missing when creating Account / already-invalid state |
| 200 | — | Sync success (`alreadyConverted` may be true) |
| 202 | — | Async accepted (not used by `lead.convert`) |

Do not 404 a **known** action when its package is disabled — 409 tells the customer which pack to enable.

---

## Current state (baseline)

| Surface | Today | Gap |
|---|---|---|
| Managed packages | Objects, field extensions, AgentSpec/Canvas templates | No `Actions` on `packages.Module` |
| Client | CRUD, query, composite, bulk, automation **runs**, agent runs | No generic verb catalog |
| Guest SDK | `get/create/update/delete/query/log` + async `http`/`connector` | Cannot call product verbs without reimplementing them in TS |
| `lead_marketing` | Lead/Campaign/MarketingList objects; Status includes `Converted` | Convert is documented as a follow-up |
| Merge / Quote accept | Open BP-046 / BP-044 | Would otherwise invent sibling routes |

---

## Target types (Phase 1)

`internal/packages`:

```go
type ActionDef struct {
    APIName           string   // "lead.convert"
    Label             string
    Description       string
    RequiresPackages  []string // all must be enabled
    OptionalPackages  []string // options may require these
    Objects           []string // describe/docs; AuthZ still per record
    SyncSafe          bool
    InputJSONSchema   string   // draft-07 object schema, product-owned
    OutputJSONSchema  string
}

type Module struct {
    // existing fields…
    Actions []ActionDef
}
```

`internal/actions` (new):

- `Catalog(ctx, enabledPackages) []Action` — union of registry rows whose `RequiresPackages` ⊆ enabled
- `Describe(apiName)` — def + enabled/disabled reasons
- `Invoke(ctx, principal, apiName, input) (result, error)` — package gate → schema → handler
- Handlers registered by apiName in Go (`lead.convert` → `internal/actions/lead_convert.go` or `internal/dataengine` helper used by the action package)

HTTP handlers stay thin in `internal/httpapi` (same file pattern as automation runs: Client prefix only, not flat `/v1`).

---

## Phases

Execute in order. Each phase is mergeable and test-gated.

### Phase 0 — Docs & contracts (this change set)

**Owner:** architecture / product  
**Status:** Done (this file + ADR-029 + BP-061 + index/playbook/module cross-links)

**Exit:** Agents can implement without re-litigating family, SDK, or package gating.

### Phase 1 — Registry + Client catalog/invoke shell

**Packages:** `internal/packages`, `internal/actions`, `internal/httpapi`, `internal/seed`, `internal/metadata` (enabled-package list already exists), `internal/testutil`  
**Agents:** `api-families`, `db-backend-perf`  
**Playbooks:** api-families + data-architecture  
**Status:** Done

**Deliverables**

1. `ActionDef` + `Module.Actions`; `packages.ActionsByName()` uniqueness check in tests (no duplicate apiNames across modules)
2. `internal/actions` catalog/describe/invoke dispatcher; unknown name → `ACTION_NOT_FOUND`
3. Client routes under `/client/v1` only (no new `/v1` alias):
   - `GET /actions` — enabled-package catalog (no input schemas stripped of huge text if needed; include apiName, label, packages, syncSafe)
   - `GET /actions/{apiName}` — full describe; 409 if required pack disabled
   - `POST /actions/{apiName}` — invoke; empty registry still returns 404/409 correctly
4. Metadata `PackageStatus` grows `actionApiNames` (declared on the module, independent of enable)
5. AuthZ: scope `client`; invoke path wired to caller actor (object checks happen in handlers — Phase 2)

**Exit:** `go test` HTTP: list empty-or-known; POST unknown → 404; POST known-but-pack-disabled → 409 `PACKAGE_NOT_ENABLED`. No Deno yet.

### Phase 2 — `lead.convert` (proving verb)

**Packages:** `internal/actions`, `internal/dataengine`, `internal/seed/module_lead_marketing.go`, `internal/httpapi` tests  
**Agents:** `db-backend-perf`, `api-families`, `authz-security`  
**Playbooks:** data-architecture + authz  
**Status:** Done  
**Depends:** Phase 1; `lead_marketing` objects already seeded

**Deliverables**

1. Register `lead.convert` on `lead_marketing` (`RequiresPackages: ["lead_marketing"]`, `OptionalPackages: ["sales"]`, `SyncSafe: true`)
2. Input/output per ADR-029 §6; JSON Schema on the `ActionDef`
3. Transactional DataEngine: create/update Account, Contact, optional Opportunity; set Lead.Status=`Converted`, AccountId, ContactId
4. AuthZ: Lead read+update; Account/Contact create or update as used; Opportunity create when requested; sharing on existing records
5. Idempotent already-converted path
6. Fail closed: `createOpportunity: true` without `sales` → 409 `PACKAGE_NOT_ENABLED` `{ packageName: "sales", option: "createOpportunity" }`
7. Integration tests (`internal/testutil`): pack off; happy convert; existing Account/Contact; already converted; missing Company; FLS/create deny; Opportunity party ≥1

**Exit:** Enable `lead_marketing`, POST convert as a user with grants → Account+Contact+Lead updated in one tx; disable pack → 409; `sales` off + `createOpportunity` → 409.

### Phase 3 — Guest `invokeAction`

**Packages:** `internal/automation` (deno host RPC, SDK freeze list, unit harness), `tools/automation-sdk/one_automation.d.ts`, `docs/automation-sdk.md`  
**Agents:** `worker-jobs` (+ api-families if HTTP contract docs only)  
**Playbook:** [agent-worker.md](./agent-worker.md) · ADR-014  
**Status:** Done

**Deliverables**

1. Add `invokeAction` to `FrozenSDKMethods`; host RPC dispatches to `actions.Invoke` with run-as actor
2. Sync guest: only `SyncSafe` actions; share the write tx (same cap/depth accounting as nested create/update — action mutations count toward the 50-op / depth-3 caps)
3. Async guest: same Invoke, no outbound from convert
4. Unit harness: mock `invokeAction` + `getCalls({ method: "invokeAction" })`
5. Contract example: customer automation `OnLeadConvert_CopyRegion` that calls `lead.convert` then patches `Account.Region__c` (synthetic customer pack; **not** product seed)

**Exit:** `TestSyncCodeAutomation` style: Contact/Lead trigger → invokeAction → child fields committed; AuthZ deny rolls back; `ctx.invokeAction` in sync with a hypothetical async-only action rejects.

### Phase 4 — Package docs + describe polish

**Packages:** `docs/modules/*`, Metadata package payload if not done in Phase 1, optional `GET /client/v1/describe` pointer  
**Agents:** `api-families`, `db-backend-perf`  
**Status:** Done

**Deliverables**

1. Each module doc lists **Platform actions** (apiName, requires, optional)
2. `GET /metadata/v1/packages/{name}` includes declared actions (schemas optional; names required)
3. Client catalog includes `requiresPackages` / `optionalPackages` so UIs can show “enable sales for Opportunity”

**Exit:** `GET /packages/lead_marketing` shows `lead.convert` even when disabled; Client `GET /actions` omits it until enable.

### Phase 5 — Follow-on verbs (do not invent sibling APIs)

| Verb | When | Notes |
|---|---|---|
| `record.merge` | [BP-046](../../backlog/BP-046-record-merge-dedupe.md) | Register on `core`; implement handler in DataEngine; **must** use this catalog |
| `quote.accept` | [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md) / [ADR-031](../adr/031-billing-managed-module.md) | Optional pack `billing`; syncSafe snapshot Order from QuoteLine — **implemented** |
| Composite subrequest | Optional later | `POST /composite` method `action` / url `/actions/{apiName}` |
| MCP `invoke_action` | [BP-006](../../backlog/BP-006-agent-guardrails.md) | Map to the same Client POST; invent no extra capability |
| Control IDE Operate convert button | [BP-018](../adr/030-install-agent-runtime.md) / [BP-019](../adr/030-install-agent-runtime.md) | JWT Client consumer only; no convert logic in Electron |

### Phase 6 — Optional AuthZ hardening (only if needed)

If object CRUD is insufficient for a verb, a **new ADR** may add PS `actionAccess` (catalog stubs on register, Admin grant) — same pattern as `automationAccess`. Do not add it speculatively in Phases 1–3.

---

## How to add an action later (checklist)

1. Confirm it is an **integrity / multi-object verb**, not customer process or CPQ.
2. Pick owning managed package; list `RequiresPackages` / `OptionalPackages`.
3. Claim a unique dotted `apiName` in ADR-029 reserved table (or this plan’s catalog).
4. Implement Go handler using DataEngine (no guest TS, no `internal` imports from Deno).
5. Register `ActionDef` on the module; bump module version if the pack already shipped.
6. Document under `docs/modules/<package>.md`.
7. HTTP + guest tests: pack off, AuthZ deny, happy path, optional-pack option.
8. Do **not** add a new Client route or a new `ctx` method.

---

## Non-goals (plan scope)

- Managed uneditable TypeScript in packages
- Prompt templates as the convert runtime
- npm allowlists
- Customer-registered actions
- Per-verb HTTP paths
- In-kernel email on convert
- Auto-merge (BP-046 still requires explicit principal action)

---

## Implementation order (agents)

```text
Phase 0 architecture (this PR)
  → Phase 1 api-families + db-backend-perf (registry + HTTP shell)
  → Phase 2 db-backend-perf + authz-security (lead.convert)
  → Phase 3 worker-jobs (invokeAction)
  → Phase 4 docs/catalog polish
  → Phase 5 merge/quote.accept via their BPs
```

IDE agents do not implement convert. They consume `POST /client/v1/actions/lead.convert` when Operate grows a convert control.

## Related

- [ADR-029](../adr/029-platform-actions.md)
- [customer-automations-build.md](./customer-automations-build.md)
- [modules/lead-marketing.md](../modules/lead-marketing.md)
- [customization-authz.md](./customization-authz.md)
