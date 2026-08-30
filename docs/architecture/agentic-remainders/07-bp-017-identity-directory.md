# Identity directory post-GA remainders — tech design + agentic build plan

**Work-order slot:** 7 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-017](../../../backlog/BP-017-identity-directory-productionization.md)
**Track:** Finish
**Status of remainder:** Open (R1 Groups-as-tags shipped; bulk and richer filters remain)
**Domain agents:** `authz-security` (primary) / `api-families` (route ownership) / `db-backend-perf` (kernel SQL + list indexes only)
**Playbooks:** [agent-authz.md](../agent-authz.md), [agent-api-families.md](../agent-api-families.md)
**Existing plans (do not duplicate):** [identity-directory-productionization.md](../identity-directory-productionization.md) (Phases 1–4), [scim-provisioning.md](../scim-provisioning.md) (Users adapter), [user-identity-extension-build-plan.md](../user-identity-extension-build-plan.md) (BP-058 mitigated)

---

## 1. Remainder inventory

Honestly mark work already in tree. Do not re-plan shipped phases.

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| Customer Role CRUD; Role required on create; last-Role guard | Client Role routes + transactional create | — | `internal/httpapi/principal_routes.go` (`handleCreateRole`, `handleCreatePrincipal`); `migrations/0026_identity_directory.sql` (`roles.is_system`); `internal/httpapi/principals_integration_test.go` |
| Freeze / unfreeze distinct from `isActive` | Client freeze routes; AuthN reject frozen | — | `principal_routes.go` freeze/unfreeze; `users.frozen_at`; `internal/authz/actor_load.go`, `apikey.go` (`user frozen`) |
| SCIM-shaped kernel profile columns | `user_name`, `external_id`, name parts, phone, locale, timezone, title, department | — | `migrations/0026_identity_directory.sql`; `internal/db/users.go` (`userSelectCols`) |
| `/scim/v2` Users adapter + Principal extension | RFC 7644 Users + `roleApiNames` / `permissionSetApiNames` / `dataRoleApiName` | — | `internal/scim/user.go`, `schemas.go`; `internal/httpapi/scim_routes.go`; `internal/httpapi/scim_integration_test.go` |
| Filter v1 | `eq` + `and` on `userName`, `externalId`, `emails.value`, `active`, `principalType`; `meta.created` accepted and **ignored** | `or` / `ne` / `co` / `sw` / `pr`; department/title/employeeNumber; custom-field eq; group membership; Client list pagination | `internal/scim/filter.go`; `UserStore.List` / `ListPrincipalsFilter` in `internal/db/users.go`; Client `GET /principals` query keys in `handleListPrincipals` |
| User custom fields + UserCustom schema + JIT/SCIM defaults | Kernel `User` metadata object, `users.data`, `employee_number`, install `provisioning` | — | `migrations/0055_user_kernel_metadata.sql`, `0056_install_auth_provisioning.sql`; BP-058 mitigated |
| SCIM ServiceProviderConfig | `patch` + `filter` true; **`bulk.supported=false`**; ResourceTypes = User + Group | Bulk endpoint | `internal/scim/schemas.go` `ServiceProviderConfig`, `ResourceTypes` |
| SCIM Groups | Groups adapter over **directory tags** (non-AuthZ) | Nested groups; members.value filter (R2) | `internal/scim/group.go`; `registerSCIMGroupRoutes` in `scim_group_routes.go` |
| Client bulk principal ops | Record `POST /client/v1/bulk/{object}` is DataEngine (kernel User rejected) | Identity bulk (sync, small batch) + optional SCIM Bulk | `internal/httpapi/server.go` bulk route; ADR-026 / DataEngine refuse `storage_mode=kernel` |
| Directory tags / group membership store | Kernel `directory_tags` + `user_directory_tags`; Client tag APIs | List filter `?directoryTagApiName=` (R2) | `migrations/0061_directory_tags.sql`; `internal/db/directory_tags.go` |
| Control IDE directory UI | Out of remainder (ADR-030 chrome frozen) | — | Do not unfreeze; builders use MCP + `one` |

Identity-binding hardening already on BP-017 (immutable `(provider, issuer, subject)`, no email-only OIDC link, fail-closed grant load, verified-email before social JIT) is **shipped**. Do not reopen it here.

---

## 2. Detailed design (remainder only)

