# User identity extension — build plan

Executable plan so a customer can **configure identity**, **provision users (SCIM + JIT)**, and **extend the User object** with custom fields that appear on Client identity APIs and `/scim/v2`.

**Backlog:** [BP-058](../../backlog/BP-058-user-identity-extension.md) (this work) · [BP-003](../../backlog/BP-003-enterprise-auth.md) AuthZ remainders (shipped; do not dump this plan into BP-003) · [BP-017](../../backlog/BP-017-identity-directory-productionization.md) directory Phases 1–4 (shipped) · [BP-037](../../backlog/BP-037-install-claim-customer-sso.md) install auth  
**Playbooks:** [agent-authz.md](./agent-authz.md) · [agent-data-architecture.md](./agent-data-architecture.md) · [agent-api-families.md](./agent-api-families.md)  
**Domain agents:** `authz-security` (primary) · `db-backend-perf` (kernel User metadata + `users.data`) · `api-families` (route ownership)  
**ADRs:** [ADR-002](../adr/002-hybrid-metadata-storage.md) · [ADR-006](../adr/006-jwt-auth.md) · [ADR-008](../adr/008-core-data-model.md) · [ADR-009](../adr/009-record-audit-authz-packaging.md) · [ADR-015](../adr/015-idp-agnostic-social-login.md) · [ADR-016](../adr/016-record-sharing.md) · [ADR-017](../adr/017-canonical-field-types.md)

Say **execute** on this file. Do not start implementation from BP-003’s stale “SCIM remaining” bullet.

---

## Thesis

> Identity **AuthZ** (Roles, permission sets, FLS, sharing) is already the kernel. What B2B customers still cannot do is treat **User as a metadata object**: add `CostCenter__c`, map an IdP claim or SCIM attribute onto it, and read/write that field on the same APIs they already use for directory (`/client/v1/principals`, `/scim/v2/Users`). User stays a kernel table (ADR-008). Custom values live in `users.data` JSONB, described by Metadata — never customer DDL, never `records`.

```text
Customer admin
  → PUT /metadata/v1/install/auth          (SSO / JIT / social / password + provisioning defaults)
  → POST /metadata/v1/fields               (User.CostCenter__c, ownership=custom)
  → Deploy promote field defs (not user rows)

IdP / HRIS
  → /scim/v2/Users  OR  OIDC login + JIT
  → same users row + Role/PS/data-role grants
  → custom attributes ↔ users.data

API consumers
  → GET/PATCH /client/v1/principals/{id}   (standard + custom fields, FLS-stripped)
  → GET /client/v1/sobjects/User/describe  (field catalog; no record object CRUD in v1)
```

---

## How BP-003 relates (close criteria)

BP-003 was **enterprise AuthZ**: object CRUD, FLS, Metadata/Deploy capabilities, record sharing. That work **shipped**.

| Concern | Status | Close on BP-003? |
|---|---|---|
| Object PS + deny-by-default FLS + `ide.*` | Shipped | Yes |
| Metadata / Deploy system capabilities | Shipped | Yes |
| Sharing: OWD, data roles, criteria rules (ADR-016) | Shipped | Yes |
| Manual shares / owner-based rules / queues | Explicitly deferred in ADR-016 | **No** — follow-up, not this plan |
| API key / credential rotation UX | Issue/revoke APIs shipped; schedules/UI polish | **No** — IDE polish |
| SCIM / IdP provisioning | `/scim/v2` Users adapter shipped (BP-017 Phase 4) | **No** — was miscategorized on BP-003 |
| Customer-extendable User + provisioning config | **This plan / BP-058** | **No** |

**Recommendation:** BP-003 is **Mitigated**. Keep ADR-016 deferred sharing as a sentence in BP-003, not an open High-severity gate. Do not reopen BP-003 for User JSONB.

---

## Current state (inventory)

### Identity configuration (how a customer turns identity on)

Shipped on `GET|PUT /metadata/v1/install/auth` (`identity.manage`):

| Field | Meaning |
|---|---|
| `oidcIssuer` / `oidcAudience` / `oidcJwksUri` / `oidcClientId` / secret | Customer SSO |
| `jitProvisionUsers` | Create `users` row on first SSO/social login |
| `jitDefaultRole` | Role for JIT humans (default `StandardUser`) |
| `allowedEmailDomains` | JIT allowlist |
| `socialProviders` | Optional Google/Apple |
| `passwordLoginEnabled` | Local password grant |

Gaps:

