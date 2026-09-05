# Objects

Static catalog of **managed** objects Majesta One ships. Use this page (and the per-module tables it links) as the public object list.

Runtime schema for **one install** is authenticated `GET /client/v1/describe` and `GET /metadata/v1/objects`. Those endpoints reflect what is enabled and what the caller may see. They are **not** a public website catalog and must not be scraped into docs.

Custom objects and fields you create with the [Metadata API](./api/metadata.md) are not listed here. They live on that install and promote with [Deploy](./api/deploy.md) as `ownership=custom`.

## Always-on (`core`)

Seeded when `AUTO_SEED=1`. Package `core`, `ownership=managed`. Details: [modules/core.md](./modules/core.md).

| Object | Storage | Client records? | Role |
|---|---|---|---|
| **User** | Kernel table `users` | No `/sobjects/User` CRUD. Writes go to `/client/v1/principals`. Describe may return User metadata. | Identity (`principal_type`: user \| service \| agent) |
| **Account** | `records` JSONB | Yes | Organization / party |
| **Contact** | `records` JSONB | Yes | Person; optional `AccountId` |

### Record system fields (flexible objects)

| Field | Rules |
|---|---|
| `CreatedById` | Set on create; client cannot set |
| `LastModifiedById` | Set on create/update; client cannot set |
| `OwnerId` | Optional; omit/`null` stores NULL |
| `CreatedAt` / `UpdatedAt` | Set by the platform |

### Account (standard fields)

| Field | Type | Notes |
|---|---|---|
| Name | text, required, indexed, searchable | |
| AccountNumber | text, indexed, searchable | |
| Website | url, searchable | |
| Industry | text | |
| Phone | phone, searchable | |
| Fax | phone | |
| TickerSymbol | text | |
| Type | picklist | Prospect / Customer / Partner |
| Ownership | picklist | Public / Private / Subsidiary / Other |
| Description | textarea | |
| ParentAccountId | lookup → Account | Hierarchy |
| PrimaryContactId | lookup → Contact | Optional |
| BillingStreet / City / State / PostalCode / Country | text | |
| ShippingStreet / City / State / PostalCode / Country | text | |

### Contact (standard fields)

| Field | Type | Notes |
|---|---|---|
| Salutation | picklist | Mr. / Mrs. / Ms. / Dr. / Prof. |
| FirstName | text, searchable | |
| MiddleName | text | |
| LastName | text, required, indexed, searchable | |
| Email | email, indexed, searchable | |
| JobTitle | text | |
| Department | text | |
| MobilePhone / HomePhone | phone, searchable | |
| Fax | phone | |
| Description | textarea | |
| AccountId | lookup → Account, optional, indexed | May be null |
| MailingStreet / City / State / PostalCode / Country | text | |

### Relationship rules

- Account only, Contact only, or both is supported.
- Account may have zero Contacts.
- Contact may exist without Account.
- Nothing else is enforced between core objects.

You may add **custom** fields on Account / Contact (and User via `users.data`) and custom objects with lookups. You may not edit managed field types or labels through Metadata.

## Optional modules

Enable with `POST /metadata/v1/packages/{name}/enable` (admin + `metadata`). Each module page lists objects, fields, and platform actions.

| Package | Objects (summary) |
|---|---|
| [address](./modules/address.md) | Address |
| [notes](./modules/notes.md) | Note |
| [activities](./modules/activities.md) | Task, Appointment, PhoneCall, Email |
| [lead_marketing](./modules/lead-marketing.md) | Lead, Campaign, MarketingList, MarketingListMember · action `lead.convert` |
| [catalog](./modules/catalog.md) | Product, PriceList, PriceListEntry, Unit, UnitGroup |
| [sales](./modules/sales.md) | Opportunity, OpportunityContactRole, Quote, QuoteLine, Competitor · action `quote.accept` |
| [service](./modules/service.md) | Case, CaseComment, Asset, Entitlement, ServiceContract, ContractLineItem, WorkOrder |
| [crm_bridge](./modules/crm-bridge.md) | Cross-cloud fields only (auto-enabled when sales + service are on) |
| [billing](./modules/billing.md) | Order, OrderLine |
| [healthcare](./modules/healthcare.md) | Patient, Practitioner, CarePlan, Encounter, … |
| [financial_services](./modules/financial-services.md) | Bank, Branch, FinancialProduct, … |
| [retail](./modules/retail.md) | Loyalty*, CustomerAsset, ProductBrand/Category, … |
| [sustainability](./modules/sustainability.md) | Facility, Emission*, Material, … |
| [education](./modules/education.md) | AcademicPeriod, Program, Course*, … |
| [automotive](./modules/automotive.md) | Device*, Deal*, BusinessFacility, … |
| [nonprofit](./modules/nonprofit.md) | Designation, DonorCommitment, Award, … |
| [marketing_events](./modules/marketing-events.md) | MarketingEvent, EventRegistration, … |
| [portals](./modules/portals.md) | Website, WebPage, Forum*, … |
| [project_service](./modules/project-service.md) | Project, ProjectTask, BookableResource, … |

Always-on [agents_starter](./modules/agents-starter.md) clones AgentSpec templates to `ownership=custom`; it is not a record-object pack.

Full enable/disable contract: [modules/README.md](./modules/README.md).

## Related

- [API families](./api-families.md) · [Client](./api/client.md) · [Metadata](./api/metadata.md)
- Contributor storage / performance notes: [data-model.md](./data-model.md) (not a public nav target)
