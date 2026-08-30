# Cross-object search + Operate find/bulk (build plan)

**Active plan** that **merges and closes** [BP-043](../../backlog/BP-043-cross-object-search-api.md) and [BP-020](../../backlog/BP-043-cross-object-search-api.md) in one workstream: a product-owned **Client search API** over **indexed searchable fields**, consumed by an always-on **Operate global search bar**, plus **list bulk actions** on Object Home (existing composite — no second search store, no Elasticsearch).

**Playbooks:** [agent-data-architecture.md](./agent-data-architecture.md) · [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md) · [agent-control-ide.md](./agent-control-ide.md)  
**Domain agents:** `db-backend-perf` (index + DataEngine) → `api-families` (Client route) → `authz-security` (visibility/FLS) → `worker-jobs` (reindex) → `control-ide` (Operate chrome + bulk). Cross-plane: API logic stays in Go; the IDE is a JWT Client.  
**Related:** [ADR-003](../adr/003-sql-query-engine.md) · [ADR-004](../adr/004-three-api-families.md) · [ADR-013](../adr/013-high-volume-flexible-storage.md) · [ADR-016](../adr/016-record-sharing.md) · [ADR-017](../adr/017-canonical-field-types.md) · [ADR-025](../adr/025-api-revision-versioning.md) · [api-families.md](../api-families.md) · [customer-ide-ux.md](../customer-ide-ux.md) · [BP-001](../../backlog/BP-001-jsonb-query-scale.md) · [BP-003](../../backlog/BP-003-enterprise-auth.md) · [BP-035](../../backlog/BP-035-records-list-partition-covering.md) · [BP-018](../adr/030-install-agent-runtime.md) · [BP-019](../adr/030-install-agent-runtime.md) · [BP-024](../adr/030-install-agent-runtime.md) · [BP-041](../../backlog/BP-041-record-external-id-upsert-bulk.md) · [BP-042](../../backlog/BP-042-change-feed-cdc-consumer.md) · [BP-046](../../backlog/BP-046-record-merge-dedupe.md)

---

## Thesis

> CRM find is **“type a name, email, or phone and land on the record.”** The product owns a ranked, AuthZ-honest **Client** search surface over a **maintained search document** plus **trigram indexes**. Control IDE Operate keeps a **global search bar always visible in the top bar**. List hygiene (assign owner, change status/stage) uses **existing** `POST /client/v1/composite` — not a new bulk engine and not IDE-local scan.

```text
Metadata searchable=true on declared fields
  → DataEngine maintains records.search_document / search_title / search_subtitle on CRUD
  → pg_trgm GIN on search_document (LIST-partitioned records + records_hv)
  → POST /client/v1/search  (scope: client; sharing + FLS)
       → Control IDE Operate top-bar find
       → agents / MCP / sdk/client
Object Home multi-select
  → POST /client/v1/composite PATCH (owner / status / stage)
```

**Close-out rule:** both BPs move to **Mitigated** only when the exit checklist at the bottom is green. Do not close 043 without the Operate bar, and do not close 020 with a fake IDE-only scan.

---

## Current state (baseline)

| Surface | Today | Gap |
|---|---|---|
| Field metadata | `indexed` / `filterable` / `sortable`; btree/cast expression indexes via `field_projections` | No `searchable`; Phone/Email/Name find is not a product API |
| Query | `POST /client/v1/query` — one object, SQL filters; `like` requires `indexed=true` | No cross-object ranked find; LIKE is per-field, not “type and go” |
| JSONB GIN | Dropped (`0031_drop_records_data_gin`, BP-001) | Must **not** revive global `records.data` GIN |
| LIST partitions | `records` partitioned by `object_api_name` (`0037`, BP-035) | Search must predicate `object_api_name` so partitions prune |
| Covering projections | Open remainder of BP-035 | Search uses a **dedicated document column**, not matviews |
| Client HTTP | query / sobjects / composite / sync bulk insert / ingest jobs | No `/search` |
| Operate chrome | Top bar: brand · mode title (center) · env · theme · session | No find field |
| Object Home | `RunObjectHomePanel` already has `selectedRowIds` + “Add to chat” | No mass update / assign |
| Agents / MCP / SDK | `query`, `sobjects.read` | Cannot call a stable find API |
| Contact phones | `MobilePhone` / `HomePhone` / `Fax` — **not** indexed; no `Phone` field | Seed must mark these searchable (do not invent `Contact.Phone`) |

