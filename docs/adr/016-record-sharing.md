# ADR-016: Record sharing (OWD, data roles, criteria rules)

## Status

Accepted

## Context

[ADR-009](./009-record-audit-authz-packaging.md) ships owner/creator/viewAll record visibility via permission sets. B2B buyers expect enterprise organization-wide defaults (OWD), role hierarchy, and criteria-based sharing rules ([BP-003](../backlog/BP-003-enterprise-auth.md)).

Majesta One **API scope Roles** (`roles` + `user_roles`) must remain separate from **data roles** used for record visibility ([BP-017](../backlog/BP-017-identity-directory-productionization.md)).

## Decision

### 1. Opt-in, irreversible enablement

- Per-install latch: `organization_settings.record_sharing_enabled`.
- `POST /metadata/v1/sharing/enable` with `{ "confirm": true }` sets the latch once.
- Cannot be disabled via API; downgrade requires restore-from-backup.

When disabled, [ADR-009](./009-record-audit-authz-packaging.md) visibility rules apply unchanged.

### 2. Organization-wide defaults (OWD)

Per-object `object_sharing_settings.default_access`:

| Value | Read (record-level, beyond object PS) | Write |
|---|---|---|
| `private` | Owner, creator, data-role hierarchy, rule grants | Owner, hierarchy modify grants, read_write rule grants |
| `public_read` | All principals with object read | Same as private for write |
| `public_read_write` | All with object read | All with object update |

Default for all objects: `private`.

Permission sets still gate object CRUD; sharing widens record visibility only among principals who already hold object access.

### 3. Data roles (separate from API roles)

- Table `data_roles` with optional `parent_data_role_id` hierarchy.
- `users.data_role_id` — at most one primary data role per principal.
- Client API: `/client/v1/data-roles` CRUD + assign via principal PATCH.

### 4. Criteria-based sharing rules

- Table `sharing_rules` on objects where `sharing_rules_enabled=true`.
- Criteria: Client query filter shape (`[]QueryFilter`).
- Grantee: `shared_to_data_role_id` plus all subordinate data roles.
- Access: `read` | `read_write`.
- Materialized in `record_access_grants`; recalc via worker job `sharing.recalc`.

### 5. Enforcement

Order: object PS → admin/viewAll/modifyAll bypass → sharing (when enabled) → FLS.

Query visibility pushed into SQL when sharing is enabled ([ADR-003](./003-sql-query-engine.md)).

**Parity requirement:** Client HTTP, MCP tools, and composite subrequests must apply the same object + record checks. No plane may call data-engine query/get/update/delete without them.

Criteria rules must include ≥1 filter; empty filters are rejected (fail closed). Rule PATCH/DELETE invalidates materialized grants synchronously before async recalc.

Data-role hierarchy writes reject cycles; subordinate walks use a visited set.
## Consequences

- API scope Roles are unchanged; never attach sharing to `roles.parent_role_id`.
- Deploy promote exports `data_roles`, `object_sharing_settings`, `sharing_rules`; not the enable latch or grants.
- Manual shares, queues, owner-based rules deferred.

## Related

- [record-sharing.md](../architecture/record-sharing.md)
- [BP-003](../backlog/BP-003-enterprise-auth.md)
- [ADR-009](./009-record-audit-authz-packaging.md)
