# External ID, upsert, Bulk jobs, and data packs

**Active plan** for [BP-041](../../backlog/BP-041-record-external-id-upsert-bulk.md): give headless integrations **idempotent sync primitives** — external-ID metadata, REST upsert, industry bulk-job async jobs, and optional **portable data packs** for ordered multi-object loads between installs.

**Playbooks:** [agent-data-architecture.md](./agent-data-architecture.md) · [agent-api-families.md](./agent-api-families.md) · [agent-worker.md](./agent-worker.md) · [agent-authz.md](./agent-authz.md) · [agent-deploy.md](./agent-deploy.md)  
**Domain agents:** `db-backend-perf`, `api-families`, `worker-jobs`, `authz-security`, `deploy-ops`  
**Related:** [ADR-003](../adr/003-sql-query-engine.md) · [ADR-004](../adr/004-three-api-families.md) · [ADR-013](../adr/013-high-volume-flexible-storage.md) · [ADR-016](../adr/016-record-sharing.md) · [ADR-017](../adr/017-canonical-field-types.md) · [api-families.md](../api-families.md) · [multi-env-deploy.md](../multi-env-deploy.md) · [customer-repo.md](../customer-repo.md) · [BP-001](../../backlog/BP-001-jsonb-query-scale.md) · [BP-003](../../backlog/BP-003-enterprise-auth.md) · [BP-005](./agent-worker.md) · [BP-033](../../backlog/BP-033-customer-runtime-isolation.md) · [BP-035](../../backlog/BP-035-records-list-partition-covering.md) · [BP-046](../../backlog/BP-046-record-merge-dedupe.md)

---

## Thesis

> Integrations sync records by **external system keys**, not Majesta One UUIDs. Mark any eligible field as an external ID; upsert and Bulk jobs resolve create-vs-update from that key under normal AuthZ. Multi-object “box-to-box” sync uses **data packs** that store a **source peer** pointer in the customer repo (`environments/*.yaml`); the operator applies on the target with Connected App credentials — pull from source, upsert into target. No peer-push Deploy channel.

```text
ETL / iPaaS / one datapack apply
  → Metadata: field.externalId=true (+ unique projection)
  → Client REST upsert (sync, small)
  → Client Bulk 2.0 jobs (async insert|update|upsert|delete)
  → DataPack (sourceEnv peer + ordered steps)
       → auth source alias + target alias (Connected App)
       → query source → upsert target
```

---

## Current state (baseline)

| Surface | Today | Gap |
|---|---|---|
| Field metadata | `uniqueField`, `indexed`; expression indexes via `field_projections` | No `externalId` flag; unique fields do **not** get unique indexes |
| Record identity | Majesta One UUID `Id` only | No get/patch/upsert by external key |
| `POST /client/v1/bulk/{object}` | **Synchronous create-only** (`BulkInsert`) | Docs say “async via jobs”; no update/upsert/delete; no job status/results |
| Composite | POST/GET/PATCH/DELETE by Majesta One `Id` | No upsert method / external-ID path |
| Deploy / customer Git | Metadata packages + tests (`one/v1`) | Business data explicitly **not** promoted; no data-pack format |
| Multi-env | Repo→org; install→install promote **removed** | Data migration must not reintroduce peer artifact push |

Principals already have `users.external_id` (SCIM) — **out of scope** here (BP-041 non-goal).

---

## Locked decisions