- JIT assigns a **Role only** — no default permission sets, no default **data role**
- No IdP **claim → User field** mapping (standard or custom)
- No SCIM-specific defaults when the Majesta One Principal extension is omitted
- One OIDC IdP only (multi-IdP stays BP-037)
- No single “provisioning” document a SI can copy between envs (Deploy does not promote `users`)

### User provisioning (SCIM + JIT)

Shipped:

- Client principals CRUD, freeze, Role/PS assign (`identity.users` / `authz.manage`)
- SCIM-shaped kernel columns (`user_name`, `external_id`, name parts, phone, locale, timezone, title, department)
- `/scim/v2/Users` RFC 7644 adapter + Majesta One Principal extension (`principalType`, `roleApiNames`, `permissionSetApiNames`)
- JIT via install auth + social/OIDC (`CreateSocialUser` + `identity_links`)
- Humans without Majesta One extension default to Role `StandardUser`

Gaps:

- SCIM `enterprise.employeeNumber` is **aliased to** `users.external_id` (federation key collision)
- SCIM cannot set `dataRoleApiName`
- SCIM schema is **static** — customer User fields never appear
- JIT does not copy IdP claims into profile columns (email + display name only)
- Deactivate via SCIM is soft-delete; freeze stays Client-only (keep that)
- Groups / bulk remain post-GA (BP-017)

### User object (the product hole)

`docs/data-model.md` and ADR-008: User is the kernel `users` table, **not** a flexible record object. There is **no** `metadata_objects` row for `User`. Customers cannot:

- `POST /metadata/v1/fields` with `objectApiName=User`
- Describe User on Client/Metadata
- Apply FLS to profile fields
- Store `CostCenter__c` without a product migration
- See custom attributes on principals or SCIM

Account/Contact already use “managed parent + customer fields + JSONB”. User needs the same **metadata contract** without moving rows into `records`.

---

## Locked decisions

| Decision | Choice | Why |
|---|---|---|
| User storage | Stay kernel `users` (ADR-008). **Never** `records` / `records_hv`. | AuthN, credentials, freeze, unique email/`userName` need typed integrity |
| Custom field values | `users.data JSONB NOT NULL DEFAULT '{}'` | Same hybrid as ADR-002; no customer DDL |
| Metadata object | Seed managed `User` in package `core`, `storage_mode=kernel` | Describe + customer fields + FLS catalog without DataEngine CRUD |
| Standard fields | Managed `metadata_fields` with `kernelColumn` mapping to `users.*` | Describe/FLS without duplicating SoR |
| Customer fields | Metadata API on object `User`, `ownership=custom`, values in `users.data` | Same as `Account.Region__c` |
| Client identity SoR | `GET/PATCH /client/v1/principals` remains the write API | `identity.users` capability; do not bypass via record object CRUD |
| Client describe | `GET /client/v1/sobjects/User/describe` (+ Metadata GET object) | Field catalog + FLS-filtered list |
| Client record object CRUD / query | **Out of v1** | Avoid a second write path and a DataEngine kernel adapter |
| SCIM | Same SoR; **dynamic** Majesta One UserCustom schema from customer+managed User fields | Okta/Entra map attributes from `GET /scim/v2/Schemas` |
| Federation key | `users.external_id` = SCIM `externalId` only | Stop aliasing `enterprise.employeeNumber` onto it |
| Employee number | New nullable `users.employee_number` **or** managed field mapped to that column | SCIM enterprise extension stays useful |
| Data role on SCIM | Majesta One Principal `dataRoleApiName` | Sharing hierarchy at provision time |
| JIT / SCIM defaults | Nested `provisioning` on install auth | One customer identity document |
| AuthZ from IdP groups | **Forbidden** | ADR-006 / ADR-015; map group → Role **in the connector** into Majesta One extension |
| Deploy | Promote **User field metadata** (customer); never `users` rows, credentials, or grants | ADR-001 / multi-env |
| FLS | Existing `field_permissions` on object `User`; deny-by-default | Same evaluator as records; strip on principal GET/PATCH |
| Lookups on User | Allowed as customer fields (UUID strings in `data`); no kernel FK | ADR-017 lookup type; v1 no join planner on `users.data` |
| Indexed custom User fields | Optional later (`field_projections` analog on `users`) | v1 list/filter stays standard columns |

Phase 0 writes **ADR-026** (kernel User + `storage_mode=kernel` + `users.data`) and amends ADR-008 (“User as a Client record object follow-up” → “User is a kernel metadata object; not a flexible record object”).

---

## Customer identity configuration (target)

Keep **one** Metadata resource: `GET|PUT /metadata/v1/install/auth`. Add a nested `provisioning` object (omit = today’s behavior).

