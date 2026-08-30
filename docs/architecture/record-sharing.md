# Record sharing — operator & agent playbook

Enterprise record sharing for BP-003. See [ADR-016](../adr/016-record-sharing.md).

## Planes

| Plane | Tables / API | Purpose |
|---|---|---|
| API roles | `roles`, `user_roles` | HTTP scopes (`client`, `metadata`, …) |
| Permission sets | `object_permissions`, `field_permissions` | Object CRUD + FLS |
| Data roles | `data_roles`, `users.data_role_id` | Sharing hierarchy + rule grantees |
| Sharing config | `organization_settings`, `object_sharing_settings`, `sharing_rules` | OWD + criteria rules |

## Enable sharing (once)

```http
POST /metadata/v1/sharing/enable
Authorization: Bearer …
Content-Type: application/json

{ "confirm": true }
```

Requires `authz.manage`. Irreversible.

## Configure OWD per object

```http
PATCH /metadata/v1/sharing/objects/Account
{ "defaultAccess": "private", "sharingRulesEnabled": true }
```

## Criteria rule

```http
POST /metadata/v1/sharing/objects/Account/rules
{
  "apiName": "ShareWestCoast",
  "label": "West → Sales Manager",
  "active": true,
  "accessLevel": "read",
  "sharedToDataRoleApiName": "SalesManager",
  "criteria": {
    "filters": [{ "field": "Region__c", "op": "eq", "value": "West" }]
  }
}
```

## Data roles

```http
POST /client/v1/data-roles
{ "apiName": "SalesManager", "label": "Sales Manager", "parentDataRoleApiName": "CEO" }

PATCH /client/v1/principals/{id}
{ "dataRoleApiName": "SalesRep" }
```

## Code map

| Concern | Path |
|---|---|
| Evaluator | `internal/authz/sharing.go` |
| DB | `internal/db/sharing.go`, `internal/db/data_roles.go` |
| Metadata routes | `internal/httpapi/sharing_routes.go` |
| Worker recalc | `internal/worker/sharing_recalc.go` |
| Query SQL | `internal/dataengine/sharing_sql.go` |

## Agent checklist

- [ ] Read ADR-016 before changing sharing enforcement
- [ ] Do not conflate API `roles` with `data_roles`
- [ ] Permission sets gate object access; sharing is record-level only
- [ ] Enforce the same record checks on HTTP, MCP, and composite (no bypass plane)
- [ ] Require `authz.manage` for data-role assignment and sharing config
- [ ] Reject empty `criteria.filters` (never silently share all records)
- [ ] Invalidate rule grants synchronously on rule PATCH/DELETE; rebuild via `sharing.recalc`
- [ ] Reject data-role hierarchy cycles
- [ ] Enqueue `sharing.recalc` after rule/OWD/data-role changes (`hierarchy` ≡ full rebuild)
- [ ] Update BP-003 when materially de-risking
