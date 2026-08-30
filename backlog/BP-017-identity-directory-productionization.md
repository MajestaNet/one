# BP-017: Identity directory productionization (users, roles, permission sets)

- **Severity:** High
- **Status:** Partially mitigated (Phases 1–4 and identity-binding hardening shipped; User custom fields / provisioning config shipped on [BP-058](./BP-058-user-identity-extension.md); **R1 Groups-as-tags shipped**; post-GA bulk/richer filters remain)
- **Area:** `internal/authz`, `internal/db`, `internal/httpapi` (`principal_routes.go`), kernel migrations, `/client/v1` identity admin
- **Plan:** [docs/architecture/identity-directory-productionization.md](../docs/architecture/identity-directory-productionization.md) (Phases 1–4 shipped)
- **Remainder plan:** [docs/architecture/agentic-remainders/07-bp-017-identity-directory.md](../docs/architecture/agentic-remainders/07-bp-017-identity-directory.md) (post-GA: Groups-as-tags, richer filters, bulk)
- **Related ADR:** [ADR-006](../docs/adr/006-jwt-auth.md), [ADR-009](../docs/adr/009-record-audit-authz-packaging.md)
- **Related BP:** [BP-013](./BP-013-jwt-unified-principals.md) (Token Service / principals foundation), [BP-003](./BP-003-enterprise-auth.md) (AuthZ mitigated), [BP-058](./BP-058-user-identity-extension.md) (User metadata + SCIM custom attributes)

## Problem

Client identity admin ships principals, Role **assignment**, and permission-set **assignment**, but customers cannot yet run a production directory:

1. User create only accepts `email` / `displayName` — not SCIM-expected profile fields
2. Lifecycle is a single `is_active` flag — no freeze / unfreeze
3. Create does not assign a Role (AuthN then rejects until a second call)
4. No API to create customer-defined Roles (seed Roles only)
5. Multi-PS is schema-ready but assignment UX is thin (UUID-only assign, no unassign, no create-time attach)

## Why it matters

B2B installs need IdP-friendly user provisioning, least-privilege Roles with custom scope mixes, and permission-set composition per user before production business data and Admin UI / SCIM connectors land. Leaving create-without-role forces a footgun; lacking customer Role CRUD forces SQL/seed for every custom persona.

## Direction

Follow the phased plan in [identity-directory-productionization.md](../docs/architecture/identity-directory-productionization.md):

| Phase | Outcome |
|---|---|
| **1** | Customer Role CRUD; Role required on principal create; PS assign/unassign by `apiName`; create-time PS attach |
| **2** | Freeze / unfreeze distinct from admin deactivate; AuthN rejects frozen |
| **3** | SCIM-shaped profile columns on `users` + Client JSON |
| **4** | `/scim/v2` protocol adapter ([scim-provisioning.md](../docs/architecture/scim-provisioning.md)) |

## Status notes

Phases 1–4 landed in the SCIM provisioning workstream: kernel migration `0026_identity_directory`, Client identity admin expansions, and `/scim/v2` Users adapter.

**Phase 5** (customer-extendable User, SCIM UserCustom schema, JIT/SCIM provisioning defaults) shipped on [BP-058](./BP-058-user-identity-extension.md) / [user-identity-extension-build-plan.md](../docs/architecture/user-identity-extension-build-plan.md) (**mitigated**).

**Remainder R1** (directory tags + SCIM Groups adapter, non-AuthZ) shipped: kernel `directory_tags` / `user_directory_tags`, Client `/client/v1/directory-tags`, `/scim/v2/Groups`. Membership does not grant Roles, permission sets, or data roles. **Still open:** richer filters (R2) and bulk (R3). Executable remainder: [07-bp-017-identity-directory.md](../docs/architecture/agentic-remainders/07-bp-017-identity-directory.md).

The August 2026 backend hardening pass also made `(provider, issuer, subject)`
links immutable across users, removed email-only OIDC linking, and made directory
grant loading fail closed. Verified-email is required before social-login JIT can
create a new user.

The 2026-08-25 review made directory-tag metadata updates atomic, moved SCIM
membership authorization before mutation, and ensured Group PATCH/PUT uses the
complete membership set rather than treating the 200-member response page as the
whole group.

Constraints (do not violate):

- Roles → scopes only; permission sets only via `user_permission_sets`
- Identity admin stays on **Client**; human ops `identity.users`, integration ops `identity.integrations` (`identity.manage` is a legacy alias only); PS **definitions** stay on **Metadata**
- Cognito is identity backend write-through, never AuthZ SoR
- ≥1 Role required; multi-role remains allowed

## Acceptance (Phase 1 minimum)

- [x] `POST /client/v1/roles` creates a non-system Role with validated scopes
- [x] System Roles cannot be patched/deleted
- [x] `POST /client/v1/principals` requires ≥1 `roleApiNames` and persists grants transactionally
- [x] Last Role cannot be unassigned
- [x] Multiple permission sets assignable (and unassignable) by `apiName` on one user
- [x] Integration tests cover create → token → Client path with custom Role + dual PS (principal harness updated for role-on-create)

## Acceptance (Phases 2–4)

- [x] Freeze / unfreeze + AuthN reject frozen
- [x] SCIM-shaped profile columns + Client JSON
- [x] `/scim/v2` Users adapter with Majesta One Principal extension

## Acceptance (R1 Groups-as-tags)

- [x] Kernel `directory_tags` + `user_directory_tags` (no AuthZ FKs; no `tenant_id`)
- [x] Client `/client/v1/directory-tags` CRUD + assign/unassign + principal `directoryTagApiNames`
- [x] `/scim/v2/Groups` CRUD + PATCH members; ResourceTypes includes Group; `User.groups` read-only
- [x] Membership does not change JWT scopes or object CRUD
- [ ] Richer SCIM/Client filters (R2)
- [ ] Client `POST /principals/bulk` + SCIM Bulk (R3)

## Related

- [BP-013](./BP-013-jwt-unified-principals.md) — JWT + principal foundation (shipped MVP)
- [BP-003](./BP-003-enterprise-auth.md) — AuthZ (mitigated)
- [BP-058](./BP-058-user-identity-extension.md) — User object extension + provisioning config (mitigated)
- [docs/architecture/agent-authz.md](../docs/architecture/agent-authz.md)
- [docs/architecture/agentic-remainders/07-bp-017-identity-directory.md](../docs/architecture/agentic-remainders/07-bp-017-identity-directory.md) — post-GA Groups-as-tags / filters / bulk
