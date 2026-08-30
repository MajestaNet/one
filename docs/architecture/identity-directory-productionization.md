# Identity directory productionization (users, roles, permission sets)

Plan to harden Client identity admin and related AuthZ so customers can provision users, manage lifecycle, define Roles, and assign multiple permission sets — without breaking ADR-006 / ADR-009 packaging.

**Backlog:** [BP-017](../../backlog/BP-017-identity-directory-productionization.md)  
**Playbooks:** [agent-authz.md](./agent-authz.md), [agent-api-families.md](./agent-api-families.md)  
**Domain agent:** `authz-security` (+ `api-families` for route ownership)

## Current state (inventory)

Already shipped (BP-013 P1):

| Surface | What works |
|---|---|
| Client principals | `POST/GET/PATCH /client/v1/principals` (+ credentials) gated by `identity.manage` |
| Role assignment | `GET /client/v1/roles`, `POST /client/v1/roles/assign` |
| PS assignment | `POST /client/v1/permissions/assign` → `user_permission_sets` (many-to-many) |
| PS definitions | Metadata `GET/POST/PATCH /metadata/v1/permissions/sets` (`metadata.assignAuthz`) |
| AuthN gate | Inactive principals rejected; ≥1 Role required to mint/resolve JWT |
| Seed | Roles `SystemAdmin` / `StandardUser` / `MetadataDeveloper` / `DeployBot`; PS `Admin` + capability sets including `IdentityManage` |

Profile fields today: `email`, `displayName`, `principalType`, `isActive`, `isAdmin` only. No freeze model. No customer Role CRUD. Create principal does **not** assign a Role in the same request.

## Product decisions (locked for this plan)

| Decision | Choice | Rationale |
|---|---|---|
| SCIM | **SCIM-shaped Client APIs first**; full `/scim/v2` protocol later | ADR-006 still treats protocol SCIM as non-GA; customers need IdP-mappable fields now |
| Role cardinality | Keep **≥1 Role** (multi-role OK); require Role on create | Matches ADR-009 AuthN; “each user needs a role” ≠ force exactly-one |
| Role vs PS | Roles = family scopes only; PS only via `user_permission_sets` | Never revive `role_permission_sets` |
| Family ownership | Identity CRUD + Role CRUD + assignments → **Client**; PS definitions → **Metadata** | ADR-004 / ADR-006 |
| Freeze vs inactive | Model **both**: `isActive` (admin disable) + `frozenAt` / unfreeze (security lock) | Requirement #2 asks for unfreeze as a distinct action |
| System Roles | Seeded Roles become `is_system=true`; customers create non-system Roles | Protect bootstrap AuthZ from accidental delete |

## Requirement → gap → target

### 1. Create users with standard SCIM expected fields

**Gap:** Create accepts only `email` + `displayName` (+ type/admin). No `userName`, name parts, `externalId`, phone, locale, timezone, title, department. Email cannot be PATCHed.

**Target (Phase A — SCIM-shaped attributes):**

Extend `users` with nullable columns (kernel migration), expose camelCase JSON on Client principal APIs:

| Majesta One column | SCIM mapping |
|---|---|
| `user_name` | `userName` (unique when set; default = email local-part or full email) |
| `external_id` | `externalId` (IdP subject / employee id; unique when set) |
| `given_name` / `family_name` | `name.givenName` / `name.familyName` |
| `display_name` | `displayName` (existing) |
| `email` | `emails[0].value` (existing; primary) |
| `phone_number` | `phoneNumbers[0].value` |
| `locale` / `timezone` | `locale` / `timezone` |
| `title` / `department` | enterprise extension `title` / `department` |
| `is_active` | `active` |

API shape (Client):

```http
POST /client/v1/principals
{
  "principalType": "user",
  "userName": "jdoe",
  "externalId": "emp-1042",
  "email": "jdoe@example.com",
  "displayName": "Jane Doe",
  "name": { "givenName": "Jane", "familyName": "Doe" },
  "phoneNumbers": [{ "value": "+1-555-0100", "type": "work" }],
  "locale": "en-US",
  "timezone": "America/Los_Angeles",
  "title": "AE",
  "department": "Sales",
  "roleApiNames": ["StandardUser"],
  "permissionSetApiNames": ["IdentityManage"]
}
```

Rules:

- `email` is **optional** for social-broker humans ([ADR-015](../adr/015-idp-agnostic-social-login.md) / [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md)); uniqueness still enforced when set.
- SCIM / directory **admin create** and Cognito write-through paths may still **require** `email` for `user` principals until operators opt into email-less social-only users.
- `service` / `agent` may keep email-as-identifier; SCIM name fields optional.
- PATCH may update profile fields including email (with uniqueness checks) and optional Cognito write-through where enabled.
- Do **not** store IdP groups as AuthZ (ADR-006 / ADR-015).