Cite [ADR-004](../../adr/004-three-api-families.md), [ADR-006](../../adr/006-jwt-auth.md), [ADR-009](../../adr/009-record-audit-authz-packaging.md), [ADR-015](../../adr/015-idp-agnostic-social-login.md), [ADR-016](../../adr/016-record-sharing.md), [ADR-026](../../adr/026-kernel-user-metadata.md). Do not invent a parallel stack. Do not unfreeze Control IDE chrome.

### 2.1 Locked product rules (unchanged)

| Rule | Remainder implication |
|---|---|
| Roles → API family scopes only | Directory tags / SCIM Groups **never** write `user_roles` |
| Permission sets only via `user_permission_sets` | Groups **never** write `user_permission_sets` |
| Sharing uses `data_roles` (ADR-016), not API `roles.parent_role_id` | Groups **never** write `users.data_role_id` |
| Identity CRUD stays **Client** (`scope: client`) | Tag CRUD + membership + bulk principals are Client; SCIM is a Client-owned wire adapter, not a fourth family |
| PS **definitions** stay **Metadata** | Remainder does not add Metadata group/tag objects |
| Cognito (or any IdP) is never AuthZ SoR | Optional `IDENTITY_SYNC=cognito` must not ingest Cognito groups as Roles/PS/data roles |
| One install = one customer DB (ADR-001) | No `tenant_id` on tag rows; Deploy does not promote principals, tags, or memberships |
| User stays kernel `users` (ADR-026) | Bulk/filter/tags do **not** go through DataEngine / `records` |

Live capability split (already in `internal/authz/system_perms.go`; BP-017’s phrase `identity.manage` is the **legacy alias** that still expands to both user + integration caps):

| Operation | Capability |
|---|---|
| Human principal + human tag membership | `identity.users` |
| `service` / `agent` principal + tagging those types | `identity.integrations` |
| Role / permission-set **assignment** | `authz.manage` (unchanged; **not** required to tag) |
| Permission-set **definitions** | Metadata + `authz.manage` |
| Family mux | `scope: client` on Client and `/scim/v2` |

Admin privilege still implies all system capabilities. Scope matching stays exact (no substring).

### 2.2 Groups-as-tags (first implementable remainder)

Okta/Entra **Push Groups** is the connector hole. Sequential `/scim/v2/Users` already works; bulk is advertised `supported: false`. Groups must land **before** anyone maps IdP groups onto Roles.

**SoR name:** directory tags (Client). **Wire name:** SCIM Group (RFC 7643 core Group). Same Postgres rows.

#### Schema (next kernel migration, likely `0060_directory_tags.sql` + journal tag `0060_directory_tags`)

```sql
CREATE TABLE directory_tags (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_name text NOT NULL,
  display_name text NOT NULL,
  external_id text,
  description text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_tags_api_name_key UNIQUE (api_name),
  CONSTRAINT directory_tags_display_name_key UNIQUE (display_name)
);
CREATE UNIQUE INDEX directory_tags_external_id_uidx
  ON directory_tags (external_id) WHERE external_id IS NOT NULL;

CREATE TABLE user_directory_tags (
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES directory_tags (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, tag_id)
);
CREATE INDEX user_directory_tags_tag_id_idx ON user_directory_tags (tag_id);
```

Rules:

- No FK to `roles`, `permission_sets`, or `data_roles`.
- No evaluator in `internal/authz` (`object_perms.go`, `sharing.go`, `actor_load.go`, JWT mint) may read these tables.
- `api_name` is the Client identifier (PascalCase, same style as Role apiNames, e.g. `OktaSales`). SCIM `id` is the UUID.
- `display_name` is SCIM `displayName` (unique so IdP push is deterministic).
- `external_id` is the IdP group id / federation key (same idea as `users.external_id`).
- Nested groups (Group member type `Group`) are **out of this remainder**.
- DELETE tag cascades memberships only; users stay. SCIM DELETE Group does **not** deprovision Users.

Store: new `internal/db/directory_tags.go` (`DirectoryTagStore`). Do not stuff this into `users.go` beyond listing tag apiNames on a principal.

#### Client API (family: Client)

All routes `scope: client`. Human tag CRUD/membership: `identity.users`. If a member’s `principal_type` is `service` or `agent`, require `identity.integrations` (same split as SCIM Users).

