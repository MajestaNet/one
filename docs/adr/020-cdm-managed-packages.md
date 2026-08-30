# ADR-020: Managed domain packages

## Status

Accepted

## Context

Majesta One ships a thin always-on `core` (Account / Contact) per [ADR-008](./008-core-data-model.md) and optional Sales/Service modules per [ADR-011](./011-sales-service-managed-modules.md). Product language already prefers familiar commercial apiNames (`PriceList`, `Product`, `Opportunity`, `Case`).

Customers need richer party attributes and optional Microsoft Common Data Model (CDM) applicationCommon / crmCommon shapes as **enableable managed packages**, without:

- A wholesale import of the full CDM library (~50+ packs / thousands of entities including `operationsCommon` and industry accelerators).
- A Party / Customer base object or polymorphic `parentCustomer` (rejected for join/AuthZ simplicity — [ADR-003](./003-sql-query-engine.md)).
- Forking Majesta One into a second Opportunity/Product graph beside existing `catalog` / `sales` / `service`.

## Decision

### 1. Scope: curated domain packs (CRM + industry)

**1B — applicationCommon + crmCommon spine**

| Package | Role |
|---|---|
| `core` v2+ | Always-on Account + Contact with enriched standard attributes ([§2](#2-party-model-2a)) |
| `address` | Optional multi-address (`Address`) |
| `catalog` v2+ | Evolve existing thin catalog (+ Unit / UnitGroup, Product/PriceList fields) |
| `activities` | Optional Task / Appointment / PhoneCall / Email |
| `lead_marketing` | Optional Lead / Campaign / MarketingList (+ member) |
| `sales` / `service` v2+ | Evolve existing modules toward richer standard attributes; keep Quote-centric spine |
| `crm_bridge` | Unchanged AutoEnable |

**Industry — curated vertical packs** (enableable; see [cdm-industry-packages.md](../architecture/cdm-industry-packages.md)):

| Package | Role |
|---|---|
| `healthcare` | Patient / clinical care spine |
| `financial_services` | Bank, products, claims, KYC |
| `retail` | Loyalty, brands, retail appointments, surveys |
| `sustainability` | Facilities, emissions, materials |
| `education` | Programs, courses, scholarships |
| `automotive` | Devices, deals, facilities |
| `nonprofit` | Donors, awards, disbursements, indicators |
| `marketing_events` | Events, registrations, journeys (not Campaign) |
| `portals` | Website / community portal surfaces |
| `project_service` | Projects, resources, time, expense |

**Still deferred:** Dynamics `operationsCommon` (Finance / SupplyChain / Commerce ERP entity dumps), full FHIR / CDM folder fidelity, CustomerInsightsJourneys telemetry-as-objects. Same enable/registry machinery applies when those land.

Industry packs **must not** re-register Majesta One spine apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …).

### 2. Party model (2A)

- Keep **Account** (org) and **Contact** (person) apiNames.
- Keep **optional** `Contact.AccountId`.
- Expand managed fields additively to the standard attribute set (see [cdm-mapping.md](../architecture/cdm-mapping.md)).
- **Do not** introduce Party/Customer base, Person Accounts, or polymorphic parent customer columns.
- Primary address scalars may live on Account/Contact; multi-address rows live in optional `address`.

### 3. Reuse Majesta One packages

Evolve `catalog` / `sales` / `service` in place. Do **not** register duplicate CDM Opportunity/Product objects under different package names.

### 4. Lead

[ADR-011](./011-sales-service-managed-modules.md) “No Lead” is amended: Lead is **omitted from `sales` and always-on `core`**; Lead (and Campaign / MarketingList) may ship only in optional `lead_marketing`. Lead→Account/Contact/Opportunity convert is platform action `lead.convert` ([ADR-029](./029-platform-actions.md)), not customer TS in this pack and not a `sales` verb.

### 5. Activities

Optional `activities` uses explicit dual regarding lookups (`RegardingAccountId` / `RegardingContactId`). No polymorphic ActivityParty storage in v1. Does not own apiName `Note` (`notes` module remains separate). Email activity is a **record shape**, not a product mailer ([ADR-011](./011-sales-service-managed-modules.md) §10 / BP-038).

**Messages vs Activities:** Keep Task / Appointment / PhoneCall / Email on `flexible` storage. Do **not** promote them to `storage_mode=high_volume`. There is no product `messages` / `Message` channel object ([ADR-032](./032-retire-messages-polymorphic-lookup.md)). Unify work items at Activity Feed composition (`GET /client/v1/activity-feed`), not a shared HV table.

### 6. Import policy

Hand-curated Go `FieldDef` / `ObjectDef` in `internal/seed`, documented in [cdm-mapping.md](../architecture/cdm-mapping.md). Do **not** vendor the Microsoft CDM JSON tree into the product image or run a CDM SDK at boot.

### 7. Quote / Order spine

Order / OrderLine ship in optional `billing` ([ADR-031](./031-billing-managed-module.md)). Invoice stays deferred. Do not dump full CDM sales-folder Order/Invoice into `sales`.

## Consequences

- Customers enable domain packs via existing Metadata `/packages/{name}/enable`.
- Core field expansion is a fleet-wide additive migrate (`CorePackageVersion` bump).
- Industry packs (`healthcare`, `financial_services`, …) attach as optional modules without reopening the party model ([cdm-industry-packages.md](../architecture/cdm-industry-packages.md)).
- Agents follow module contracts + mapping docs; do not invent parallel party objects or collide on managed apiNames.
- Integrity verbs (Lead convert, later merge) register as package-gated **platform actions** ([ADR-029](./029-platform-actions.md)), not as managed TypeScript in the pack.
