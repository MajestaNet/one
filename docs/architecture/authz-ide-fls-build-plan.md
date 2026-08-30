# AuthZ security build plan — IDE functions + bulletproof FLS

**Status:** Active implementation  
**Playbooks:** [agent-authz.md](./agent-authz.md) · [agent-control-ide.md](./agent-control-ide.md)  
**Backlog:** [BP-003](../../backlog/BP-003-enterprise-auth.md) · [BP-022](../../backlog/BP-022-client-access-ide-device.md)

## Goals

1. Permission sets grant Control IDE **modes** (Operate / Build / Ship / Govern) and **every rail tool** via additive `ide.*` system capabilities.
2. Field-level security is **deny-by-default** with OR-union across assigned permission sets (additive only), enforced on every Client record path.

## Models (locked)

| Concern | Model |
|---|---|
| IDE chrome | Extend `permission_sets.system_permissions` with `ide.*` (no separate `uiAccess` table) |
| FLS | Materialize field stubs on every PS; deny-if-absent evaluator; one-shot freeze migration |

Roles still grant API family scopes only. `ide.*` never replaces `client` / `metadata` / `deploy`. Server remains AuthZ SoR; IDE chrome is fail-closed after `/me` loads.

## Capability catalog (`ide.*`)

Mode caps: `ide.operate`, `ide.build`, `ide.ship`, `ide.govern`.  
Tool caps: `ide.{mode}.{tileId}` for each entry in Control IDE `MODE_WORKSPACE_TOOLS` (see [system-capabilities.md](./system-capabilities.md)).  
Also registered: `debug.read`, `debug.trace`.

Semantics: mode + tool both required for chrome; OR-union across assigned PSs; `IsAdmin` implies all.

## FLS deny-by-default

1. Freeze: insert missing `(ps, object, field)` rows as `can_read=true, can_edit=true` (preserves pre-change allow-if-absent effective access).
2. New fields: Admin = grant; other PSs = deny stubs.
3. Evaluator: field absent from effective OR map ⇒ deny (strip / forbid edit).
4. Client `describe` / `describe/{object}` filtered by object read (+ readable fields).

## Phases

| Phase | Deliverable |
|---|---|
| 0 | This doc + BP / capability catalog docs |
| 1 | Go `ide.*` / `debug.*` catalog, seed packs, migration upsert, `/me` tests |
| 2 | FLS freeze + deny-if-absent + catalog stubs + multi-PS tests |
| 3 | Describe filtering + path audit (upsert / ingest / MCP / events) |
| 4 | IDE `scopes.ts` fail-closed + PermissionsPanel checkboxes + Vitest |

## Non-goals

- Assigning permission sets through Roles
- AuthZ policy in JWT claims
- Replacing Role family scopes with `ide.*`
- Full Metadata object-list FLS (Client describe is in scope)
