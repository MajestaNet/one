# SCIM provisioning API

Wire-compatible SCIM 2.0 adapter for enterprise IdP connectors (Okta, Entra, etc.) over Majesta One’s unified principal store.

**Backlog:** [BP-017](../../backlog/BP-017-identity-directory-productionization.md) Phase 4 (Users adapter) · [BP-058](../../backlog/BP-058-user-identity-extension.md) (UserCustom + provisioning; mitigated)  
**Playbook:** [agent-authz.md](./agent-authz.md)  
**ADR:** [ADR-006](../adr/006-jwt-auth.md) · [ADR-026](../adr/026-kernel-user-metadata.md)

## Ownership

| Concern | Owner |
|---|---|
| Protocol surface | `/scim/v2` (standards adapter) |
| Source of truth | Same `users` / Roles / permission sets as Client identity admin |
| AuthZ packaging | Unchanged — Roles → scopes; PS → object/field/system caps |
| Connected App OAuth shape | Remains `/client/v1/integrations` (not SCIM) |

Do not invent a fourth API family. SCIM is Client identity domain logic with a wire-compatible path.

## Connector auth

1. Create a Connected App / service principal with Role scope `client`.
2. Grant permission sets:
   - `identity.users` — human Users
   - `identity.integrations` — `service` / `agent` principals
   - `authz.manage` — set `roleApiNames` / `permissionSetApiNames`
3. Mint Majesta One JWT: `POST /auth/v1/token` (client credentials).
4. Call `/scim/v2/*` with `Authorization: Bearer <jwt>`.

Prefer install `clientAccessMode` of `registered_clients` or `open`.

## Endpoints

| Method | Path | Notes |
|---|---|---|
| `GET` | `/scim/v2/ServiceProviderConfig` | patch + filter; **no bulk** (`bulk.supported=false`) |
| `GET` | `/scim/v2/Schemas` | core User + core Group + enterprise + Majesta One Principal + UserCustom |
| `GET` | `/scim/v2/ResourceTypes` | `User` and `Group` |
| `POST/GET/PUT/PATCH/DELETE` | `/scim/v2/Users` | RFC 7644 |
| `POST/GET/PUT/PATCH/DELETE` | `/scim/v2/Groups` | RFC 7644 Group = directory tag (non-AuthZ) |

Content-Type: `application/scim+json`.

## Resource model

One **Users** resource for all Majesta One principal types. Default `principalType=user` when omitted (Okta/Entra human sync unchanged).

Majesta One extension URN: `urn:ietf:params:scim:schemas:extension:one:2.0:Principal`

```json
{
  "schemas": [
    "urn:ietf:params:scim:schemas:core:2.0:User",
    "urn:ietf:params:scim:schemas:extension:one:2.0:Principal"
  ],
  "userName": "billing-bot",
  "displayName": "Billing Bot",
  "active": true,
  "urn:ietf:params:scim:schemas:extension:one:2.0:Principal": {
    "principalType": "service",
    "roleApiNames": ["DeployBot"],
    "permissionSetApiNames": ["IdentityIntegrations"],
    "dataRoleApiName": "SalesRep"
  }
}
```

### Attribute map

| SCIM | Majesta One |
|---|---|
| `id` | `users.id` |
| `externalId` | `users.external_id` |
| `userName` | `users.user_name` (required on create) |
| `name.givenName` / `familyName` | `given_name` / `family_name` |
| `displayName` | `display_name` |
| `emails[primary].value` | `email` (required for `user`) |
| `phoneNumbers[0].value` | `phone_number` |
| `active` | `is_active ∧ frozen_at IS NULL` (read); write sets `is_active` |
| `locale` / `timezone` / `title` / `department` | matching columns |
| `enterprise.employeeNumber` | `users.employee_number` (not `external_id`) |
| Customer custom User fields | Majesta One UserCustom URN `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom`; attribute names are field apiNames (`CostCenter__c`) → `users.data` |
| Majesta One `dataRoleApiName` | `users.data_role_id` (resolved) |

Create without Majesta One `roleApiNames` uses install auth `provisioning.scimDefaultRoleApiName` (default `StandardUser`). Omitted permission sets / data role are filled from `provisioning.scimDefault*`. See [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md).

### Groups

SCIM Group is the wire name for **directory tags** (Client SoR). Same Postgres rows (`directory_tags` / `user_directory_tags`). Membership is **not** AuthZ: Groups never write `user_roles`, `user_permission_sets`, or `users.data_role_id`. JWT scopes, object/field perms, and record sharing ignore tags.

