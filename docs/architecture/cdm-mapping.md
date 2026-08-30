# CDM ↔ Majesta One mapping

Pinned mapping from Microsoft Common Data Model (CDM) applicationCommon / crmCommon attributes to Majesta One managed metadata. Source policy: [ADR-020](../adr/020-cdm-managed-packages.md).

**CDM pin:** [microsoft/CDM](https://github.com/microsoft/CDM) `schemaDocuments/core/applicationCommon` (+ `foundationCommon/crmCommon`) — latest unversioned `*.cdm.json` entity docs. Majesta One does **not** vendor CDM JSON into the product image; seed defs are hand-authored in `internal/seed`.

Majesta One field apiNames use PascalCase. Types use Majesta One canonical field types ([ADR-017](../adr/017-canonical-field-types.md)).

## Package DAG

| Majesta One package | CDM folders (curated) | Enablement |
|---|---|---|
| `core` | applicationCommon Account, Contact (attributes) | Always-on |
| `address` | applicationCommon Address | Optional |
| `catalog` | foundationCommon Product, PriceList, PriceListItem→PriceListEntry, Unit, UnitGroup | Optional |
| `activities` | applicationCommon Task, Appointment, PhoneCall, Email | Optional |
| `lead_marketing` | crmCommon Lead, Campaign, MarketingList, MarketingListMember | Optional |
| `sales` | crmCommon/sales (+ Opportunity/Quote spine already Majesta One) | Optional |
| `service` | crmCommon/service (+ Case spine already Majesta One) | Optional |
| `billing` | crmCommon/sales Order + OrderProduct (curated; Majesta One `OrderLine`) | Optional |

## `core` — Account

| Majesta One field | Type | CDM attribute (approx.) | Notes |
|---|---|---|---|
| Name | text, required, indexed | `name` | Existing |
| Website | url | `webSiteURL` | Existing |
| Industry | text | `industry` / industryCode display | Existing |
| Phone | phone | `telephone1` | Existing |
| Type | picklist | `customerTypeCode` / account category | Prospect, Customer, Partner (Majesta One values) |
| AccountNumber | text, indexed | `accountNumber` | New |
| Description | textarea | `description` | New |
| Fax | phone | `fax` | New |
| TickerSymbol | text | `tickerSymbol` | New |
| Ownership | picklist | `ownershipCode` | Public, Private, Subsidiary, Other |
| ParentAccountId | lookup→Account | `parentAccountId` | Hierarchy |
| PrimaryContactId | lookup→Contact | `primaryContactId` | Optional |
| BillingStreet | text | address1_line1 | Primary billing scalar |
| BillingCity | text | address1_city | |
| BillingState | text | address1_stateorprovince | |
| BillingPostalCode | text | address1_postalcode | |
| BillingCountry | text | address1_country | |
| ShippingStreet | text | address2_line1 | Primary shipping scalar |
| ShippingCity | text | address2_city | |
| ShippingState | text | address2_stateorprovince | |
| ShippingPostalCode | text | address2_postalcode | |
| ShippingCountry | text | address2_country | |

## `core` — Contact

| Majesta One field | Type | CDM attribute (approx.) | Notes |
|---|---|---|---|
| FirstName | text | `firstName` | Existing |
| LastName | text, required, indexed | `lastName` | Existing |
| Email | email, indexed | `emailAddress1` | Existing |
| AccountId | lookup→Account, **optional** | `parentCustomerId` when Account | Majesta One keeps optional org link; no polymorphic parent |
| MiddleName | text | `middleName` | New |
| Salutation | picklist | `salutation` | Mr., Mrs., Ms., Dr., Prof. |
| JobTitle | text | `jobTitle` | New |
| Department | text | `department` | New |
| MobilePhone | phone | `mobilePhone` | New |
| HomePhone | phone | `telephone2` | New |
| Fax | phone | `fax` | New |
| Description | textarea | `description` | New |
| MailingStreet | text | address1_line1 | |
| MailingCity | text | address1_city | |
| MailingState | text | address1_stateorprovince | |
| MailingPostalCode | text | address1_postalcode | |
| MailingCountry | text | address1_country | |

## `address` — Address

| Majesta One field | Type | Notes |
|---|---|---|
| Name | text, required | Label for the address row |
| Street | text | |
| City | text, indexed | |
| State | text | |
| PostalCode | text, indexed | |
| Country | text | |
| AddressType | picklist | Billing, Shipping, Mailing, Other |
| AccountId | lookup→Account | Optional parent |
| ContactId | lookup→Contact | Optional parent |
| IsPrimary | boolean | |

## `catalog` additions

| Object | Fields |
|---|---|
| Product | + Description, ProductURL, QuantityUnitOfMeasureId (lookup→Unit), StockKeepingUnit, ProductType (Good / Service / Subscription) |
| PriceList | (existing Name, IsActive, IsStandard, CurrencyCode) + Description, BeginDate, EndDate |
| PriceListEntry | + UnitId (lookup→Unit) |
| UnitGroup | Name (required), Description |
| Unit | Name (required), UnitGroupId (master_detail), Quantity, IsBaseUnit |

## `activities`

Shared regarding pattern on each activity object: `Subject` (required), `Status`, `Priority`, `ScheduledStart`, `ScheduledEnd`, `Description`, `RegardingAccountId`, `RegardingContactId`.

| Object | Extra fields |
|---|---|
| Task | DueDate, PercentComplete |
| Appointment | Location |
| PhoneCall | PhoneNumber, Direction (Inbound/Outbound) |
| Email | FromAddress, ToAddress, CcAddress |

**Storage / timeline:** Activities stay `flexible` CRM work items. There is no product `messages` channel object ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md)). Operate/Client compose Activities via Activity Feed — do not promote Activities to `high_volume`.

