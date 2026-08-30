# Managed modules

Public documentation for Majesta One **managed modules**. These docs describe what each module contains and how to enable it (when optional). Module **source and seed definitions** live in the product image (`internal/seed`, `internal/packages`) and are not published as installable artifacts.

Domain packaging decisions: [ADR-020](../adr/020-cdm-managed-packages.md). Attribute mapping (provenance): [cdm-mapping.md](../architecture/cdm-mapping.md).

## Two extension layers

| Layer | Author | Ownership | How it lands |
|---|---|---|---|
| Customer customizations | Customer admin (Metadata API) | `custom` | Local write; Deploy promote between same-`CUSTOMER_ID` installs |
| Managed modules | Majesta One product | `managed` | Always-on seed and/or admin **enable**; Majesta One upgrades via product image |

Runtime schema remains authenticated (`GET /client/v1/describe`, `GET /metadata/v1/objects/...`). There is no anonymous schema catalog.

**Platform actions** (integrity verbs such as `lead.convert`) are product Go registered on the module, invoked on Client `GET/POST /client/v1/actions/{apiName}`, and gated by `package_installs` — [ADR-029](../adr/029-platform-actions.md). They are not customer automations and are not locked TypeScript in the pack.

## Always-on

Seeded with `AUTO_SEED=1`. Not Metadata enable/disable targets.

| Package | Objects / artifacts |
|---|---|
| [`core`](./core.md) | Account, Contact (+ kernel User) |
| [`agents_starter`](./agents-starter.md) | Clones AdminSetup + MetadataBuilder AgentSpecs (customer-owned); customers may define more AgentSpecs anytime |

## Optional

Admin enable via Metadata API (or Control IDE Modules).

| Package | Enablement | Objects / artifacts |
|---|---|---|
| [`address`](./address.md) | Admin enable | Address (multi-address rows) |
| [`notes`](./notes.md) | Admin enable | Note |
| [`activities`](./activities.md) | Admin enable | Task, Appointment, PhoneCall, Email (`flexible` work items; Activity Feed) |
| [`lead_marketing`](./lead-marketing.md) | Admin enable | Lead, Campaign, MarketingList, MarketingListMember; platform action `lead.convert` |
| [`catalog`](./catalog.md) | Admin enable | Product, PriceList, PriceListEntry, Unit, UnitGroup |
| [`sales`](./sales.md) | Admin enable (requires `catalog`) | Opportunity, OpportunityContactRole, Quote, QuoteLine, Competitor |
| [`service`](./service.md) | Admin enable (requires `catalog`) | Case, CaseComment, Asset, Entitlement, ServiceContract, ContractLineItem, WorkOrder |
| [`crm_bridge`](./crm-bridge.md) | **Auto-enabled** when `sales` + `service` are both on | Cross-cloud fields only (`Case.OpportunityId`) |
| [`billing`](./billing.md) | Admin enable (requires `catalog` + `sales`) | Order, OrderLine; FieldExtension `Quote.OrderId`; `quote.accept` `createOrder` |
| [`healthcare`](./healthcare.md) | Admin enable | Patient, Practitioner, CarePlan, Encounter, Condition, AllergyIntolerance, Observation, MedicationRequest |
| [`financial_services`](./financial-services.md) | Admin enable | Bank, Branch, FinancialProduct, Collateral, Claim, Coverage, Limit, MortgageApplication, KYC |
| [`retail`](./retail.md) | Admin enable | Loyalty*, CustomerAsset, ProductBrand/Category, RetailAppointment, Survey* |
| [`sustainability`](./sustainability.md) | Admin enable | Facility, Emission*, Material, FuelType, BusinessTravel, EmployeeCommuting |
| [`education`](./education.md) | Admin enable | AcademicPeriod, Program, Course*, Scholarship, Internship, TestScore, … |
| [`automotive`](./automotive.md) | Admin enable | Device*, Deal*, BusinessFacility, DeviceInspection |
| [`nonprofit`](./nonprofit.md) | Admin enable | Designation, DonorCommitment, Award, Disbursement, Indicator, Budget, … |
| [`marketing_events`](./marketing-events.md) | Admin enable | MarketingEvent, EventRegistration, CustomerJourney, … |
| [`portals`](./portals.md) | Admin enable | Website, WebPage, Forum*, Blog*, Idea, Poll, … |
| [`project_service`](./project-service.md) | Admin enable | Project, ProjectTask, BookableResource, TimeEntry, Expense, Estimate |

Sales/Service architecture: [sales-service-data-model.md](../architecture/sales-service-data-model.md), [ADR-011](../adr/011-sales-service-managed-modules.md). Industry packs: [cdm-industry-packages.md](../architecture/cdm-industry-packages.md). Seed: `internal/seed/module_*.go`.

## Enable / disable API

Requires scope `metadata` and admin. Always-on packages (`core`, `agents_starter`) are not enable/disable targets.

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/metadata/v1/packages` | Catalog from product image + install state |
| `GET` | `/metadata/v1/packages/{name}` | Detail (version, deps, objects, enabled?) |
| `POST` | `/metadata/v1/packages/{name}/enable` | Idempotent install/migrate of managed defs |
| `POST` | `/metadata/v1/packages/{name}/disable` | Soft-disable: stop future upgrades; keep metadata/records |

Hard uninstall (delete managed metadata and records) is out of scope.

## Upgrade semantics

1. Majesta One ships new module defs in a product release.
2. Ops rolls the product image on the install.
3. On API boot, **always-on** packages and **enabled** optional modules re-run additive migrate and bump `package_installs.version`.
4. Soft-disabled modules are not re-migrated.
5. Prefer additive field/object changes; breaking managed changes need an explicit product policy ([BP-007](../adr/020-cdm-managed-packages.md)).

## AuthZ

- On enable (and when new managed objects appear), **every** permission set receives an object data-access stub: system **Admin** gets full CRUD; other sets get deny stubs (visible in the Metadata `dataAccess` section).
- New fields get a `field_permissions` stub on every permission set (Admin = read+edit; others = deny).
- Non-admin principals still need explicit grants on a permission set (object + optional field permissions) before Client CRUD succeeds.
- Client CRUD enforces object AuthZ and field-level security (`field_permissions`; deny-by-default; OR-union across assigned PSs).

## Related

- [Core data model](../data-model.md)
- [Attribute mapping](../architecture/cdm-mapping.md)
- [Industry packages](../architecture/cdm-industry-packages.md)
- [Sales & Service data model](../architecture/sales-service-data-model.md)
- [ADR-011: Sales & Service managed modules](../adr/011-sales-service-managed-modules.md)
- [ADR-020: Managed domain packages](../adr/020-cdm-managed-packages.md)
- [API families](../api-families.md)
- [Customer customizations](../customer-customizations.md)
- [Customer agents](../customer-agents.md)

## Retired

| Package | Notes |
|---|---|
| [`messages`](./messages.md) | Retired ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md)). Not in the enable catalog. |