| SCIM Group | Majesta One |
|---|---|
| `id` | `directory_tags.id` |
| `displayName` | `display_name` (unique; required on create) |
| `externalId` | `directory_tags.external_id` |
| `members[].value` | `users.id` (`type=User` only) |
| `members[].$ref` | `/scim/v2/Users/{id}` |

`api_name` is the Client identifier (derived from `displayName` on SCIM create; not a SCIM attribute). PUT/PATCH `displayName` updates `display_name` only.

Okta/Entra **Push Groups** → `/scim/v2/Groups`. Map IdP group → Role via connector attribute mappings into `urn:ietf:params:scim:schemas:extension:one:2.0:Principal:roleApiNames` on **User** — never by treating the Group as a Role.

`User.groups` is populated read-only on GET/list. PATCH/PUT User with `groups` → `400` `invalidValue` (`membership is managed on /Groups`). Nested Group members are rejected. GET Group returns up to 200 members; callers see only members they `canReadPrincipalType`. Client list/detail: `/client/v1/directory-tags`.

Group filter (R1): `displayName eq`, `externalId eq`. Richer filters and Bulk remain post-GA ([BP-017](../../backlog/BP-017-identity-directory-productionization.md) remainder).

## Multi-environment (test / staging / prod)

Each Majesta One install has its **own** Postgres directory (ADR-001). Deploy promotes **metadata**, not principals.

Enterprise pattern for “some users only in prod, some in every env”:

| Need | How |
|---|---|
| Same person across installs | Same SCIM `externalId` (HR/IdP federation id) + stable `userName` on each install’s `/scim/v2` |
| Prod-only access | Provision that User **only** into the prod install |
| All-env access | Provision the same `externalId` / `userName` / email into each install (separate SCIM target per env) |
| Social login per env | Link Google/Apple `sub` via `identity_links` on that install after provision or auto-provision |

Do **not** add a separate `federation_id` column — `users.external_id` is already the federation/correlation key and is already on the SCIM User resource. Do **not** Deploy-promote `users`, credentials, or role assignments.

## Lifecycle

| Event | Behavior |
|---|---|
| Provision | Insert principal + Roles (+ PS) in one TX → Cognito write-through → audit `scim.user.create` |
| Update | Patch profile / extension grants; audit `scim.user.update` details include changed User field apiNames (not values) |
| Deactivate / DELETE | Soft-delete: `is_active=false`, revoke credentials, Cognito disable; audit `scim.user.deprovision` |
| Reactivate | `active=true` only if not frozen; credentials stay revoked |
| Freeze | Client `POST …/principals/{id}/freeze` only — not SCIM |

## Filter support (v1)

`eq` and `and` on: `userName`, `externalId`, `emails.value`, `active`, Majesta One `principalType`.

## Packages

| Package | Role |
|---|---|
| `internal/scim` | Serializers, filter, patch |
| `internal/httpapi/scim_routes.go` | HTTP adapter |
| `internal/db` | Principal store (BP-017 columns + grants) |

## Non-goals

- IdP groups as AuthZ SoR
- Multi-tenant SaaS `tenant_id` on SCIM rows
- Hard delete in GA
- Bulk endpoint (post-GA; `ServiceProviderConfig.bulk.supported` stays `false`)
- Nested SCIM Groups; mapping tags to `data_roles`
- Control IDE directory UI (separate IDE follow-up)
- Customer custom attributes on User (shipped; [BP-058](../../backlog/BP-058-user-identity-extension.md) mitigated). Richer filters still post-GA on BP-017.

## Okta / Entra custom-attribute recipe

Operator runbook (Connected App JWT, Schemas discovery, UserCustom URN paths, employeeNumber vs externalId, no groups-as-AuthZ): [auth-adapters.md](../auth-adapters.md) (section **SCIM directory**).

Connector checklist:

1. `POST /metadata/v1/fields` on object `User` for each directory attribute (`CostCenter__c`, …).
2. `GET /scim/v2/Schemas` must list those apiNames under `urn:ietf:params:scim:schemas:extension:one:2.0:UserCustom`.
3. Map IdP attributes to that URN + apiName. PATCH `add`/`replace` on `UserCustom:<apiName>` writes `users.data`.
4. Optional `PUT /metadata/v1/install/auth` `provisioning.scimDefault*` so creates without the Majesta One Principal extension still receive a Role / PS / data role.
5. Client list `GET /client/v1/principals` omits `users.data`; `?include=data` returns FLS-stripped custom fields. GET-by-id remains the full FLS view.