| Method | Path | Body / behavior |
|---|---|---|
| `POST` | `/client/v1/directory-tags` | `{ "apiName", "label", "externalId?", "description?" }` → 201. `label` → `display_name`. If `apiName` omitted, slug from `label` (non-empty `[A-Za-z][A-Za-z0-9_]*`, unique; suffix `2`… on collision). |
| `GET` | `/client/v1/directory-tags` | `{ "tags": [ … ] }` |
| `GET` | `/client/v1/directory-tags/{apiName}` | Detail + `memberCount` (not full member dump by default) |
| `PATCH` | `/client/v1/directory-tags/{apiName}` | `label` / `externalId` / `description`; `apiName` immutable |
| `DELETE` | `/client/v1/directory-tags/{apiName}` | 204; cascade memberships |
| `GET` | `/client/v1/directory-tags/{apiName}/members` | Paginated principals (cap 200); FLS-stripped list shape |
| `POST` | `/client/v1/directory-tags/assign` | `{ "principalId", "tagApiName" }` idempotent |
| `POST` | `/client/v1/directory-tags/unassign` | Same keys; 404 if missing pair is OK as idempotent 204 |

Principal JSON (GET/PATCH/create):

```json
{
  "directoryTagApiNames": ["OktaSales"]
}
```

Create/PATCH may set `directoryTagApiNames` (replace-on-PATCH when the key is present; omit = leave unchanged). This does **not** require `authz.manage`. Unknown tag apiName → `404` / `DIRECTORY_TAG_NOT_FOUND`.

List principals (Phase R2 may add `?directoryTagApiName=`); Phase R1 GET-by-id is enough if list filter slips.

Audit: `identity.tag.create|update|delete`, `identity.tag.assign|unassign` (ids + apiNames, not PII dumps).

#### SCIM Groups adapter

Register next to existing Users routes in `internal/httpapi/scim_routes.go`. Content-Type `application/scim+json`. Bearer = Majesta One JWT with `scope: client` (already `requireScope(ScopeClient)`).

| Method | Path | Notes |
|---|---|---|
| `GET` | `/scim/v2/ResourceTypes` | Add Group (`endpoint: /Groups`, schema core Group). `totalResults` becomes 2. |
| `GET` | `/scim/v2/Schemas` | Add `urn:ietf:params:scim:schemas:core:2.0:Group` |
| `POST/GET/PUT/PATCH/DELETE` | `/scim/v2/Groups` | RFC 7644 Group |

Create example:

```http
POST /scim/v2/Groups
Content-Type: application/scim+json

{
  "schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
  "displayName": "Sales",
  "externalId": "okta-grp-01",
  "members": [
    { "value": "<users.id>", "type": "User" }
  ]
}
```

Map:

| SCIM Group | Majesta One |
|---|---|
| `id` | `directory_tags.id` |
| `displayName` | `display_name` (required on create) |
| `externalId` | `directory_tags.external_id` |
| `members[].value` | `users.id` |
| `members[].$ref` | `/scim/v2/Users/{id}` |
| `members[].type` | `User` only |
| `members[].display` | `users.display_name` |

`api_name` is not a SCIM attribute. Derive on create from `displayName` (strip non-alphanumeric, PascalCase, unique). PUT/PATCH `displayName` updates `display_name` only (do not rename `api_name`).

PATCH ops (reuse `internal/scim/patch.go` patterns; add Group-specific paths):

- `add` / `remove` / `replace` on `members` (and `members[value eq "…"]` remove).
- `replace` `displayName` / `externalId`.
- Member add of unknown user → `404` `invalidValue`.
- Member type `Group` → `400` `invalidValue`.

GET Group returns up to **200** members (same cap as User list `maxResults`). Over-cap: still `total` via `memberCount` in a Majesta One-only Client view; SCIM GET may omit extras and document that full membership is `filter=members.value eq "…"`. Prefer returning all members when `count ≤ 200`; if a group exceeds 200, return 200 + `itemsPerPage` and do **not** silently drop without exceeding RFC list semantics — Phase R1 exit criterion is “Okta-sized groups ≤ 200 members”; larger groups are Phase R2 filter/`startIndex` on members (optional `GET /Groups/{id}` query `startIndex`/`count` on members is allowed in R1 if cheap).

**User.groups is read-only.** RFC 7643 User may include:

```json
"groups": [
  { "value": "<tag-id>", "display": "Sales", "$ref": "/scim/v2/Groups/<tag-id>" }
]
```