---

## Locked decisions

| Decision | Choice | Rationale |
|---|---|---|
| One workstream | Execute this plan; close **BP-043 and BP-020 together** | User ask; IDE must not own a search store |
| Family / path | **Client** `POST /client/v1/search` (`scope: client`). Register on `/v1` alias via existing `registerClientFamily`. | ADR-004; matches query/composite |
| GET alias | Optional `GET /client/v1/search?q=` for 1–2 objects max; **POST is canonical** (object list + options) | Avoid exploding query strings |
| API revision | **Additive.** Do not bump `apiRevision` (ADR-025). Document the route on the describe/version surface if one already lists Client capabilities. | Old pinned clients ignore unknown routes |
| Search store | **Postgres only.** Maintained columns on `records` and `records_hv`. | Dedicated install DB; ADR-002/003; BP-043 non-goal is ES |
| Index type | **`pg_trgm` GIN** on `search_document` (+ `search_title`). Not `tsvector` as the primary matcher. | CRM find is prefix/contains on names, emails, phones — stemming hurts |
| vs `indexed` | `searchable` is a **separate** metadata flag. It does **not** force btree `indexed`. Btree `indexed` remains for query equality/range/LIKE. | Different access path |
| Eligible types | `text`, `email`, `phone`, `url`, `autonumber` only. Reject `searchable` on textarea, richtext, lookups, picklists, booleans, numbers, compounds, json. | Keep the document small and find-shaped |
| Document | Concatenated **normalized** searchable values: lowercase, collapsed whitespace; for `phone`, also append **digits-only**. Title/subtitle denormalized for the hit row. | One index probe per object partition |
| Maintenance | **Synchronous on Create/Update/Delete** in DataEngine. Worker `search.reindex` for backfill and after searchable-metadata changes. | CRUD is source of truth; reindex is catch-up |
| CDC | Do **not** wait on [BP-042](../../backlog/BP-042-change-feed-cdc-consumer.md). Optional later consumer. | Close 043 without CDC |
| AuthZ | Same as query: `AssertObjectAccess(read)` per candidate object; **SQL visibility pushdown** via `QueryVisibility` (owner/creator or sharing, ADR-016). Admin / view-all bypass unchanged. | No post-filter paging holes |
| FLS | Hits return only FLS-readable snippet fields. Title falls back to `object` + `Id` when name fields are unreadable. Matching may include searchable fields the principal cannot read (**existence leak accepted in v1**; do not put secrets on searchable fields). | Matches query-filter reality; document it |
| Objects in a search | Default: every object the principal can **read** that has ≥1 `searchable` field (enabled packages only). Optional `objects[]` allowlist. Skip objects with zero searchable fields. | Package-gated Case/Opportunity appear when modules are on |
| HV | Search **includes** `high_volume` objects. Trigram match on `search_document` is the selective predicate (HV analog of indexed filter). | Messages can be found by subject-like searchable fields if marked |
| Limits | `q` min length **2** (or **3 digits** if the query is digits-only). Default `limit` **20**, max **50**. Timeout budget: one SQL round-trip, `statement_timeout` inherited. | Snappy chrome; no locator-scale scan |
| Ranking | Exact title → prefix title → trigram `similarity` on title then document → `updated_at` desc. Tie-break `id`. | Type-a-few-chars CRM feel |
| Empty `q` | **400** `VALIDATION_ERROR` — never “list everything”. | Prevents partition scans |
| User / kernel | **Out of scope.** User is not a flexible record object ([BP-058](../../backlog/BP-058-user-identity-extension.md)). | Do not search `users` in v1 |
| Operate chrome | When `section === "operate"` **and** JWT connected: find is a **persistent command bar on the graph tile** ([ADR-028](../adr/028-operate-graph-surface.md)). v1 shipped a top-bar field; that placement is superseded — do not put search in the app header (it shifts Mode title / Env / session). `Ctrl/Cmd+K` focuses the graph bar (reopens the graph tile if closed). Other modes keep today’s centered mode title. | User end-goal |
| Hit gesture | Click / Enter → **ensure + focus the matching collection** on the graph and select the record in the list sheet. Optional secondary: pin to graph / add to chat excerpt. Do not open a swapped Object Home tile from the command bar. | Lands on the record in graph context |
| Bulk | Object Home selection toolbar: **Assign owner**, **Change Status/Stage** (describe picklist when present). Execute via `POST /client/v1/composite` PATCH, **max 25** ids per submit. Per-row 403/400 surfaced. | Closes BP-020 without new ingest jobs |
| Duplicate merge | **Not this plan** — [BP-046](../../backlog/BP-046-record-merge-dedupe.md). | Explicit BP-020 later item |
| CSV import | **Not this plan.** | BP-020 non-goal |
| Incumbent CRM search grammar | **Not this plan.** Majesta One JSON contract only. | BP-043 non-goal |