| Decision | Choice |
|---|---|
| External ID model | Boolean metadata flag `externalId` on a field (customer or managed). Client **names** the field per request (`externalIdField`). Multiple external-ID fields per object allowed. |
| Implies | `externalId=true` ⇒ `uniqueField=true` and `indexed=true` (enforced on Metadata write). Clearing `externalId` does not auto-clear unique/indexed. |
| Eligible types | `text`, `email`, `phone`, `url`, `autonumber`, `integer` only (ADR-017). Reject on `textarea`, `richtext`, lookups, picklists, booleans, compounds, `json`, etc. |
| Uniqueness | Partial **unique** expression index on `(object_api_name, data->>field)` for external-ID (and, as a follow-on hardening, for `uniqueField` generally). Empty/null values excluded from uniqueness. |
| Sync upsert | `PATCH /client/v1/sobjects/{object}/{externalIdField}/{externalId}` (+ optional `GET` by same path). Alias: `POST .../sobjects/{object}/upsert` with `{ externalIdField, externalId, ...fields }`. |
| Match semantics | Exact string equality on JSONB text extract (integer fields coerced to canonical decimal string without scientific notation). Case-sensitive. |
| Duplicate on upsert | If unique index finds **more than one** row (legacy dirty data), fail `409 CONFLICT` / `DUPLICATE_EXTERNAL_ID` — do not merge (merge is [BP-046](../../backlog/BP-046-record-merge-dedupe.md)). |
| AuthZ on upsert | Resolve existing → `update` + record sharing + FLS; missing → `create` + FLS. Never escalate via upsert. |
| Bulk ownership | **Client** family only. Async via `jobs` + worker ([BP-005](./agent-worker.md)). |
| Bulk shape | industry bulk-job job lifecycle (create → upload → complete → poll → results). **Not** wire-compatible with other CRM bulk APIs. |
| Bulk operations v1 | `insert`, `update`, `upsert`, `delete`. **No** bulk query / SQL export in v1 (leave enum room; CDC/search stay BP-042/043). |
| Bulk payload | **JSON Lines** first-class; CSV optional Phase 4b. UTF-8. |
| Sync bulk route | Keep `POST /bulk/{object}` as **small sync insert** helper (cap rows). Document as non-job path; new work uses `/jobs/ingest`. |
| Data packs | Manifest `one-datapack/v1` under customer Git `data/`. **Primary source = peer org** referenced via `environments/<role>.yaml` (installId + baseUrl). Optional inline JSONL files remain a secondary seed path. |
| Source ↔ target | Pack stores **sourceEnv** (peer pointer). Operator runs apply **against the target** install; tooling resolves `environments/{sourceEnv}.yaml`, authenticates to the source with a Connected App / saved org alias (`one auth login`), queries/exports rows, upserts into the target. No peer-push Deploy channel. |
| Pack vs Deploy | Packs are **data**, not metadata. Deploy remains metadata/tests. Git holds the pack recipe + peer refs (not PII dumps by default). |
| Reference rewrite | Lookups rewritten via parent external IDs during pull→upsert; Majesta One UUIDs stay install-local. |

---

## 1. External ID metadata

### 1.1 Schema

Add column on `metadata_fields`:

```sql
ALTER TABLE metadata_fields
  ADD COLUMN IF NOT EXISTS external_id boolean NOT NULL DEFAULT false;
```

API / YAML: `externalId: true` on field definitions (Metadata HTTP + `one/v1` field YAML + Deploy snapshot types).

### 1.2 Write rules (`internal/metadata`)

On create/update/sync:

1. If `externalId=true`, field type must be in the allowlist.
2. Force `uniqueField=true`, `indexed=true`, `filterable=true`.
3. Enqueue / require projection build (unique index DDL — see §1.3).
4. Managed seed may mark fields (e.g. future ERP keys); customer fields work on any flexible object including custom objects and extensions on Account/Contact.
5. Describe exposes `externalId` so clients can discover upsert keys.

### 1.3 Unique projections

Today `BuildFieldProjections` creates **non-unique** expression indexes. For external ID (and preferably all `uniqueField`):

```sql
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ...
  ON records ((data ->> 'ERP_Id__c'))
  WHERE object_api_name = 'Account'
    AND NULLIF(data ->> 'ERP_Id__c', '') IS NOT NULL;
```

Same pattern on `records_hv` when `storage_mode=high_volume`.

| Concern | Rule |
|---|---|
| Dirty data | Unique index build fails → projection `failed` + clear Metadata error; operator must dedupe before enabling |
| Race | Upsert uses `INSERT … ON CONFLICT` on the unique expression **or** select-for-update by key then insert/update in one transaction |
| Selectivity | Partial unique indexes stay selective (BP-001 / BP-035); do not revive GIN for this path |

---

## 2. DataEngine upsert path

**Packages:** `internal/dataengine`  
**Agent:** `db-backend-perf`

### 2.1 APIs

```go
GetByExternalID(ctx, object, externalIdField, externalIdValue) (SObjectRecord, error)
Upsert(ctx, object, externalIdField, externalIdValue, input, actor, az UpsertAuthz) (result UpsertResult, error)
// UpsertResult: { Record, Created bool }
```

`UpsertAuthz` mirrors composite AuthZ hooks (object CRUD, sharing, FLS strip/assert).

### 2.2 Algorithm

1. Validate `externalIdField` exists on object and `externalId=true` (or allow any `uniqueField`? **No** — only fields flagged `externalId`, so operators opt in to upsert keys).
2. Normalize value; reject empty.
3. Lookup by expression equality on partition table for object.
4. If found: AuthZ update + `Update`; set `Created=false`.
5. If not found: AuthZ create; merge `{externalIdField: value}` into body; `Create`; `Created=true`.
6. Unique violation mid-flight → retry once as update, else `409`.