**Phase B (later):** Optional SCIM 2.0 protocol adapter (`/scim/v2/Users`) mapping to the same store — tracked under BP-017 remainders; not required to close Phase A.

### 2. Active / inactive / unfreeze

**Gap:** Only `is_active`. “Unfreeze” has no distinct model. No HTTP test for deactivate → AuthN reject → reactivate.

**Target:**

| Field / action | Semantics |
|---|---|
| `PATCH { "isActive": false }` | Admin **deactivate** — cannot authenticate; Cognito disable for `user` |
| `PATCH { "isActive": true }` | Admin **activate** — allowed only if not frozen (or combined with unfreeze) |
| `POST …/principals/{id}/freeze` | Set `frozen_at=now()`, reason optional; forces inactive auth path |
| `POST …/principals/{id}/unfreeze` | Clear `frozen_at`; restore `is_active=true` unless caller passes `reactivate: false` |

AuthN (`ResolveOneJWT`, client_credentials, exchange, API key resolve): reject when `!is_active` **or** `frozen_at IS NOT NULL`.

Schema: `users.frozen_at timestamptz`, `users.frozen_reason text` (nullable).

Optional later (not Phase A): failed-login lockout counters writing `frozen_at` automatically.

### 3. Each user needs a role

**Gap:** AuthN enforces ≥1 Role, but `POST /principals` creates rows with zero roles — JWT mint fails until a separate assign call.

**Target:**

- Create requires `roleApiNames` (non-empty array) **or** a single `roleApiName` alias.
- Persist principal + role links in **one transaction**; fail closed if any role missing.
- Default for OIDC auto-provision remains `StandardUser` (unchanged).
- Add `POST /client/v1/roles/unassign` with guard: refuse removing the last Role (`409` / `PRINCIPAL_REQUIRES_ROLE`).
- List/get principal always returns `roleApiNames`.

Do **not** add a DB check constraint that forces exactly one role — multi-role remains supported (e.g. MetadataDeveloper + future customer roles).

### 4. API to create customer roles

**Gap:** No HTTP create/update/delete; only `EnsureSystemRoles` seed.

**Target (Client, `identity.manage`):**

| Method | Path | Behavior |
|---|---|---|
| `POST` | `/client/v1/roles` | Create customer Role `{ apiName, label, scopes[] }` |
| `GET` | `/client/v1/roles` | Existing list; include `isSystem`, `scopes` |
| `GET` | `/client/v1/roles/{apiName}` | Detail |
| `PATCH` | `/client/v1/roles/{apiName}` | Update label/scopes; **reject** `is_system` |
| `DELETE` | `/client/v1/roles/{apiName}` | Delete if non-system and unassigned (or cascade unassign with explicit `?force=true`) |

Schema:

- `roles.is_system boolean NOT NULL DEFAULT false`
- Seeded Roles marked system on migrate/ensure
- `role_api_scopes` validation: only `client|metadata|deploy|ops|admin` (exact match, no substring)

Store: `UserStore.CreateRole`, `UpdateRole`, `DeleteRole` (today only `ListRoles` / `EnsureSystemRoles` / assign helpers).

Hierarchy (`parent_role_id` on API `roles`) stays out of scope — sharing uses `data_roles` (ADR-016, shipped). API-role hierarchy remainder is BP-013 P2, not User fields.

### 5. Multiple permission sets per user

**Gap:** Schema and assign already support many PS; missing unassign, assign-by-`apiName`, create-time attach, existence validation, multi-PS tests.

**Target:**

| Method | Path | Behavior |
|---|---|---|
| `POST` | `/client/v1/permissions/assign` | Accept `permissionSetId` **or** `permissionSetApiName`; validate FK; keep idempotent |
| `POST` | `/client/v1/permissions/unassign` | Remove one assignment |
| `PUT` | `/client/v1/principals/{id}/permission-sets` | Replace full set (optional convenience) |
| Create/PATCH principal | body `permissionSetApiNames[]` | Assign in same transaction as create |

Effective AuthZ continues to union all assigned PS (object/field/system) — no change to evaluation.

## Phased delivery

### Phase 1 — Role completeness (highest leverage)

Closes requirements **3** and **4**, and the create-time half of **5**.

1. Migration: `roles.is_system`; mark seed Roles system.
2. `POST/GET/PATCH/DELETE /client/v1/roles` (+ get by apiName).
3. Create principal requires `roleApiNames`; transactional assign.
4. Role unassign with last-role guard.
5. Create/PATCH accept `permissionSetApiNames`; assign-by-apiName; unassign endpoint.
6. Integration tests: create user+role+two PS → token → Client call; create custom Role with scopes → assign → JWT scopes match.