Populate on User GET/list after R1. PATCH/PUT User with `groups` present → `400` `invalidValue` (`membership is managed on /Groups`). Do **not** put group names on the Majesta One Principal extension.

AuthZ on Groups:

- Create/update/delete Group resource: `identity.users`.
- Add/remove a member: `identity.users` for `user`; `identity.integrations` for `service`/`agent`.
- GET Group / list: caller sees members they `canReadPrincipalType` (existing helper in `scim_routes.go`).
- **Never** call `authz.manage` for membership.

Connector recipe (docs-only in the implementation PR): Okta/Entra Push Groups → `/scim/v2/Groups`. Role assignment stays connector attribute mapping into `urn:ietf:params:scim:schemas:extension:one:2.0:Principal:roleApiNames` on **User**. [auth-adapters.md](../../auth-adapters.md) SCIM section already says no groups-as-AuthZ — extend with the Groups-as-tags mapping.

ServiceProviderConfig: still `bulk.supported=false` until Phase R3. Filter remains supported.

Cognito write-through (`internal/identity`): **no group sync** in this remainder. Tags stay install-local Postgres.

Failure modes:

| Case | Response |
|---|---|
| Duplicate `displayName` / `apiName` / `externalId` | HTTP 409; SCIM `uniqueness` |
| PATCH Group members used as Role grant | Membership stored; JWT `scopes` and PS union **unchanged** (required test) |
| IdP PATCH User.groups | 400; membership unchanged |
| DELETE Group | Tag + join rows gone; users/Roles/PS intact |
| Metadata-only token | 403 on `/scim/v2/Groups` and Client tag routes |
| Client without `identity.users` | 403 |
| Deploy promote of tags | Not an API; snapshot must continue to omit `users` and must omit `directory_tags` |

### 2.3 Richer filters (Phase R2)

Today: `internal/scim/filter.go` is `attr eq value [and …]`. Client list is exact match on `principalType`, `email`, `userName`, `externalId`, `isActive`, **unbounded** `ORDER BY created_at`.

**SCIM User filter (post-GA):**

Replace `ParseFilter` with a small recursive-descent parser (keep it in `internal/scim/filter.go`). Supported operators: `eq`, `ne`, `co`, `sw`, `ew`, `pr`, `and`, `or`. Parentheses required for mixed `and`/`or`. Reject everything else with `invalidFilter` (including `gt`/`lt` on dates in R2 — `meta.created` either implement `eq` as timestamptz UTC day **or** keep rejecting with a clear error; do not keep “accept and ignore”).

Allowlisted attributes (exact names, case-insensitive attr path):

| Attribute | Store |
|---|---|
| `userName`, `externalId`, `emails.value`, `active`, `displayName`, `title` | `users.*` (`active` remains `is_active ∧ frozen_at IS NULL` on **read match**, same as today) |
| `urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department` / `employeeNumber` | `users.department` / `employee_number` |
| Principal `principalType` (existing URN + short name) | `users.principal_type` |
| `groups.value`, `groups.display` | join `user_directory_tags` / `directory_tags` |
| UserCustom `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom:<apiName>` | `users.data` **eq/ne/pr only** (no `co`/`sw` on JSONB in R2) |

Push predicates into `ListPrincipalsFilter` (extend the struct). Do not interpolate operator strings into SQL; bind values. Cap `startIndex`/`count` remains 200. Custom-field filter requires the apiName to exist on metadata object `User`; unknown → `invalidFilter`.

**SCIM Group filter (R1 minimum, R2 complete):** R1: `displayName eq`, `externalId eq`. R2: `members.value eq`, `and`/`or`.

**Client `GET /client/v1/principals`:**

Add query keys (all AND, exact unless noted):

- `department`, `title`, `employeeNumber`
- `frozen=true|false` (`frozen_at IS NOT NULL`)
- `roleApiName`, `permissionSetApiName`, `directoryTagApiName`
- `q` — prefix (`sw`) on `user_name` **or** `email` **or** `display_name` (escape `%`/`_`)
- `limit` (default 50, max 200) + `offset` (default 0)
- Response: `{ "principals": […], "totalSize": N }` (`totalSize` = un-paged count)

Do not add a JSON query language on principals. Do not route this through `/query` / DataEngine.

### 2.4 Bulk (Phase R3)

Two surfaces, one kernel store. **Do not** reuse `POST /client/v1/bulk/{object}` or `ingest.process` (those are record objects; User is `storage_mode=kernel`).

#### Client sync bulk