### Explicit non-goals

- Elasticsearch / OpenSearch / a second search appliance
- Incumbent CRM search grammar
- Full-text on all JSONB keys
- Reviving global GIN on `records.data`
- Materialized covering projections (stays BP-035)
- Searching textarea/richtext bodies in v1
- Admin mass-transfer of the whole org
- New async Bulk update job type (BP-041 ingest stays ETL; Operate bulk is composite)
- In-kernel email / CTI screen-pop wiring (BP-024 C consumes this API later)

---

## Product UX (Operate)

When the user is **in Operate** and connected:

```text
┌─ Top bar: brand · [centered Operate ▾] · Env · ☀ · JWT ─────────────┐
┌─ Graph tile ────────────────────────────────────────────────────────┐
│ [  Find records, tools, objects…                          ⌘K ]      │
└─────────────────────────────────────────────────────────────────────┘
```

| Rule | Behavior |
|---|---|
| Visibility | Search field is **always shown on the Operate graph tile** when that tile is open (connected). Hidden in the app top bar. `⌘K` reopens the graph if needed. Hidden when signed out or in Build / Govern / Settings. |
| Placeholder | `Search records…` (or `Search Account, Contact…` once describe cache knows enabled objects) |
| Debounce | 200ms after last keystroke; cancel in-flight if `q` changes |
| Min chars | Mirror server (2 / 3 digits); below that show hint, do not call |
| Results | Popover under the field: grouped by object label, max 20. Each row: **title**, object badge, subtitle (email/phone/account number). Keyboard ↑↓ Enter Esc. |
| Loading / empty / error | Honest: spinner, “No matching records”, Client 403/400 text. No fake seed hits. |
| Offline | Disabled + Connect CTA |
| Shortcut | `Ctrl/Cmd+K` focuses the field; does **not** replace the visible bar |
| AuthZ | 403 objects omitted from groups; never invent hits |
| Accessibility | `role="combobox"` / listbox; label “Search records” |

**Not** a left-rail inspect tool (that is Query / Monitor / Explorer — [BP-034](../adr/030-install-agent-runtime.md)). Search is **graph chrome**. Hit landing: ensure a collection + select the row; do not swap to a List View workspace tile. Pin is a secondary action ([ADR-028](../adr/028-operate-graph-surface.md)).

### Bulk (Object Home)

`RunObjectHomePanel` already tracks `selectedRowIds`. Add a selection bar when `size > 0`:

| Action | Fields | API |
|---|---|---|
| Assign owner | `OwnerId` (user picker from existing principal/list patterns, or paste user id if no picker yet) | composite PATCH |
| Change status / stage | `Status` or `StageName` when describe says picklist + updateable | composite PATCH |
| Clear selection | UI only | — |

Show a result toast: `12 updated, 3 forbidden`. Do not hide 403s. Disable actions the principal cannot attempt (object `update` grant). Cap 25; if more selected, require another submit (“Update next 25”).

Pipeline / Case queue boards from [BP-019](../adr/030-install-agent-runtime.md): **reuse the same composite helper** if those lists already expose row ids; do not block search close-out on unfinished domain boards. Object Home bulk is sufficient to close BP-020’s bulk bullet.