## `lead_marketing`

| Object | Key fields |
|---|---|
| Lead | FirstName, LastName (required), Company, Email, Phone, Status, Source, AccountId, ContactId, Description |
| Campaign | Name (required), Status, Type, StartDate, EndDate, Description |
| MarketingList | Name (required), Type (Static/Dynamic), MemberType (Account/Contact/Lead), Description |
| MarketingListMember | MarketingListId (required), AccountId, ContactId, LeadId |

## `sales` / `service` v2 attribute parity

Additive fields only; Quote-centric spine unchanged.

**Opportunity:** Description, NextStep, Probability, LeadSource, Type (New Business / Existing / Renewal)  
**Quote:** Description, Subtotal, TotalAmount, BillingName, CurrencyCode, TaxAmount, ShippingAmount, AcceptedAt, Billing/Shipping address scalars  
**QuoteLine:** Description, LineNumber, UnitId  
**Competitor** (new object): Name (required), Website, Strengths, Weaknesses, AccountId  
**Case:** Description, Type, Reason, IsEscalated  
**Asset:** SerialNumber, InstallDate, PurchaseDate, Description  
**WorkOrder:** Description, Priority, ContactId  

## `billing` — curated CDM Order (not full sales-folder dump)

CDM entity `sales/Order` (CDS `SalesOrder`) is “Quote that has been accepted.” Majesta One keeps PascalCase names and Account/Contact party links (no `customerId`). Line object is `OrderLine` (ADR-011), not CDM `OrderProduct`.

| Majesta One field | Type | CDM attribute (approx.) | Notes |
|---|---|---|---|
| Order.Name | text, required | `name` | |
| Order.OrderNumber | autonumber | `orderNumber` | Format `ORD-{00000}` |
| Order.Status | picklist | `stateCode` / `statusCode` | Draft / Activated / Fulfilled / Cancelled — no Invoiced until Invoice ships |
| Order.AccountId / ContactId | lookup | denormalized `accountId` / `contactId` | No polymorphic `customerId` |
| Order.QuoteId | lookup | `quoteId` | Required on `quote.accept` |
| Order.OpportunityId | lookup | `opportunityId` | Optional |
| Order.PriceListId | lookup | `priceLevelId` | Majesta One PriceList |
| Order.CurrencyCode | text | `transactionCurrencyId` display | No FX `*Base` twins |
| Order.Subtotal | currency | `totalLineItemAmount` | Copied, not calculated |
| Order.TaxAmount | currency | `totalTax` | Copied, not a tax engine |
| Order.ShippingAmount | currency | `freightAmount` | Copied |
| Order.TotalAmount | currency | `totalAmount` | Copied |
| Order.BillingStreet… | text | `billToLine1` / city / state / postal / country | Same scalar pattern as Account |
| Order.ShippingStreet… | text | `shipTo*` | |
| Order.EffectiveDate | date | `submitDate` / request delivery | |
| Order.ActivatedAt | datetime | (Majesta One) | Set by `quote.accept` |
| OrderLine.QuoteLineId | lookup | `quoteDetailId` | |
| OrderLine.ProductId | lookup | `productId` | Required |
| OrderLine.UnitId | lookup | `uoMId` | Optional |
| OrderLine.Quantity | number | `quantity` | |
| OrderLine.UnitPrice | currency | `pricePerUnit` | |
| OrderLine.LineNumber | number | `lineItemNumber` | |
| Quote.OrderId | lookup | (reverse of `quoteId`) | `billing` FieldExtension |

## Explicit non-mappings (this delivery)

- Party / Customer base entity  
- `parentCustomerId` + type discriminator  
- ActivityParty polymorphic  
- Invoice / Payment (billing v2)  
- Knowledge / EntitlementProcess  
- Full CDM sales-folder dump (`customerId`, `*Base` currency, BPF, CDS owner teams)  
- Industry / operationsCommon packs (1C)
