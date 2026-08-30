# ADR-009: Record audit fields and AuthZ packaging (Role scopes / PS grants)

## Status

Accepted

## Context

ADR-006 packaged Roles as scopes **and** permission sets, with optional direct `user_permission_sets`. Record ownership required `records.owner_id` and used it as the only non–view-all visibility key. `principal_type` used `human` for people principals, which collided with the entity name `users` / User.

## Decision

### 1. Record system fields

| API field | Column | Rules |
|---|---|---|
| `CreatedById` | `created_by_id` | NOT NULL; set on create to actor; client cannot set |
| `LastModifiedById` | `last_modified_by_id` | NOT NULL; set on create/update to actor; client cannot set |
| `OwnerId` | `owner_id` | **Optional** (nullable); never defaulted to actor |
| `CreatedAt` / `UpdatedAt` | timestamps | Unchanged |

Visibility for Client GET/query: admin **or** `view_all`/`modify_all` on the object **or** `OwnerId == actor` (when set) **or** `CreatedById == actor`.

### 2. AuthZ packaging

```text
Principal (users) MUST have ≥1 Role
  ├── Role → role_api_scopes only (client|metadata|deploy|ops|admin)
  └── Permission sets → user_permission_sets only → object/field (and future system) permissions
```

- Drop `role_permission_sets` (migrated existing grants onto `user_permission_sets`).
- Seed `SystemAdmin` (all scopes + admin) and `StandardUser` (`client`); bootstrap service user gets SystemAdmin + Admin PS.
- OIDC auto-provision assigns `StandardUser` when the principal has no roles.
- AuthN rejects principals with zero roles (`ErrPrincipalNoRole`).
- `permission_sets.system_permissions` JSONB holds individual system capabilities (`metadata.customize`, `deploy.promote`, …) — see [customization-authz.md](../architecture/customization-authz.md).

### 3. Principal type naming

`principal_type` is `user` | `service` | `agent` (`human` → `user`). Table remains `users`.

## Consequences

- ADR-006 AuthZ packaging is superseded by this decision for Role vs PS split and principal type names.
- Record ownership is no longer required for persistence or identity junction; audit columns are the durable user links.
- Field-level and system-permission enforcement shipped (BP-003 mitigated). User-object FLS for profile/custom fields shipped with [BP-058](../../backlog/BP-058-user-identity-extension.md).

## Related

- [ADR-006](./006-jwt-auth.md)
- [ADR-008](./008-core-data-model.md)
- [data-model.md](../data-model.md)
- Migration `0013_record_audit_and_authz`