---

## 1. Metadata: `searchable`

**Packages:** `internal/metadata`, `internal/packages`, `internal/seed`, `internal/deploy`, `migrations/`  
**Agent:** `db-backend-perf` (+ metadata write path)

### 1.1 Kernel

Migration `0057_record_search.sql` (after `0056_install_auth_provisioning`; confirm journal before landing):

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS searchable boolean NOT NULL DEFAULT false;

ALTER TABLE records
  ADD COLUMN IF NOT EXISTS search_document text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_subtitle text NOT NULL DEFAULT '';

ALTER TABLE records_hv
  ADD COLUMN IF NOT EXISTS search_document text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_title text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS search_subtitle text NOT NULL DEFAULT '';

-- Parent indexes propagate to LIST partitions.
CREATE INDEX IF NOT EXISTS records_search_document_trgm_idx
  ON records USING gin (search_document gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_search_title_trgm_idx
  ON records USING gin (search_title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_hv_search_document_trgm_idx
  ON records_hv USING gin (search_document gin_trgm_ops);
CREATE INDEX IF NOT EXISTS records_hv_search_title_trgm_idx
  ON records_hv USING gin (search_title gin_trgm_ops);
```

`EnsureFlexiblePartition` / HV attach paths must copy the new columns (`LIKE records INCLUDING DEFAULTS` already does if parent has them — verify attach helpers still `LIKE` the parent).

Journal: add `0057_record_search` to `migrations/meta/_journal.json`.

### 1.2 Write rules (`internal/metadata`)

On create/update/sync:

1. If `searchable=true`, `fieldType` must be in the allowlist (`text|email|phone|url|autonumber`).
2. `searchable=true` ⇒ `filterable=true` (describe clients can show the field). Do **not** auto-set `indexed`.
3. Clearing `searchable` does not drop btree indexes.
4. Describe JSON exposes `searchable`.
5. Metadata write that **changes** searchable set for an object enqueues worker job `search.reindex` with `{ "objectApiName": "Account" }`.
6. Customer YAML / Deploy snapshot / `one/v1` field files grow optional `searchable: true`.

`internal/packages.FieldDef` and deploy apply types get `Searchable bool`.

### 1.3 Managed seed (v1 searchable map)

Bump package versions when defs change (`CorePackageVersion` 2.0.0 → **2.1.0**; sales/service/catalog/notes/address similarly).

| Package | Object | Searchable fields |
|---|---|---|
| `core` | Account | `Name`, `AccountNumber`, `Phone`, `Website` |
| `core` | Contact | `FirstName`, `LastName`, `Email`, `MobilePhone`, `HomePhone` |
| `sales` | Opportunity | `Name` |
| `sales` | Quote | `Name` |
| `service` | Case | `Subject` |
| `service` | Asset | `Name`, `SerialNumber` |
| `service` | WorkOrder | `Subject` |
| `catalog` | Product | `Name`, `ProductCode`, `StockKeepingUnit` |
| `notes` | Note | `Title` |
| `address` | Address | `Name`, `PostalCode` |
| `activities` | Task / Appointment / PhoneCall / Email | `Subject` (confirm apiNames in seed; mark the human title field only) |
| `lead_marketing` | Lead | `LastName`, `FirstName`, `Email`, `Company` (if present) |

Do **not** mark lookups, picklists (`StageName`, `Status`), or `Description` textarea. Industry packs: mark `Name` / email / phone analogs in the same PR if those objects are find-worthy; otherwise a follow-up is allowed **only** if core+sales+service+catalog are done (Operate v1 must find Account/Contact/Opportunity/Case).

Update [data-model.md](../data-model.md) field tables: add `searchable` on those rows.

---

## 2. DataEngine: document + `Search`

**Packages:** `internal/dataengine`  
**Agent:** `db-backend-perf`

### 2.1 Normalize + document

New helpers (suggested `internal/dataengine/search.go`):

```go
func NormalizeSearchQuery(q string) (normalized string, digits string, err error)
func BuildSearchDocument(fields []metadata.FieldDefinition, data map[string]any) (document, title, subtitle string)
```

Rules:

- Skip non-searchable fields.
- Coerce values to string; skip empty.
- Lowercase; `strings.Join` with a single space.
- `phone`: append `digitsOnly(value)` as an extra token.
- **Title:** first present among `Name`, `Subject`, `Title`, else `FirstName + LastName`, else `LastName`.
- **Subtitle:** first present among `Email`, `Phone`, `MobilePhone`, `AccountNumber`, `SerialNumber`, `ProductCode` that is **not** already the title.

Call `BuildSearchDocument` from `Create` and `Update` **after** validate/coerce, and persist the three columns in the same `INSERT`/`UPDATE` as `data`. `Delete` removes the row (hard delete) — nothing to maintain.

Do not compute documents in HTTP handlers.

### 2.2 `Search` API

```go
type SearchRequest struct {
    Query   string   `json:"q"`
    Objects []string `json:"objects,omitempty"`
    Limit   int      `json:"limit,omitempty"`
}

type SearchHit struct {
    ID        string `json:"id"`
    Object    string `json:"object"`
    Title     string `json:"title"`
    Subtitle  string `json:"subtitle,omitempty"`
    UpdatedAt string `json:"updatedAt"`
    Score     float64 `json:"score"`
}

type SearchResult struct {
    Hits  []SearchHit `json:"hits"`
    Query string      `json:"query"`
}
```

Algorithm:

1. Normalize `q`; reject short queries.
2. Resolve candidate objects: intersection of (requested or all described flexible objects) × (principal `AssertObjectAccess` read) × (has searchable field). Drop failures silently for default-all; **400** if caller named an object they cannot read or that does not exist.
3. Split candidates by `storage_mode` (`records` vs `records_hv`).
4. For each store, **one** SQL with `object_api_name = ANY($1)` (partition prune) and:

```sql
WHERE object_api_name = ANY($objects)
  AND (
        search_title ILIKE $prefix          -- q || '%'
     OR search_document ILIKE $contains     -- '%' || q || '%'
     OR search_document % $q                -- trigram similar
  )
  -- AppendSharingVisibility(alias, ...)
ORDER BY
  (lower(search_title) = $q) DESC,
  (search_title ILIKE $prefix) DESC,
  similarity(search_title, $q) DESC,
  similarity(search_document, $q) DESC,
  updated_at DESC,
  id DESC
LIMIT $limit
```

5. Merge flexible + HV hit lists in memory by score then recency; cap to `limit`.
6. Strip/redact title/subtitle with FLS in the HTTP layer (or DataEngine if `SearchAuthz` is passed — prefer the same pattern as query: engine returns rows, HTTP strips).

`like`/`%` in user `q` are **stripped/escaped** — treat `q` as a literal needle, never a LIKE pattern from the client.

### 2.3 Tests (`internal/dataengine`)

| Test | Assert |
|---|---|
| Document includes Name + digits of Phone | `"acme"` and `"415555"` both in document |
| Search Name prefix | `q=acm` hits Account titled Acme |
| Search email | `q=jane@` hits Contact |
| Search phone digits | `q=415555` hits despite formatted `(415) 555-0100` |
| Cross-object | one query returns Account + Contact |
| Object filter | `objects:["Contact"]` excludes Account |
| AuthZ | non-owner with sharing off does not see others’ rows |
| Unindexed LIKE still rejected on `/query` | search path is separate — do not weaken query guardrails |
| Empty/short `q` | validation error |
| HV object | searchable Message/Note-class row is found when module enabled (or unit with routed table) |

---

## 3. Client HTTP

**Packages:** `internal/httpapi`  
**Agent:** `api-families`

| Item | Spec |
|---|---|
| Route | `POST {prefix}/search` inside `registerClientFamily` (`/client/v1` and `/v1`) |
| Handler | `handleSearch` — thin: parse body, object access, visibility, `data.Search`, FLS strip, JSON |
| Scope | `client` only |
| Errors | 400 validation; 401/403 via existing helpers; 503 if data-engine nil |
| Body | `{ "q": "acme", "objects": ["Account","Contact"], "limit": 20 }` |
| Response | `{ "query": "acme", "hits": [ { "id", "object", "title", "subtitle", "updatedAt", "score" } ] }` |

Integration: `internal/testutil` + `internal/httpapi` test — seed Account/Contact, search as StandardUser vs other user, assert 403 object omitted / sharing.

Optional GET: `q` required, `objects` comma-separated, `limit` query param. Skip GET if POST + IDE are enough; do not block close-out on GET.

---

## 4. Worker reindex

**Packages:** `internal/worker`, enqueue from `internal/metadata`  
**Agent:** `worker-jobs`

| Item | Spec |
|---|---|
| Job type | `search.reindex` |
| Payload | `{ "objectApiName": "Account" }` or omit for all objects with searchable fields |
| Work | Keyset scan `(created_at, id)` pages of 500; `BuildSearchDocument`; `UPDATE … SET search_document, search_title, search_subtitle` |
| Concurrency | One object at a time per job; BP-005 claim/lease already serializes the row |
| Failure | Retry via existing job attempts; do not fail user Metadata writes if enqueue succeeds |
| When | Searchable flag change; operator/admin may POST later if a Metadata rebuild route already exists for projections — **do not** invent a new family. Reuse projection rebuild UX only if it already takes a job name; otherwise worker-only is enough for v1. |

Boot after migrate: enqueue reindex for core objects so existing rows become findable without waiting for the next edit.

---

## 5. Control IDE

**Packages:** `tools/control-ide/**` only  
**Agent:** `control-ide`

### 5.1 Search bar

| File (expected) | Change |
|---|---|
| `src/renderer/App.tsx` | **Superseded by ADR-028:** do **not** render `OperateSearch` in `top-bar-center`. Mode title stays centered. Mount the combobox on `RunGraphHome` ([ADR-028](../adr/028-operate-graph-surface.md)) |
| `src/renderer/operate/OperateSearch.tsx` | Combobox; `POST /client/v1/search`; debounce |
| `src/renderer/operate/OperateSearch.test.tsx` | Types query, shows grouped hits, Enter opens object home, 403 empty, unsigned-in hidden |
| `src/renderer/styles.css` | `.operate-search` using existing tokens (`--nav`, `--accent`, `--line`); compact height matching top bar |
| `src/renderer/workspace/types.ts` | If needed, a handoff to open `object:{apiName}` with `selectedId` |

Keyboard: `Ctrl/Cmd+K` in Operate focuses the input (`data-testid="operate-global-search"`).

Opening a hit: reuse whatever Object Home open path graph/tools already use (`objectHomeRailId`). Pass `initialObjectApiName` + selected record id.

### 5.2 Bulk bar

| File | Change |
|---|---|
| `RunObjectHomePanel.tsx` | Selection toolbar when `selectedRowIds.size > 0`: Assign owner, Change status/stage, counts |
| Helper `operate/bulkComposite.ts` | Build ≤25 PATCH subrequests; parse per-row status |
| Tests | Selecting 2 rows + mock composite 200/403 mix |

Do not call `/query` with OR-LIKE from the IDE.

---

## 6. Agents, MCP, SDK

Complete BP-043’s “docs for agents and sdk/client”:

| Surface | Change |
|---|---|
| `internal/mcp` `ListTools` | Add `search` → `POST /client/v1/search` (`q`, `objects`, `limit`) |
| `internal/agentharness` | Allow `search` in the tool catalog; **do not** silently add it to every floor — customers opt in. Document that find UX for humans is the IDE bar; agents use the same API. |
| `tools/one-mcp` | `one_search` wrapping the same path |
| `sdk/client` | `search({ q, objects, limit })` |
| [api-families.md](../api-families.md) | Add Search row next to Query |
| [customer-connect.md](../customer-connect.md) | One line: Client search is the find API |

Harness floors stay `sobjects.read` + `query` unless a starter playbook explicitly wants `search`. Adding `search` to `agents_starter` Query demo is **allowed** and desirable.

---

## 7. AuthZ / FLS notes (`authz-security`)

- Object read required; no new scope.
- Reuse `buildQueryVisibility` per object (or a small helper that maps object → vis). For default-all, loop objects; do not build one visibility for mixed objects.
- Sharing SQL must use the same table alias as the search `FROM`.
- FLS: strip subtitle pieces; if title field unread, `title` becomes `""` and the IDE shows `object` + short id.
- Do not log `q` at info in production handlers (PII); debug only.

---

## Execution order (agents)

Execute in this order; do not start IDE until POST `/search` is green in Go tests.

| Phase | Owner | Deliverable | Exit |
|---|---|---|---|
| **0** | docs (this file) | Locked contract | This PR |
| **1** | `db-backend-perf` | Migration + `searchable` metadata + seed flags + versions | `go test` metadata/seed; migrate applies |
| **2** | `db-backend-perf` | Document on CRUD + `Search()` + unit tests | Name/email/phone/cross-object tests pass |
| **3** | `api-families` + `authz-security` | `POST /client/v1/search` + HTTP integration | Sharing + FLS cases pass |
| **4** | `worker-jobs` | `search.reindex` + enqueue on metadata change + boot backfill | Existing seeded rows become searchable |
| **5** | `control-ide` | Operate top-bar search | Vitest: bar visible in Operate, hidden elsewhere; hit opens Object Home |
| **6** | `control-ide` | Object Home bulk via composite | Vitest: 25-cap, mixed 403 |
| **7** | `api-families` / MCP / SDK | `search` tool + sdk method + docs | MCP list includes `search`; api-families table updated |
| **8** | any | Mark **BP-043 and BP-020 Mitigated**; README table + alignment buckets | Both IDs Mitigated in the same change set |

Phases 1–4 may land as one Go PR; 5–6 as one IDE PR; 7 can ride either. **Do not split 043 vs 020 across releases** — the human-visible bar is part of “search works.”

---

## Test plan (definition of done)

**Go**

```bash
go test -p 1 ./internal/dataengine/ ./internal/metadata/ ./internal/seed/ ./internal/httpapi/ ./internal/worker/ ./internal/mcp/
```

With `DATABASE_URL`: integration search as two users; phone digits; object filter; short `q` → 400.

**IDE**

```bash
make test-ide
```

Plus component tests named above. Live `make test-ide-integration` when API is up: create Account “Acme SearchCo”, type `acme` in Operate bar, see the hit.

**Manual (when executing, not this docs PR)**

1. Operate connected → search field visible without opening a tool.
2. Type name / email / phone → ranked hits.
3. Click hit → Object Home on that record.
4. `Cmd+K` focuses the field.
5. Switch to Build → field gone.
6. Select 3 rows → assign owner → two succeed / one 403 if sharing requires it.

---

## Exit checklist (both BPs Mitigated)

- [x] `searchable` on metadata + describe
- [x] Seed map in §1.3 applied and package versions bumped
- [x] `search_document` maintained on CRUD for flexible + HV
- [x] `pg_trgm` GIN in place; no `records.data` GIN revival
- [x] `POST /client/v1/search` ranked, AuthZ-honest, limit-capped
- [x] Worker backfill so pre-existing rows are findable
- [x] Operate top-bar search always visible in Operate when connected
- [x] Hit opens Object Home
- [x] Object Home bulk assign/status via composite (≤25)
- [x] MCP + `sdk/client` can call search
- [x] [api-families.md](../api-families.md) + this plan’s related BPs updated
- [x] BP-043 and BP-020 status **Mitigated**; README table + alignment buckets

Shipped in the Client search + Operate find/bulk workstream. Keep this file as the contract; do not reopen 043/020 for CDC, merge, or User search.

---

## Follow-ons (not close-out)

| Item | Where |
|---|---|
| CDC-maintained index | BP-042 |
| Duplicate hints in search results | BP-046 |
| Search User directory | BP-058 |
| Screen-pop / CTI | BP-024 C |
| Reporting | BP-021 |
| Covering matviews for analytics | BP-035 |
| textarea/richtext body search | new BP if a customer needs it |
| Relocate find from app top bar to graph command bar; land on collections | [ADR-028](../adr/028-operate-graph-surface.md) · [BP-060](../../backlog/BP-060-operate-graph-surface.md) · [ADR-028](../adr/028-operate-graph-surface.md) |