```http
POST /client/v1/principals/bulk
{
  "failOnErrors": 1,
  "operations": [
    { "op": "create", "record": { /* same body as POST /principals */ } },
    { "op": "patch", "id": "<uuid>", "record": { /* PATCH body */ } },
    { "op": "freeze", "id": "<uuid>", "reason": "…" },
    { "op": "unfreeze", "id": "<uuid>" }
  ]
}
```

- Max **25** operations, body ≤ 1 MiB (existing `LimitReader` pattern).
- Each op uses the **same** validation + capability checks as the single-resource handlers (`identity.users` / `identity.integrations` / `authz.manage` when the op assigns Roles/PS).
- `failOnErrors`: stop after N failed ops; already-applied ops **stay** (not one giant TX across 25 creates — uniqueness conflicts would poison the batch). Each op is its own TX (create+roles already transactional today).
- Response: `{ "results": [ { "index", "status", "id?", "error?" } ] }` using existing API error codes.
- Freeze remains Client-only (SCIM Bulk must not grow a freeze op).

Optional later: `POST /client/v1/directory-tags/bulk` — **out of R3** unless Groups bulk is needed for the SCIM Bulk path below.

#### SCIM Bulk

```http
POST /scim/v2/Bulk
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],
  "failOnErrors": 1,
  "Operations": [
    { "method": "POST", "path": "/Users", "bulkId": "a", "data": { … } },
    { "method": "PATCH", "path": "/Groups/{id}", "data": { … } }
  ]
}
```

- Flip `ServiceProviderConfig.bulk` to `supported: true`, `maxOperations: 25`, `maxPayloadSize: 1048576`.
- Allowed `path` prefixes: `/Users`, `/Groups` only.
- Reuse existing Users/Groups handlers internally (shared service functions — avoid HTTP self-calls).
- `bulkId` resolution for POST→later PATCH in the same request: in-memory map for this payload only.
- AuthZ per operation (same caps as single-resource).
- Soft-delete DELETE Users unchanged.

Worker/async identity ingest is **out** (would be a new job class; not needed for connector-sized batches).

### 2.5 AuthZ evaluation contract (regression)

After any Groups/bulk/filter work, these must still hold:

1. JWT `scopes` come only from assigned Roles (`role_api_scopes`).
2. Object/field/system caps come only from assigned permission sets.
3. Record sharing uses `data_roles` + `record_access_grants` (ADR-016), never directory tags.
4. IdP / Cognito group claims are ignored for AuthZ (ADR-006 / ADR-015). Connector maps group → `roleApiNames` on User if the customer wants a Role.
5. ≥1 Role still required to authenticate; tagging a user does not satisfy that.

### 2.6 IDE / MCP

No Control IDE panels, no `ide.*` HTTP gates, no new Electron chrome (ADR-030). MCP builders call Client + SCIM with JWT. BP-065 lockstep is unrelated.

---

## 3. Concrete agentic build plan

### Phase R1 — Directory tags + SCIM Groups adapter (first slice)

- **Owner domain agent:** `authz-security` (HTTP registration reviewed against `api-families`)
- **Packages allowed:** `migrations/`, `migrations/meta/_journal.json`, `internal/db`, `internal/scim`, `internal/httpapi` (`scim_routes.go`, `principal_routes.go`, tests), `internal/authz` **only** if a new constant is required (prefer none — reuse `CapIdentityUsers` / `CapIdentityIntegrations`). Docs: this remainder, [BP-017](../../../backlog/BP-017-identity-directory-productionization.md), [scim-provisioning.md](../scim-provisioning.md), [auth-adapters.md](../../auth-adapters.md) SCIM section, [ADR-006](../../adr/006-jwt-auth.md) identity-admin table row for Groups/tags.
- **Packages forbidden:** `tools/control-ide/**`, `internal/dataengine` User CRUD, `internal/identity` Cognito group mapping, Metadata permission-set definition routes, `internal/authz/object_perms.go` / `sharing.go` (must not consult tags).
- **Files likely to change:** `migrations/0060_directory_tags.sql`; `internal/db/directory_tags.go` (new); `internal/db/users.go` (load/save `directoryTagApiNames` on get/create/patch — join only); `internal/scim/schemas.go` (ResourceTypes + Group schema); `internal/scim/group.go` (new serializers); `internal/scim/patch.go` (members ops); `internal/httpapi/scim_routes.go`; `internal/httpapi/principal_routes.go`; `internal/httpapi/scim_integration_test.go`; `internal/httpapi/principals_integration_test.go` or a focused `directory_tags_integration_test.go`; `internal/scim/scim_test.go`.
- **Tests to add or extend:** `go test ./internal/scim/... ./internal/db/... ./internal/httpapi/...` (DB-gated via `internal/testutil`). See exit criteria.
- **Exit criteria (observable):**
  1. `GET /scim/v2/ResourceTypes` lists User and Group.
  2. SCIM Group create + PATCH add/remove member round-trips; Client `directoryTagApiNames` matches.
  3. After Group membership, mint JWT for that user: `scopes` unchanged vs pre-membership; object CRUD still follows PS only (Account create still 403 if PS denies).
  4. PATCH User with `groups` → 400; membership unchanged.
  5. Metadata-scoped key cannot call `/scim/v2/Groups` (403).
  6. Duplicate `displayName` → 409.
  7. `ServiceProviderConfig.bulk.supported` still `false`.