```http
PUT /metadata/v1/install/auth
{
  "oidcIssuer": "https://login.microsoftonline.com/<tid>/v2.0",
  "oidcAudience": "<app-id>",
  "oidcJwksUri": "https://login.microsoftonline.com/<tid>/discovery/v2.0/keys",
  "jitProvisionUsers": true,
  "jitDefaultRole": "StandardUser",
  "socialProviders": [],
  "passwordLoginEnabled": false,
  "allowedEmailDomains": ["example.com"],
  "provisioning": {
    "jitDefaultPermissionSetApiNames": ["SalesUser"],
    "jitDefaultDataRoleApiName": "SalesRep",
    "scimDefaultRoleApiName": "StandardUser",
    "scimDefaultPermissionSetApiNames": ["SalesUser"],
    "scimDefaultDataRoleApiName": "SalesRep",
    "claimMappings": [
      { "claim": "given_name", "fieldApiName": "GivenName" },
      { "claim": "family_name", "fieldApiName": "FamilyName" },
      { "claim": "department", "fieldApiName": "Department" },
      { "claim": "cost_center", "fieldApiName": "CostCenter__c" }
    ]
  }
}
```

Rules:

- `fieldApiName` must exist on metadata object `User` (managed or customer).
- Claims only apply on **JIT create** (and optional “fill empty on login” — v1 create-only to avoid IdP clobbering SCIM).
- If a SCIM create omits Majesta One `roleApiNames`, use `scimDefaultRoleApiName` (today: hard-coded `StandardUser` for `principalType=user`).
- Permission sets / data role from SCIM extension **override** defaults when present; defaults fill gaps.
- IdP groups still **must not** become Roles. Connector attribute mapping → `roleApiNames` is the supported path.
- `provisioning` is install-local (like OIDC secrets). Document copy-paste between envs; do not Deploy-promote secrets.

SCIM connector setup (already documented, keep):

1. Connected App / service principal, scope `client`
2. PS: `identity.users` (+ `identity.integrations` if provisioning bots) + `authz.manage` for grants
3. `POST /auth/v1/token` client_credentials → Bearer `/scim/v2/*`
4. Prefer `clientAccessMode` `registered_clients` or `open`

---

## User object (target)

### Seeded managed object

```text
metadata_objects:
  apiName: User
  label: User
  storageMode: kernel
  packageName: core
  ownership: managed
```

DataEngine **rejects** `storage_mode=kernel` for record CRUD/query (`ErrUnsupportedStorage`). Identity package is the only writer.

### Standard fields (managed, kernelColumn)

| Field apiName | kernelColumn | SCIM |
|---|---|---|
| `Id` | `id` | `id` |
| `Username` | `user_name` | `userName` |
| `Email` | `email` | `emails[primary].value` |
| `DisplayName` | `display_name` | `displayName` |
| `GivenName` / `FamilyName` | `given_name` / `family_name` | `name.*` |
| `Phone` | `phone_number` | `phoneNumbers[0]` |
| `Locale` / `Timezone` / `Title` / `Department` | matching | core / enterprise |
| `EmployeeNumber` | `employee_number` | `enterprise.employeeNumber` |
| `ExternalId` | `external_id` | `externalId` |
| `IsActive` | `is_active` | `active` (still AND NOT frozen on read) |
| `PrincipalType` | `principal_type` | Majesta One extension |
| `DataRoleId` | `data_role_id` | Majesta One `dataRoleApiName` (resolved) |

Auth-only columns (`frozen_at`, password hashes, `api_key_name`) stay **undescribed** (not Metadata, not SCIM, not principal JSON).

### Customer custom fields

```http
POST /metadata/v1/fields
{
  "objectApiName": "User",
  "apiName": "CostCenter__c",
  "label": "Cost Center",
  "fieldType": "text"
}
```

- Same validators as Account fields (ADR-017 allowlist).
- `EnsureFieldInDataAccessCatalog` — Admin grant, other PSs deny stubs.
- Value at `users.data->>'CostCenter__c'`.
- Deploy snapshot already exports customer fields on managed parents — include `User.*` the same way as `Account.Region__c`.

### Client JSON (principals)

`GET/PATCH /client/v1/principals/{id}`:

- Existing camelCase profile keys unchanged.
- Custom fields at **top level** by apiName (`CostCenter__c`) so SCIM/IdP and Client share names.
- Strip fields the caller cannot FLS-read; reject PATCH to fields they cannot FLS-edit.
- Unknown keys that are not User fields → `400 VALIDATION_ERROR` (do not silently put junk in `data`).

`GET /client/v1/me` (if it returns a user profile today): same FLS strip for the caller’s own User fields; humans may read a product-defined subset of their own standard fields even without `identity.users`. Custom fields still FLS.