### Phase 2 — Lifecycle (active / freeze)

Closes requirement **2**.

1. Migration: `frozen_at`, `frozen_reason`.
2. Freeze / unfreeze routes; AuthN checks both flags.
3. Tests: freeze blocks token; unfreeze restores; deactivate without freeze.

### Phase 3 — SCIM-shaped profile

Closes requirement **1** (attributes, not protocol).

1. Migration: profile columns + unique indexes on `user_name` / `external_id` (NULLs allowed).
2. Expand create/patch/list JSON; Cognito provision uses `userName` when set.
3. Amend ADR-006 identity admin table + note “SCIM-shaped fields shipped; protocol still later”.
4. Tests for uniqueness and PATCH email/`externalId`.

### Phase 4 — Protocol SCIM

- `/scim/v2/Users` (+ Groups as **non-AuthZ** directory tags only, if ever).
- Bearer via Majesta One JWT with `identity.users` / `identity.integrations` (+ `authz.manage` for grants).
- Design + implementation: [scim-provisioning.md](./scim-provisioning.md).
- Update ADR-006 when protocol is accepted as GA — done for Users adapter.

### Phase 5 — Customer-extendable User (moved)

Custom User fields, SCIM UserCustom schema, JIT/SCIM provisioning defaults: **shipped** on [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md) / [BP-058](../../backlog/BP-058-user-identity-extension.md) (mitigated). Do not implement as an unplanned add-on to this file.

## Non-goals (this plan)

- Permission sets attached through Roles
- Cognito groups as AuthZ SoR
- Role hierarchy / OWD sharing (shipped on ADR-016 / BP-003 mitigated; API `roles.parent_role_id` is BP-013)
- Control IDE UI for directory (follow-up under IDE playbook once APIs land)
- Customer custom fields on User / claim mapping — shipped ([user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md); BP-058 mitigated)
- Multi-tenant SaaS `tenant_id` on AuthZ rows
- Deleting the last admin / last `identity.manage` holder without safeguards (add explicit guard in Phase 1)

## Packages to touch

| Package | Changes |
|---|---|
| `migrations/` | Role `is_system`; freeze columns; SCIM profile columns |
| `internal/db` | `User` / `CreatePrincipalInput` / `UpdatePrincipalInput`; Role CRUD; transactional create+grants; freeze helpers |
| `internal/httpapi` | `principal_routes.go` (+ tests / `principals_integration_test.go`) |
| `internal/authz` | AuthN reject frozen; scope validation for Role create |
| `internal/identity` | Pass `userName` / active / freeze through Cognito sync |
| `internal/seed` | Mark system Roles; no customer fixtures |
| Docs | This file; ADR-006 table; BP-013 / BP-017; authz playbook checklist |

## Test plan (minimum)

Prefer `internal/testutil` harness.

1. **Happy path:** IdentityManage (non-admin) creates `user` with `StandardUser` + two PS apiNames → credential → `/auth/v1/token` → Client CRUD allowed by PS.
2. **Role required:** Create without roles → `400`; unassign last role → `409`.
3. **Customer Role:** Create Role `{ apiName: "SalesRep", scopes: ["client"] }` → assign → JWT `scopes` includes `client` only.
4. **System Role protect:** PATCH/DELETE `SystemAdmin` → `403`/`409`.
5. **Lifecycle:** Freeze → token `401`/`403`; unfreeze → token OK; `isActive=false` → reject.
6. **SCIM fields:** Unique `userName` / `externalId` conflicts → `409`; list filters unchanged.
7. **Family gate:** Metadata scope alone cannot create principals; Client without `identity.manage` → `403`.

## Doc / ADR updates when implementing

- [ADR-006](../adr/006-jwt-auth.md) — extend identity admin endpoint table (Role CRUD, freeze, SCIM-shaped fields); keep protocol SCIM in non-goals until Phase 4.
- [agent-authz.md](./agent-authz.md) — point principal admin at transactional create+role; Role CRUD.
- [BP-013](../../backlog/BP-013-jwt-unified-principals.md) — link BP-017 as directory follow-on (Token Service remains shipped).
- [BP-003](../../backlog/BP-003-enterprise-auth.md) — AuthZ mitigated; SCIM attributes shipped in BP-017; User extension shipped on BP-058 (mitigated).

## Suggested implementation order for agents

1. Phase 1 only in the first PR (Role CRUD + role-on-create + PS assign/unassign by apiName).
2. Phase 2 in a second PR (freeze).
3. Phase 3 in a third PR (profile columns) — can parallelize with Phase 2 if migrations are sequenced carefully.
4. Do not start post-GA SCIM bulk/Groups until Admin/IdP customers ask.
5. Customer User fields + provisioning config: execute [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md), not this file.