- **Dependencies:** BP-017 Phases 1–4 and BP-058 (shipped). No dependency on BP-041 ingest jobs. No dependency on BP-013 P2 role hierarchy.

### Phase R2 — Richer filters + Client list pagination

- **Owner:** `authz-security` (+ `db-backend-perf` if indexes are needed: `(department)`, `(title)`, GIN on `users.data` is **not** in R2 — custom eq uses `data->>apiName` and must stay rare)
- **Packages allowed:** `internal/scim/filter.go`, `internal/db/users.go` (`ListPrincipalsFilter`), `internal/httpapi/scim_routes.go`, `principal_routes.go`, tests, optional migration for btree indexes on `users.department` / `employee_number` (latter already unique-when-set)
- **Forbidden:** Elasticsearch / search index for directory (BP-043 is records); DataEngine query on User
- **Tests:** `go test ./internal/scim/... ./internal/httpapi/...` — `or` / `sw` / `groups.display eq`; invalid operator → 400; Client `limit`/`offset` + `totalSize`; custom field `co` rejected
- **Exit criteria:** Okta-style `userName sw "j" and active eq true`; `groups.display eq "Sales"` returns tagged users; Client list no longer unbounded (default 50)
- **Dependencies:** Phase R1 for group filters; can land User-only operators in a PR before Groups if Groups is delayed, but this plan sequences Groups first

### Phase R3 — Bulk (Client principals + SCIM Bulk)

- **Owner:** `authz-security` / `api-families`
- **Packages allowed:** `internal/httpapi` (new bulk handlers; extract shared create/patch helpers from `scim_routes.go` / `principal_routes.go` if needed), `internal/scim/schemas.go` (flip bulk config), tests
- **Forbidden:** `internal/worker` ingest jobs; `POST /bulk/User`; Control IDE
- **Tests:** 25-op cap → 400; mixed success/fail honors `failOnErrors`; Bulk POST User then PATCH Group members via `bulkId`; capability split still enforced per op
- **Exit criteria:** `ServiceProviderConfig.bulk.supported=true` with documented caps; Client `POST /principals/bulk` create+role in one op still transactional per item
- **Dependencies:** R1 if Bulk includes `/Groups`; Users-only Bulk could theoretically precede Groups but this plan keeps one Bulk PR after Groups so ResourceTypes/Bulk stay consistent

### Status updates when a phase lands

Update [BP-017](../../../backlog/BP-017-identity-directory-productionization.md) checkboxes + status notes, and the table row in [backlog/README.md](../../../backlog/README.md). Amend [scim-provisioning.md](../scim-provisioning.md) Groups / bulk / filter sections so they stop saying “not implemented” for landed phases. Do not mark BP-017 fully mitigated until R1–R3 exit criteria pass (R1 alone is “partially mitigated — Groups-as-tags shipped”).

---

## 4. Explicit non-goals