### 2.3 Update-by-external-id without full upsert

`Update` / `Delete` variants that accept external ID (Bulk `update`/`delete` rows identify by external ID or by Majesta One `Id`). REST: optional `PATCH`/`DELETE` on the external-ID path.

---

## 3. Client REST surface

**Packages:** `internal/httpapi`  
**Agent:** `api-families` (+ `authz-security`)

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/client/v1/sobjects/{object}/{externalIdField}/{externalId}` | Get by external ID |
| `PATCH` | same | Upsert (external-id shaped) |
| `DELETE` | same | Delete matching row |
| `POST` | `/client/v1/sobjects/{object}/upsert` | Body includes `externalIdField` + value + fields |
| Composite | method `UPSERT` **or** PATCH with `externalIdField`/`externalId` instead of `id` | Sequential; `@{ref.Id}` still works after upsert |

Response headers/body: include `created: true|false` (JSON field) so clients can branch without inferring from status alone. Prefer `200` for update and `201` for create on sync upsert (other CRMs often use 200/201 similarly).

Extend describe docs in [api-families.md](../api-families.md).

---

## 4. Bulk API (2.0–inspired)

### 4.1 Why not wire-compat

Incumbent bulk-job URLs, CSV-only ingest, and job state names are vendor-specific. Majesta One should be **familiar** to integrators but native:

- Family prefix `/client/v1`
- JSON Lines default (maps cleanly to JSONB)
- Explicit AuthZ + sharing (ADR-016)
- Job rows in Postgres `jobs` + a small `ingest_jobs` kernel table for durable status/results

### 4.2 Job lifecycle

```text
POST   /client/v1/jobs/ingest
       { object, operation, externalIdField?, contentType, lineEnding?, columnDelimiter? }
→ 201 { id, state: "Open" }

PUT    /client/v1/jobs/ingest/{id}/batches
       body: application/x-ndjson  (or text/csv)
→ 202 { id, bytesReceived, rowCountEstimate? }

PATCH  /client/v1/jobs/ingest/{id}
       { state: "UploadComplete" }   // close for processing
→ 200 { id, state: "UploadComplete" | "InProgress" }

GET    /client/v1/jobs/ingest/{id}
→ 200 { id, state, object, operation, counts, errorMessage?, createdAt, completedAt? }

GET    /client/v1/jobs/ingest/{id}/successfulResults
GET    /client/v1/jobs/ingest/{id}/failedResults
→ 200 NDJSON (Id, externalId?, created?, error?)