---

## SCIM (target)

| Item | v1 behavior |
|---|---|
| `GET /scim/v2/Schemas` | Core User + enterprise + Majesta One Principal + **UserCustom** built from User metadata (managed custom-eligible + customer fields) |
| UserCustom URN | `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom` |
| Attribute names | Field apiNames (`CostCenter__c`) |
| `externalId` | `users.external_id` only |
| `enterprise.employeeNumber` | `users.employee_number` (new column) |
| `enterprise.department` | `users.department` (unchanged) |
| Majesta One Principal | add `dataRoleApiName` |
| Create without extension roles | `provisioning.scimDefaultRoleApiName` or `StandardUser` |
| Filters | Keep v1 `eq`/`and` on `userName`, `externalId`, `emails.value`, `active`, `principalType`. Custom-attribute filter **out of v1** |
| Groups / bulk | Still post-GA (BP-017) |

Okta/Entra: operators map directory attributes → UserCustom apiNames. Majesta One does not ingest IdP group membership as AuthZ.

---

## Phased delivery

### Phase 0 — Docs and backlog hygiene (landed with this plan)

1. This plan (source of truth for execute).
2. ADR-026 on **first implementation PR**: kernel User metadata object, `storage_mode=kernel`, `users.data`, no `records`.
3. ADR-008 consequence updated: User is not a flexible record object; kernel metadata object is BP-058.
4. BP-003 marked **Mitigated**; identity remainders point here / BP-058.
5. BP-017: Phase 5 = this plan; bulk/Groups stay post-GA.
6. Pointers: playbooks, `data-model.md`, `scim-provisioning.md`, architecture README, module map.

**Done:** agents executing identity work open BP-058 + this file, not BP-003. Implementation starts at Phase 1.

### Phase 1 — Kernel User metadata (describe only)

1. `metadata_objects.storage_mode` allow `kernel` (`internal/db.StorageModeKernel`).
2. Metadata create of **customer** objects with `kernel` → reject. Only managed seed may insert `User`.
3. `internal/seed` `InstallCore`: upsert User object + standard field defs (`kernelColumn` on `metadata_fields` — new nullable column).
4. DataEngine: any CRUD/query/upsert/bulk on `kernel` → 400/404 consistent with “not a flexible object”.
5. `GET /metadata/v1/objects/User` + `GET /client/v1/sobjects/User/describe` work; FLS filters describe like other objects.
6. `EnsureObjectInDataAccessCatalog("User")` on seed; object perms: Admin full, others deny **object** CRUD (identity APIs stay capability-gated; User object CRUD is unused in v1 but catalog must exist for FLS).

**Tests:** seed has User; Metadata GET; DataEngine refuses `sobjects/User` POST; describe as IdentityManage vs StandardUser (FLS).

### Phase 2 — `users.data` + customer fields

1. Migration: `users.data jsonb NOT NULL DEFAULT '{}'` (+ `employee_number text` unique-when-set optional).
2. Metadata field create allowed on `User` for customer fields; managed standard fields remain immutable.
3. Validate/coerce writes with existing field-type registry (ADR-017) into JSONB.
4. Principal GET/PATCH merge `data` (FLS).
5. Deploy export/import customer fields on User.

**Tests:** create `CostCenter__c` → PATCH principal → GET returns it; FLS deny strips; uniqueness of `Username`/`ExternalId` unchanged; Deploy snapshot includes the field def, not the value.

### Phase 3 — SCIM dynamic schema + grant/data-role completeness

1. Split `employeeNumber` off `externalId`.
2. Majesta One Principal `dataRoleApiName`.
3. UserCustom schema generated from metadata; PATCH/PUT/POST round-trip custom attrs into `users.data`.
4. Apply `provisioning.scimDefault*` when extension omits grants (Phase 4 may land defaults in the same PR if small).

**Tests:** SCIM create with UserCustom; Schemas lists `CostCenter__c`; employeeNumber does not overwrite externalId; data role assigned; Okta-shaped PATCH `add` path.

### Phase 4 — Provisioning config + JIT mapping

1. Persist `provisioning` JSON on `organization_settings` (or dedicated columns if you prefer typed — JSONB is fine; secrets stay on existing OIDC secret column).
2. PUT install auth validates field apiNames + Role/PS/data-role existence.
3. JIT create: assign default PS + data role; apply `claimMappings` on create only.
4. SCIM create uses the same default grant helper.

**Tests:** JIT with mapped custom claim; JIT without `identity.users` still cannot call principals admin; SCIM without Majesta One extension gets default PS; invalid mapping → 400 on PUT auth.