- Cognito, Okta, Entra, Google, or any IdP as Majesta One **AuthZ** system of record (groups → Roles / permission sets / data roles)
- Reintroducing Cognito as the product GA AuthN default
- `role_permission_sets` / assigning permission sets through Roles
- SCIM Groups nested membership (Group-in-Group)
- Mapping directory tags to record sharing (ADR-016 data roles stay separate)
- Multi-tenant SaaS `tenant_id` on tag or principal rows
- Deploy-promoting `users`, credentials, grants, or directory tags
- DataEngine / `records` User CRUD or `POST /client/v1/bulk/User`
- Async identity ingest job / worker class
- Hard-delete Users (SCIM DELETE stays soft-deactivate)
- Freeze via SCIM or SCIM Bulk
- Custom-attribute `co`/`sw` filters on `users.data` (R2 is eq/ne/pr only)
- Control IDE directory UI, `ide.*` HTTP gates, Electron chrome
- Email OTP / magic link; embedded Keycloak
- Changing JIT / UserCustom / employeeNumber split (BP-058 shipped)
- Failed-login lockout auto-freeze (still optional later per the original plan)

---

## 5. Agentic implementation prompt(s)

### Phase R1 — SCIM Groups as directory tags

```text
You are the Majesta One authz-security agent. Implement BP-017 remainder Phase R1 only: directory tags (Client) + SCIM Groups adapter (non-AuthZ). Do not implement richer filters or bulk.

Read first:
- docs/architecture/agentic-remainders/07-bp-017-identity-directory.md (this remainder — follow §2.2 and Phase R1)
- docs/architecture/agent-authz.md
- docs/architecture/agent-api-families.md
- docs/architecture/scim-provisioning.md
- docs/adr/006-jwt-auth.md, docs/adr/009-record-audit-authz-packaging.md, docs/adr/015-idp-agnostic-social-login.md, docs/adr/026-kernel-user-metadata.md
- backlog/BP-017-identity-directory-productionization.md
- internal/scim/*.go, internal/httpapi/scim_routes.go, internal/httpapi/principal_routes.go, internal/db/users.go, internal/authz/system_perms.go
- migrations/0026_identity_directory.sql, migrations/0055_user_kernel_metadata.sql, migrations/meta/_journal.json

Edit scope (only):
- migrations/ (next numbered SQL, likely 0060_directory_tags.sql) + journal entry
- internal/db (new directory_tags store; principal get/create/patch may load/save directoryTagApiNames via join)
- internal/scim (Group schema/resource type, serializers, Group PATCH members; User.groups read-only on GET)
- internal/httpapi/scim_routes.go, principal_routes.go, focused tests
- Docs: scim-provisioning.md Groups section, auth-adapters.md SCIM (Push Groups → tags), ADR-006 identity-admin table, BP-017 status notes for R1

AuthZ (must hold):
- Client + /scim/v2 stay scope: client. Human tag/Group ops: identity.users. service/agent members: identity.integrations. Role/PS assignment remains authz.manage and is NOT used for tagging.
- PS definitions stay on Metadata — do not add a Metadata Group object.
- identity.manage is a legacy alias only; do not reintroduce it as the only gate.
- Directory tags must not be read by JWT mint, object_perms, or sharing evaluators.
- No tenant_id. No Cognito group → Role sync.

Implement:
1. Kernel tables directory_tags + user_directory_tags as specified in the remainder §2.2.
2. Client /client/v1/directory-tags CRUD + assign/unassign + principal JSON directoryTagApiNames.
3. /scim/v2/Groups CRUD + PATCH members; ResourceTypes + Schemas include Group; User.groups populated read-only; PATCH User.groups → 400.
4. Derive api_name from SCIM displayName; Client may pass apiName. Unique displayName, apiName, externalId.
5. Audit identity.tag.* / scim.group.* (ids + apiNames, not secret values).
6. Leave ServiceProviderConfig.bulk.supported=false.

Tests (DB-gated via internal/testutil; skip-without-DB tests still pass):
- go test ./internal/scim/... ./internal/db/... ./internal/httpapi/...
- SCIM Group create + PATCH add/remove member; Client GET principal shows directoryTagApiNames
- Membership does not change JWT scopes or Account object create when PS denies
- PATCH /scim/v2/Users/{id} with groups → 400
- Duplicate displayName → 409
- Metadata-only caller → 403 on Groups
- canReadPrincipalType filters Group members (identity.users cannot see service members)

Out of scope:
- Filter parser expansion (or/ne/co/sw), Client list pagination, SCIM Bulk, POST /principals/bulk
- DataEngine User routes, ingest jobs, tools/control-ide, Cognito as AuthZ SoR
- Nested SCIM Groups, mapping tags to data_roles, freeze via SCIM

When R1 lands: update BP-017 (Groups-as-tags shipped; bulk/filters still open) and backlog/README.md status sentence. Do not mark BP-017 fully mitigated.
```