DELETE /client/v1/jobs/ingest/{id}   // abort if Open / UploadComplete; tombstone results if Complete
```

States: `Open` → `UploadComplete` → `InProgress` → `JobComplete` | `Failed` | `Aborted`.

### 4.3 Operations

| Operation | Row identity | AuthZ |
|---|---|---|
| `insert` | none (always create) | create |
| `update` | `Id` **or** `externalIdField` value in row | update + sharing |
| `upsert` | requires job-level `externalIdField`; value in each row | create or update |
| `delete` | `Id` or external ID column | delete + sharing |

### 4.4 Limits (v1 product ceilings)

| Limit | v1 value | Notes |
|---|---|---|
| Max rows per job | 100_000 | Hard reject on close if exceeded |
| Max upload bytes | 100 MiB | Across all batches for one job |
| Max open ingest jobs / principal | 5 | Admission; pairs BP-033 job-class budgets when present |
| Max concurrent InProgress ingest / install | 2 | Worker lane `ingest` |
| Batch chunk process size | 500 rows / tx | Balance outbox/automation fan-out |
| Partial success | **default on** | Per-row errors in failedResults; job still `JobComplete` if any success |
| `allOrNone` | optional job flag | If true, abort remaining chunks on first chunk failure; mark `Failed` |
| Serial mode | optional `assignmentRule` N/A; use `processingMode: Serial\|Parallel` | **Serial** required when pack apply needs parent-before-child within one object? Prefer **pack-level ordering across objects**; within object Parallel OK if no self-FK |
| Automations / outbox | fire per successful row (same as CRUD) | Document load risk; optional later `bypassAutomations` admin-only — **not** in v1 |
| Soft-delete / recycle | none | Hard delete only (product policy) |
| Bulk query | deferred | Use `/query` + keyset or BP-042 |

### 4.5 Storage

Kernel table `ingest_jobs`:

| Column | Purpose |
|---|---|
| `id` | UUID PK (also optional pointer from `jobs.payload`) |
| `actor_id` | Principal |
| `object_api_name`, `operation`, `external_id_field` | Job spec |
| `content_type`, `state` | Lifecycle |
| `upload_bytes`, `row_count` | Counters |
| `success_count`, `failure_count` | Results |
| `payload_oid` / `bytea` / side table `ingest_job_batches` | Staged upload |
| `result_success`, `result_failed` | NDJSON stored in side table or large objects |
| timestamps | created / completed |

Worker job type: `ingest.process` with `{ ingestJobId }`. Claim/lease unchanged (BP-005). Map into BP-033 execution budgets as class `ingest` when that lands.

### 4.6 Compatibility with today’s `POST /bulk/{object}`

- Retain as sync insert for ≤500 rows (or current practical body limit).
- Response shape unchanged.
- Docs: prefer `/jobs/ingest` for production ETL.

---

## 5. Data packs (`one-datapack/v1`) — peer-sourced

Typical CRM pain migrating **config + data** between orgs mixes Metadata API with Data Loader and brittle Id remaps. Majesta One splits that cleanly:

| Concern | Mechanism |
|---|---|
| Config (objects, fields, automations, …) | Existing Deploy / `one/v1` repo→org |
| Business records | **Data packs** — recipe in Git; **rows pulled from a source peer** at apply time |

### 5.1 Source ↔ target (peer reference in customer repo)

**Do not** push data install→install via Deploy. **Do** store the peer pointer next to metadata:

1. `environments/<role>.yaml` already holds `installId`, `installRole`, `baseUrl` (non-secret).
2. Pack manifest sets `sourceEnv: test` (filename stem / alias under `environments/`).
3. Operator runs apply **on the target** (`--org prod` / Connected App against prod).
4. Tooling loads `environments/test.yaml` → source `baseUrl`, uses a **source** auth alias / Connected App (client credentials) to query the source Client API, then upserts into the target session.
5. Correlation key = each step’s `externalIdField`. Majesta One UUIDs remapped via reference rules.

Peers (`/deploy/v1/peers`) and `environments/*.yaml` are the same topology idea — Git is the portable copy of those pointers. Secrets never live in Git (`auth login` / Connected App on the operator machine or CI).

```text
[customer Git]
  environments/test.yaml   → installId + baseUrl (source peer)
  environments/prod.yaml   → installId + baseUrl (target peer)
  data/crm-seed/datapack.yaml  → sourceEnv: test + object steps

Operator (CI or laptop):
  auth login --alias src  --base-url <test>  (Connected App / token)
  auth login --alias dest --base-url <prod>
  one datapack apply data/crm-seed --org dest --source-alias src
       → query/export from src → upsert into dest (Bulk / REST)
```

Optional `file:` JSONL on a step remains for offline/demo seeds when no live peer is available.

### 5.2 Manifest shape

```yaml
apiVersion: one-datapack/v1
name: crm-seed
version: "1.0.0"
description: Pull reference CRM rows from test into the connected target
sourceEnv: test                    # → environments/test.yaml
requires:
  objects: [Account, Contact]
steps:
  - id: accounts
    object: Account
    operation: upsert
    externalIdField: ERP_Id__c
    query:
      select: [Id, Name, ERP_Id__c]
    # file: accounts.jsonl       # optional offline alternative
  - id: contacts
    object: Contact
    operation: upsert
    externalIdField: ERP_Id__c
    after: [accounts]
    query:
      select: [Id, FirstName, LastName, ERP_Id__c, AccountId]
    references:
      - field: AccountId
        toObject: Account
        toExternalIdField: ERP_Id__c
```

Apply pipeline per step:

1. Resolve `sourceEnv` → `baseUrl` from customer repo environments.
2. Authenticate to source (`--source-alias`) and target (`--org`).
3. Pull rows (query) or read `file:`; rewrite lookups via `references`.
4. Upsert into target (Bulk ingest when large; sync upsert when small).
5. Emit apply report (counts + failed external ids).

### 5.3 What packs intentionally skip

- Users / principals / Role assignments (identity stays SCIM / Client identity APIs)
- Secrets, connector tokens, OAuth grants
- Managed package internals as data
- Cross-`CUSTOMER_ID` copies
- Automatic metadata create from pack rows (schema must exist on both peers)
- Storing raw PII dumps in Git when a live `sourceEnv` works

### 5.4 CLI / DX (implementation Phase 3)

Under `cmd/one` (pairs BP-048):

| Command | Role |
|---|---|
| `datapack validate <dir>` | Manifest + DAG + `sourceEnv` resolves to `environments/*.yaml` |
| `datapack apply <dir> --org <targetAlias> --source-alias <srcAlias>` | Pull from source peer → upsert target |
| `datapack apply <dir> --org <alias> --offline` | Use step `file:` only (no peer pull) |

IDE Ship remains metadata-focused; a later Operate/Build affordance can wrap apply (out of BP-041 critical path).

---

## 6. AuthZ, audit, observability

| Front | Control |
|---|---|
| Object CRUD | Same permission sets as single-row APIs |
| Sharing | ADR-016 on update/delete/upsert-hit; create uses normal owner defaults |
| FLS | Assert editable fields on written keys; strip unreadables on results |
| Job visibility | Actor can GET own ingest jobs; admin/`viewAll` style TBD — default **owner + admin** |
| Audit | `ingest.job` summary event; per-row relies on existing create/update/delete audit/outbox |
| OTEL | Spans `ingest.process`, attributes object/operation/counts |
| Runtime isolation | Job class `ingest` when BP-033 lands; until then hard caps in §4.4 |

---

## 7. Implementation phases (build in three)

### Phase 0 — Docs + backlog alignment

This file; BP-041 Design link; peer-sourced pack decision; cross-links.

**Status:** Done (design PR)

### Phase 1 — External ID + REST upsert

**Packages:** `migrations/`, `internal/metadata`, `internal/dataengine`, `internal/httpapi`, `internal/deploy` / customerrepo field YAML  
**Status:** Done (this change)

1. Migration `external_id` + Metadata write/describe/sync (implies unique+indexed; type allowlist)
2. Unique expression projections for `externalId` / `uniqueField`
3. DataEngine `GetByExternalID` / `Upsert`
4. Client routes: GET/PATCH/DELETE by external id + POST upsert + composite UPSERT
5. AuthZ matrix tests

**Exit:** Idempotent Contact sync by `ERP_Id__c` under permission sets.

### Phase 2 — Async Bulk ingest jobs

**Packages:** `migrations/`, `internal/httpapi`, `internal/worker`, `internal/dataengine`  
**Status:** Done (this change)

1. `ingest_jobs` (+ batches/results) schema
2. Job lifecycle routes (`/client/v1/jobs/ingest…`)
3. Worker `ingest.process` for insert/update/upsert/delete
4. Caps + partial success + NDJSON results
5. Integration tests

**Exit:** Multi-thousand-row upsert job completes with per-row results; sync `/bulk/{object}` remains for small inserts.

### Phase 3 — Peer-sourced data packs

**Packages:** `internal/datapack`, `cmd/one`, `internal/customerrepo`, docs  
**Status:** Done (this change)

1. Spec `one-datapack/v1` with `sourceEnv` → `environments/*.yaml`
2. Validate DAG + peer pointer resolution
3. Apply: auth to source + target (Connected App / aliases) → query → reference rewrite → upsert/Bulk
4. Offline `file:` fallback
5. Document in one-repo + multi-env

**Exit:** `datapack apply` pulls Account→Contact from test peer into prod without UUID remapping spreadsheets.

Hardening (CSV, allOrNone, BP-033 ingest lane) can follow as Phase 2+ polish — not a fourth GA gate.

## 8. Explicit non-goals

- Third-party bulk/REST **wire** compatibility / SQL bulk query
- Cross-object upsert in one HTTP call without composite or data-pack steps
- Install→install data **push** or Deploy-promoted business rows
- External ID on kernel `users` (SCIM already)
- Merge / survivorship (BP-046)
- CDC fan-out (BP-042)
- Bypass validation rules or AuthZ for “fast load”
- Multipart binary / Files (BP-045)

---

## 9. Implementation checklist (per PR)

- [ ] Family ownership = Client for runtime; Metadata for `externalId` flag; Deploy only for packing field YAML
- [ ] Unique projection before claiming upsert GA on an object
- [ ] AuthZ parity with single-row CRUD (sharing + FLS)
- [ ] Worker claim remains `FOR UPDATE SKIP LOCKED`
- [ ] Tests in `internal/testutil` for HTTP+DB paths
- [ ] Update [api-families.md](../api-families.md) endpoint tables when routes land
- [ ] Update BP-041 status when phases complete

---

## 10. Suggested sequencing vs neighbors

```text
Phase 1–2 (externalId + REST upsert)
    → unblocks ETL adapters + agent sync reliability
Phase 3 (Bulk jobs)
    → production-scale loads
Phase 5 (data packs)
    → multi-object ordered seed / box refresh
BP-046 merge
    → cure path when upsert prevention is not enough
BP-042 CDC
    → outbound sync complement (not required for ingest)
```