### Phase 5 — Hardening

1. Audit events: `identity.user.field.patch`, `scim.user.update` include changed field apiNames (not values for secrets — User has no secret fields in v1).
2. Principal list: do not dump full `data` on list (optional `?include=data` or describe-driven sparse field set); GET-by-id remains full FLS view.
3. Docs: Okta/Entra SCIM + custom-attribute recipe on [auth-adapters.md](../auth-adapters.md) + [scim-provisioning.md](./scim-provisioning.md).
4. Control IDE Object Manager listing User as a managed object with “New Field” — **optional**, `control-ide` playbook, not required to close BP-058.

**Done:** audit names, `GET /client/v1/principals?include=data`, and adapter recipes shipped. Control IDE Object Manager remains a separate IDE follow-up.

---

## Non-goals

- Moving User rows into `records`
- `/client/v1/sobjects/User` CRUD or JSON query in this plan
- SCIM Groups as AuthZ (or as data roles)
- SCIM bulk endpoint
- Multi-IdP SSO (BP-037)
- Cross-install user promote / shared directory
- Manual sharing, queues, owner-based rules (ADR-016)
- Email OTP / magic link
- Embedded Keycloak
- Per-customer DDL on `users`
- Treating IdP groups as permission sets
- Credential rotation schedules (IDE)

---

## Packages to touch

| Package | Changes |
|---|---|
| `migrations/` | `users.data`, `users.employee_number`, `metadata_fields.kernel_column`; journal |
| `internal/db` | `StorageModeKernel`; User `Data` map; employee number; install auth `provisioning` |
| `internal/seed` | Managed User object + standard fields; bump `CorePackageVersion` |
| `internal/metadata` | Allow customer fields on kernel User; reject customer kernel objects; describe `kernelColumn` |
| `internal/dataengine` | Refuse kernel storage |
| `internal/authz` | FLS strip/enforce on User field maps |
| `internal/httpapi` | principals JSON; User describe; install auth `provisioning`; SCIM routes already exist |
| `internal/scim` | Dynamic UserCustom schema; employeeNumber split; dataRole |
| `internal/deploy` | Snapshot customer User fields (likely already generic — verify) |
| `internal/identity` | Optional Cognito write-through: do **not** push customer custom fields unless mapped |
| Docs | This file; ADR-026; ADR-008; BP-003/017/058; data-model; playbooks |

IDE (`tools/control-ide`) is **out of backend PRs** unless Phase 5 optional Object Manager work is explicitly in the execute prompt.

---

## Test plan (minimum)

Prefer `internal/testutil` harness.

1. **Describe:** Metadata GET User includes managed Email + customer `CostCenter__c` after field create.
2. **Fence:** `POST /client/v1/sobjects/User` → not a flexible object (4xx).
3. **FLS:** PS without User.CostCenter__c read → GET principal omits key; PATCH → 403.
4. **SCIM:** create with UserCustom + Majesta One dataRole; GET Schemas lists the field; employeeNumber ≠ externalId.
5. **JIT:** install auth mapping `cost_center` → `CostCenter__c`; first SSO creates user with data value + default PS.
6. **Defaults:** SCIM human with only `userName`+email → StandardUser (or configured default) + configured PS.
7. **Deploy:** customer User field in snapshot; users rows absent.
8. **Family gate:** Metadata scope alone cannot PATCH principals; Client without `identity.users` → 403.

---

## Suggested implementation order for agents

1. Phase 0 docs are on this branch (BP status + this file). First implementation PR: ADR-026 + Phase 1 (+ Phase 2 if migrations stay additive).
2. Phase 3 SCIM in a following PR (depends on metadata describe).
3. Phase 4 provisioning/JIT next (depends on fields existing).
4. Phase 5 docs/hardening with the last functional PR.

Do not implement Control IDE UI in the same PR as kernel schema.

---

## Execute checklist (first implementation PR)

- [x] Read this file + ADR-002 / 006 / 008 / 017 + authz and data playbooks
- [x] Do not edit `tools/control-ide/**` unless the execute prompt is cross-plane
- [x] Do not put User in `records`
- [x] Do not use IdP groups as AuthZ
- [x] Phase 3: SCIM UserCustom + employeeNumber split + dataRoleApiName
- [x] Phase 4: install auth `provisioning` + JIT claim maps / default grants
- [x] Update BP-058 status as phases land (Phases 1–5 shipped; item mitigated)
- [x] Phase 5: audit field apiNames, principal list `?include=data`, Okta/Entra SCIM recipes (Control IDE Object Manager optional / not required)
