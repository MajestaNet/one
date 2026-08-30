# Industry packages (Majesta One)

Curated **industry / vertical** managed modules customers can enable. Complements core CRM packs. Decision: [ADR-020](../adr/020-cdm-managed-packages.md). Attribute provenance map: [cdm-mapping.md](./cdm-mapping.md).

**Policy:** Hand-curated objects (not wholesale external schema import). No duplicate Majesta One apiNames (`Account`, `Contact`, `Product`, `Case`, `Opportunity`, `Campaign`, `Asset`, …). Industry packs depend on `core` only unless noted. ERP-scale operations packs remain deferred.

## Package DAG

```text
core
├── healthcare
├── financial_services
├── retail
├── sustainability
├── education
├── automotive
├── nonprofit
├── marketing_events        (events/journeys; Campaign stays in lead_marketing)
├── portals
└── project_service
```

| Majesta One package | Objects (v1) |
|---|---|
| `healthcare` | Patient, Practitioner, CarePlan, Encounter, Condition, AllergyIntolerance, Observation, MedicationRequest |
| `financial_services` | Bank, Branch, FinancialProduct, Collateral, Claim, Coverage, Limit, MortgageApplication, KYC |
| `retail` | LoyaltyProgram, LoyaltyAccount, LoyaltyCard, CustomerAsset, ProductBrand, ProductCategory, RetailAppointment, SurveyDefinition, SurveyResponse |
| `sustainability` | Facility, Emission, EmissionFactor, EmissionsSource, Material, FuelType, BusinessTravel, EmployeeCommuting |
| `education` | AcademicPeriod, Program, Course, CourseSection, PreviousEducation, Scholarship, Internship, TestScore, AreaOfStudy |
| `automotive` | Device, DeviceBrand, DeviceModel, Deal, DealCustomer, DealDevice, BusinessFacility, DeviceInspection |
| `nonprofit` | DonorCommitment, Designation, Award, Disbursement, BenefitRecipient, DeliveryFramework, Indicator, Budget |
| `marketing_events` | MarketingEvent, EventRegistration, EventVendor, AttendeePass, CustomerJourney, Building, Hotel |
| `portals` | Website, WebPage, WebRole, Invitation, Forum, ForumThread, ForumPost, Blog, BlogPost, Idea, Poll |
| `project_service` | Project, ProjectTask, BookableResource, Characteristic, TimeEntry, Expense, Estimate |

## Naming notes

- `MarketingEvent` (not `Event`) avoids generic collision and clarifies CRM event vs calendar Appointment.
- `FinancialProduct` / `CustomerAsset` avoid colliding with `catalog.Product` and `service.Asset`.
- `Patient` links optionally to `Contact`; practitioners may link to `Contact` as well.

## Non-goals

- Full clinical FHIR resource graph / every industry entity from upstream schemas
- Dynamics-style operations ERP entity dumps
- Journey telemetry event storm as first-class objects
- Replacing `lead_marketing.Campaign` or `sales`/`service` spines
