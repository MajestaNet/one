# Headless Client 360 remainders — remainder tech design + agentic build plan

**Work-order slot:** 11 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-041](../../../backlog/BP-041-record-external-id-upsert-bulk.md) · [BP-042](../../../backlog/BP-042-change-feed-cdc-consumer.md) · [BP-045](../../../backlog/BP-045-files-content-storage.md) · [BP-046](../../../backlog/BP-046-record-merge-dedupe.md) · [BP-061](../../../backlog/BP-061-platform-actions.md)
**Track:** Finish
**Status of remainder:** BP-041 **Mitigated**; 042 / 045 / 046 Open; 061 Partial (`record.merge` remains)
**Domain agents:** `db-backend-perf` · `api-families` · `authz-security` · `worker-jobs`
**Playbooks:** [agent-data-architecture.md](../agent-data-architecture.md) · [agent-api-families.md](../agent-api-families.md) · [platform-actions-build-plan.md](../platform-actions-build-plan.md) · [agent-worker.md](../agent-worker.md) · [agent-authz.md](../agent-authz.md)
**Existing plans (do not duplicate):** [external-id-upsert-bulk-build-plan.md](../external-id-upsert-bulk-build-plan.md) (BP-041 Phases 1–3) · [platform-actions-build-plan.md](../platform-actions-build-plan.md) (BP-061 Phases 1–4 + `quote.accept`)

**Dependencies (do not re-plan):** [BP-043](../../../backlog/BP-043-cross-object-search-api.md) search is mitigated (`POST /client/v1/search`). [BP-044](../../../backlog/BP-044-billing-module-order-from-quote.md) `quote.accept` shipped. [BP-018](../../adr/030-install-agent-runtime.md) Operate/IDE merge UX stays **frozen**.

**Sequence:** 041 remainder before 046 (prevention vs cure). 061 registers `record.merge` on the shipped actions catalog. 042 may follow 041. 045 is independent but AuthZ + admission couple to [BP-003](../../../backlog/BP-003-enterprise-auth.md) / [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md).

---

## 1. Remainder inventory

Honest mark of the current tree. Do not re-plan shipped phases.

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| **BP-041** Metadata `externalId` | Column + write rules + type allowlist + describe | — | `migrations/0043_field_external_id.sql`; `internal/metadata/fieldtypes.go` (`ExternalIDEligibleTypes`); `ApplyExternalIDRules`; `internal/metadata/external_id_test.go` |
| **BP-041** Unique projections | Unique expression indexes for `externalId` **and** `uniqueField` on flexible + HV tables | Leftover non-unique `proj_*` index when a field later becomes unique (`proj_u_*` is a new name) | `internal/dataengine/projections.go` (`CREATE UNIQUE INDEX`, `projectionUniqueIndexName`) |
| **BP-041** DataEngine upsert | `GetByExternalID`, `Upsert` (advisory lock, AuthZ create vs update, FLS, `DUPLICATE_EXTERNAL_ID`) | No DB-gated upsert integration test (unit only: normalize) | `internal/dataengine/upsert.go`; `internal/dataengine/upsert_test.go` |
| **BP-041** Client REST upsert | GET/PATCH/DELETE by `{externalIdField}/{externalId}`; `POST …/upsert`; composite `UPSERT` | No `internal/testutil` HTTP+DB AuthZ matrix | `internal/httpapi/client_upsert.go`; `internal/httpapi/server.go`; `internal/dataengine/service.go` (`case "UPSERT"`) |
| **BP-041** Async ingest jobs | Kernel `ingest_jobs`; `/client/v1/jobs/ingest` lifecycle; worker `ingest.process`; NDJSON; insert/update/upsert/delete; `allOrNone`; owner+admin GET | **No ingest HTTP/worker integration tests.** `IngestChunkSize=500` unused (line-at-a-time). No install-wide concurrent `InProgress` cap (plan: 2). CSV rejected. `processingMode` absent. BP-033 class `ingest` not mapped | `migrations/0044_ingest_jobs.sql`; `internal/dataengine/ingest.go`; `internal/httpapi/client_ingest.go`; `internal/worker/process.go` |
| **BP-041** Sync `POST /bulk/{object}` | Small create-only helper retained | — | `internal/httpapi/server.go` `handleBulk` |
| **BP-041** Data packs | `one-datapack/v1` validate + apply; peer `sourceEnv`; offline `file:`; `one datapack` CLI | Apply is **per-row REST upsert**, not Bulk ingest when large. Manifest tests only (no live apply HTTP test) | `internal/datapack/manifest.go`, `apply.go`, `manifest_test.go`; `cmd/one/datapack.go`; `docs/customer-repo.md` |
| **BP-041** Managed Message.ExternalId | Seed marks `externalId=true` | — | `docs/modules/messages.md`; `internal/seed/module_messages.go` |
| **BP-042** Operator event read | `GET /client/v1/events` (+ unpublished + ack); FLS strip of `data`/`patch`; owner/creator/`viewAll`/admin filter | Not a CDC contract: no `record.created` envelope, no changed-field list, no durable consumer cursor, JOIN `records` only (HV parents miss owner columns), `event_type` is `RecordCreated` | `internal/httpapi/client_extras.go`; `internal/httpapi/events_authz_test.go`; `internal/dataengine/sync_automations.go` `afterWrite` |
| **BP-042** Outbox → webhooks | Claim/lease, `webhook_deliveries` idempotency, payload `{id,type,payload,…}` | Webhook body is not the CDC envelope; no object opt-in; no replay cursor | `internal/worker/process.go` `deliverEvent` / `buildEventBody`; `migrations/0000_kernel.sql` `outbox_events` |
| **BP-042** Retention | `RETENTION_OUTBOX_DAYS` default 30 | CDC replay window not documented as a product contract | `internal/config/config.go`; `internal/worker/retention.go` |
| **BP-043** Search (dependency) | `POST /client/v1/search` shipped | Do not rebuild search. Dupe hints **may** reuse it | `internal/httpapi/client_search.go`; `internal/httpapi/search_integration_test.go` |
| **BP-044** `quote.accept` (dependency) | Registered on `sales`; handler + guest tests | Pattern to copy for `record.merge` | `internal/actions/quote_accept.go`; `internal/seed/module_sales.go`; `internal/httpapi/actions_test.go` |
| **BP-045** Files / blobs | None. ADR-017 explicitly has no `file` type. ADR-011 omits Files. No `CONTENT_*` config | Full content model + Client upload/download + BYO blob | `internal/metadata/fieldtypes.go`; `docs/adr/017-canonical-field-types.md`; `docs/adr/011-sales-service-managed-modules.md` §9 |
| **BP-046** Merge | None | DataEngine merge + reparent + hard-delete loser | — |
| **BP-046** Dupe hints | None (search exists as a find primitive) | Optional thin candidate API | — |
| **BP-061** Catalog / invoke shell | `GET/POST /client/v1/actions/{apiName}`; `ActionDef` on modules; guest `ctx.invokeAction` | — | `internal/actions/`; `internal/httpapi/client_actions.go`; `internal/packages/registry.go` |
| **BP-061** `lead.convert` | Shipped, pack-gated, syncSafe | — | `internal/actions/lead_convert.go`; `internal/seed/module_lead_marketing.go` |
| **BP-061** `quote.accept` | Shipped on BP-044 | — | `internal/actions/quote_accept.go` |
| **BP-061** `record.merge` | Reserved in ADR-029 + `docs/modules/core.md` (“None in v1”) | Register on `core`, handler, guest tests, close Phase 5 | `internal/actions/service.go` (`handlers` has convert + accept only); `internal/seed/packages.go` (`core` has no `Actions`) |
| **IDE Operate merge UX** | Frozen (BP-018) | Out of this work order | — |

---

## 2. Detailed design (remainder only)

### 2.1 Cross-cutting locks

| Topic | Choice |
|---|---|
| Family | **Client** for upsert, ingest, CDC poll, files, merge invoke. Metadata only for `externalId`, `changeFeed`, webhook config, `files` package enable. No fourth family |
| AuthZ floor | Object CRUD + record sharing (ADR-016) + deny-by-default FLS (BP-003 mitigated). No PS `actionAccess` (ADR-029 v1) |
| Storage | Kernel DDL in `migrations/` for new tables. Business fields stay metadata + `records.data`. **No** customer DDL. **No** `tenant_id` |
| Actions | Integrity verbs only via `POST /client/v1/actions/{apiName}`. **Never** `POST /merge` or `POST /convertLead` |
| IDE | Do **not** add Operate merge UX, Files chrome, or CDC panels (BP-018 frozen; ADR-030) |
| API revision | Additive Client routes. Bump `apiRevision` only if existing `GET /events` or upsert behavior changes for pinned clients ([ADR-025](../../adr/025-api-revision-versioning.md)) |
| Search / quote.accept | Consume as shipped. Do not reopen BP-043 / BP-044 |

---

### 2.2 BP-041 remainder (do not rewrite the existing plan)

[external-id-upsert-bulk-build-plan.md](../external-id-upsert-bulk-build-plan.md) Phases 1–3 are **in tree**. This remainder is production-hardening so BP-041 can move from Partially mitigated → Mitigated.

#### Still required to close 041

1. **HTTP+DB contract tests** (`internal/testutil`):
   - Metadata: `externalId=true` on `text`/`email`/`integer`; reject `boolean`/`textarea`/`lookup`.
   - Unique projection `ready` for that field; dirty duplicate rows → projection `failed`.
   - REST: create via `PATCH …/{externalIdField}/{externalId}` → 201 `created:true`; repeat → 200 `created:false`; GET/DELETE by key; FLS deny on unreadables; create-only PS cannot update existing; update-only PS cannot create; sharing deny on existing.
   - `409` / `DUPLICATE_EXTERNAL_ID` when two rows share the key (legacy dirty data).
   - Ingest: Open → PUT NDJSON batches → PATCH `UploadComplete` → worker `ingest.process` → `JobComplete` with successfulResults/failedResults; abort Open job; non-owner GET → 403; admin GET → 200.
   - Composite `UPSERT` sequential + `@{ref.Id}`.

2. **Ingest caps that the plan locked but code skipped:**
   - Enforce **max 2** `InProgress` ingest jobs per install (`COUNT(*) FROM ingest_jobs WHERE state='InProgress'` before claiming / transitioning). Extra jobs stay `UploadComplete` until a slot frees (worker re-enqueues or next poll).
   - Honor `IngestChunkSize` (500): wrap each chunk in its own tx when `allOrNone=false` so a 100k job does not hold one connection for the whole payload. `allOrNone=true` may keep a single tx **or** abort remaining chunks and mark `Failed` (plan §4.4). Do not silently change webhook/automation fan-out (still per successful row).

3. **Data-pack apply at volume:** when a step has `>500` rows, apply via target `/client/v1/jobs/ingest` (upsert) instead of per-row `POST …/upsert`. Keep REST for small/offline steps. Existing manifest schema is unchanged.

#### Explicit 041 follow-ons (do **not** block Mitigated)

| Item | Why deferred |
|---|---|
| CSV ingest (`text/csv`, delimiters) | Plan Phase 4b; code already rejects non-NDJSON |
| `processingMode: Serial\|Parallel` | Pack-level object order already exists; within-object Parallel is default |
| BP-033 job class `ingest` | Couple when BP-033 admission lands; until then hard caps above |
| Drop leftover non-unique `proj_*` when unique index is built | Operator can rebuild projections; not ingest-blocking |
| Kernel `users.external_id` | Non-goal (SCIM) |

Do not add Bulk query, sibling merge, CDC, or multipart files under 041.

---

### 2.3 BP-042 — CDC consumer contract (first detailed design)

There is **no** dedicated CDC plan. `GET /events` is an operator/outbox dump. Integrators need a versioned change envelope, a replay cursor, FLS, and an opt-in — **not** a Kafka/Pub/Sub clone (ADR-001 dedicated install; BP-042 non-goals).

#### Thesis

> CDC v1 is a **poll + cursor** Client read over an **extended outbox**. The worker still delivers webhooks from the same rows. The envelope is product-owned JSON. Consumers checkpoint a cursor; they do not ack individual outbox rows (that remains webhook/`PATCH …/ack` operator machinery).

```text
DataEngine afterWrite
  → INSERT outbox_events (existing + seq + actor + changed_fields + owner/creator snapshot)
        │
        ├─ worker ProcessOutbox → webhooks (compat: type RecordCreated|…)
        └─ Client GET /client/v1/changes?cursor=  → CDC envelope (record.created|…)
              optional POST /client/v1/changes/cursors  → durable named cursor
```

#### Keep vs add

| Keep | Add |
|---|---|
| `outbox_events` as the SoR | `seq BIGINT GENERATED ALWAYS AS IDENTITY` (monotonic install-wide) |
| Webhook delivery + `webhook_deliveries` | Snapshot `actor_id`, `owner_id`, `created_by_id` on the outbox **row** (stop JOIN `records` for AuthZ — HV-safe) |
| `GET /client/v1/events` as operator dump | `changed_fields text[]` (update only; apiNames) |
| `event_type` `RecordCreated` / `RecordUpdated` / `RecordDeleted` for webhook matching | CDC wire names `record.created` \| `record.updated` \| `record.deleted` **mapped in the poll API** (do not rename webhook types in v1) |
| `RETENTION_OUTBOX_DAYS` (default 30) | Document as the **replay window**. Purged seqs → `410 CURSOR_EXPIRED` |
| Metadata webhooks CRUD | Object opt-in `metadata_objects.change_feed boolean NOT NULL DEFAULT true` for `flexible`; **default false** for `high_volume` (Message volume). HV objects opt in explicitly |

Do **not** introduce a second events table or an external bus. Do **not** require SSE or long-poll in v1 (plain poll). Do **not** publish customer-defined event types.

#### Change envelope (Client poll)

```json
{
  "seq": "1042",
  "id": "<outbox uuid>",
  "type": "record.updated",
  "objectApiName": "Account",
  "recordId": "<uuid>",
  "actorId": "<uuid>",
  "createdAt": "2026-08-23T03:00:00.000Z",
  "changedFields": ["Name", "Phone"],
  "record": { "Id": "…", "Name": "Acme" }
}
```

| Field | Rules |
|---|---|
| `seq` | Opaque decimal string of `outbox_events.seq`. Cursor is this value. |
| `type` | Mapped from `RecordCreated` → `record.created`, etc. Unknown outbox types (`install.claimed`, `principal.*`) are **omitted** from `/changes` (different audience — BP-038) |
| `changedFields` | Keys of the write `patch` after FLS strip; omit on create/delete. Never invent a full before/after image in v1 |
| `record` | Current field map after FLS strip. On **delete**: `{ "Id": "…" }` only (row is gone). Never leak unreadable fields (reuse `StripUnreadableFields`) |
| Deletes | Outbox insert already happens in `afterWrite` before the row disappears; snapshot Id + object + actor on the event row |

Webhook POSTs stay on `buildEventBody` (`type: RecordUpdated`). Optional later: webhooks may subscribe to alias `record.updated` that matches the same rows — **not** required to close 042.

#### Client API

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/client/v1/changes` | Poll. Query: `cursor` (exclusive seq, empty = from retention head), `objects` (comma apiNames), `limit` (default 100, max 500) |
| `POST` | `/client/v1/changes/cursors` | `{ "name": "warehouse", "cursor": "1042" }` upsert **per actor**. Name `^[a-z0-9_]{1,64}$` |
| `GET` | `/client/v1/changes/cursors/{name}` | Return stored cursor for this actor |

Response: `{ "changes": […], "nextCursor": "1042", "lag": n }` where `lag` is count of CDC-eligible rows with `seq > nextCursor` (capped, not a full table scan — `COUNT` with the same predicates + `seq > $cursor` and a statement timeout / `LIMIT` estimate). Empty page + `nextCursor` = last seen seq (idempotent poll).

Scope: `client`. No `/v1` alias for `/changes` (new surface).

**AuthZ:** skip events whose object is not change-feed enabled; skip objects the actor cannot `read`; skip records the actor cannot view (use snapshotted owner/creator + `viewAll` + sharing evaluator — **no** JOIN to `records` required for the skip decision). FLS strip `record` + `changedFields` (drop unreadable names from the list). Admin sees all opted-in objects.

**Ordering:** `seq` ascending. Per-object order is the outbox insert order (same tx as the write). No global linearizability across replicas beyond Postgres commit order. At-least-once: consumers must upsert by `recordId` (pairs BP-041).

#### Durable cursor table

```sql
CREATE TABLE change_cursors (
  actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  cursor_seq bigint NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (actor_id, name)
);
```

Cursors are **not** Deploy-promoted. Losing a cursor means replay from `now() - RETENTION_OUTBOX_DAYS` (or `cursor=0` within retention).

#### Metadata

- `changeFeed: true` on object describe / Metadata GET/PATCH (customer objects + managed sync). Managed seed: Account/Contact default **on**; Message default **off**.
- Clearing `changeFeed` stops **new** CDC rows from appearing in `/changes`; existing outbox rows remain until retention. Webhooks unchanged.
- Do not add a Metadata “CDC subscription” object in v1 — the Client cursor **is** the subscription.

#### Failure modes

| HTTP | Code | When |
|---|---|---|
| 400 | `VALIDATION_FAILED` | Bad cursor / limit / objects |
| 403 | `FORBIDDEN` | No `client` scope (middleware) |
| 410 | `CURSOR_EXPIRED` | Cursor seq older than retained min(seq) |
| 200 | — | Empty `changes` is success (caught up) |

Do not 404 unknown objects in `objects=` — skip them (describe-or-ignore) so consumers survive pack disable.

#### Contrast (docs to add when implementing)

| Surface | Audience | Guarantee |
|---|---|---|
| Outbox + webhooks | Install automation / BYO alerts | At-least-once POST; ack = `published_at` |
| `GET /events` | Operators | Recent dump, FLS redaction, ack helper |
| `GET /changes` | Integrators / warehouses / indexers | Versioned envelope, seq cursor, object opt-in, FLS |

Search indexers (BP-043) **already** maintain `search_document` in-process. CDC is **not** required to close search. It is an outbound complement to 041 ingest.

---

### 2.4 BP-045 — Files / content (ADR-shaped design)

ADR-011 omits Files. ADR-017 has no `file` type. ADR-013 defers “large bodies → object storage.” This section is the product decision record implementers follow; a short ADR-032 may be extracted later **without** changing these locks.

#### Decision 1 — Nouns

| Noun | Author | Lives where |
|---|---|---|
| **Content** | Product kernel | Table `content_files` (metadata + blob pointer). Describe as managed object `Content` with `storage_mode=kernel` (same pattern as User: not a `records` row) |
| **Blob bytes** | Customer’s bucket or local volume | Never `records.data`, never product CDN |
| **Parent** | Existing flexible/HV/kernel record | Polymorphic `ParentType` + `ParentId` (copy Message’s pattern) |

Do **not** add canonical field type `file` in v1 (ADR-017 stays). Integrators link files by parent, not by a field value. A future `file` type would store a Content Id in JSONB — follow-on ADR.

Do **not** ship Knowledge, DAM, versioning, legal hold, or email MIME (BP-045 / BP-038 non-goals).

#### Decision 2 — Optional managed package `files`

- `packages.Register` module `files`, `depends_on: [core]`, optional, admin enable.
- Enabling syncs managed object `Content` (describe-only Client object; writes go through `/files` not `/sobjects/Content`).
- Client file routes return `409 PACKAGE_NOT_ENABLED` `{ packageName: "files" }` when the pack is off (same code as actions — do not 404 a known surface).
- Not always-on `core` (ADR-011 omit stands until enable).

`Content` fields (describe): `Id`, `ParentType`, `ParentId`, `FileName`, `ContentType`, `ByteSize`, `ChecksumSha256`, `CreatedById`, `CreatedAt`. No `Body` in JSON.

#### Decision 3 — Kernel table

```sql
CREATE TABLE content_files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_object_api_name text NOT NULL,
  parent_record_id uuid NOT NULL,
  file_name text NOT NULL,
  content_type text NOT NULL DEFAULT 'application/octet-stream',
  byte_size bigint NOT NULL,
  checksum_sha256 text NOT NULL,
  storage_backend text NOT NULL,  -- local | s3
  storage_key text NOT NULL,
  created_by_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX content_files_parent_idx
  ON content_files (parent_object_api_name, parent_record_id, created_at DESC);
```

Hard-delete only (product policy). Deleting a parent record in v1 **does not** cascade-delete files automatically (orphan GC job later). Merge (BP-046) **must** reparent `content_files.parent_record_id` when the parent object is merged.

#### Decision 4 — Blob backend (BYO, no product CDN)

| Backend | When | Config |
|---|---|---|
| `local` | Compose / tests | `CONTENT_BACKEND=local`, `CONTENT_LOCAL_DIR` (install volume). Key = `{INSTALL_ID}/{id}` |
| `s3` | Path A / Spaces / any S3-compatible | `CONTENT_BACKEND=s3`, `CONTENT_S3_ENDPOINT`, `CONTENT_S3_BUCKET`, `CONTENT_S3_REGION`, `CONTENT_S3_ACCESS_KEY`, `CONTENT_S3_SECRET_KEY`. Key prefix `{INSTALL_ID}/content/{id}` |

- Dedicated install prefix is **`INSTALL_ID`**, never a Majesta-hosted multi-tenant bucket.
- **No** public ACL, **no** product CDN, **no** DigitalOcean Spaces as a Majesta SaaS fleet.
- v1 upload/download **streams through the API** so AuthZ stays in-process. Presigned PUT/GET is a follow-on (needs clock-skew + capability `content.presign` — do not add now).
- Virus scan hook: deferred (optional worker type later).

Use AWS SDK v2 in product Go **only** as the S3-compatible client (endpoint override for Spaces). This is blob I/O, not a second Path A. Document in `docs/tech-stack.md` in the same PR.

#### Decision 5 — Client API

| Method | Path | Behavior |
|---|---|---|
| `POST` | `/client/v1/files` | `multipart/form-data`: fields `parentType`, `parentId`, file part `file`. 201 Content metadata |
| `GET` | `/client/v1/files` | Query `parentType`, `parentId` required. List metadata, keyset on `(created_at, id)`, default 50 / max 200 |
| `GET` | `/client/v1/files/{id}` | Metadata |
| `GET` | `/client/v1/files/{id}/content` | Stream bytes; `Content-Type` / `Content-Disposition` from row; `X-Content-Checksum-SHA256` |
| `DELETE` | `/client/v1/files/{id}` | Hard-delete row + blob |

Scope: `client`. No `/sobjects/Content` CRUD. No flat `/v1/files` alias.

Default `REQUEST_BODY_LIMIT` is 1 MiB — **too small**. File POST uses a dedicated limit (`CONTENT_MAX_BYTES`, default **25 MiB**) via a route-specific `MaxBytesReader`, not by raising the global limit. Align with BP-033 admission when that lands (same env knob).

Other v1 ceilings: 100 files per parent; 5 concurrent uploads per actor (in-flight counter / rate limiter). Reject `byte_size=0`. Sanitize `FileName` (no path segments). Allowlist Content-Type as stored header only — do not execute.

#### Decision 6 — AuthZ (parent is the ACL)

There is no independent Content CRUD grant in v1.

| Verb | Required |
|---|---|
| Upload | Parent object `update` + `CanModifyRecord` on parent + parent exists |
| List / metadata GET / download | Parent object `read` + `CanViewRecord` on parent |
| Delete | Parent object `update` + `CanModifyRecord` on parent |

If the parent row is gone, metadata GET → 404 (do not leak names). Sharing (ADR-016) applies to the **parent**, not a new Content OWD. Admin bypass matches other Client writes. FLS does not apply to bytes; `FileName` is not a flexible field.

Do not store files on kernel `users` in v1 (identity photos stay out of scope).

#### Failure modes

| HTTP | Code | When |
|---|---|---|
| 409 | `PACKAGE_NOT_ENABLED` | `files` pack off |
| 413 | `PAYLOAD_TOO_LARGE` | Over `CONTENT_MAX_BYTES` |
| 403 | `FORBIDDEN` | Parent AuthZ fail |
| 404 | `NOT_FOUND` | Unknown id or parent |
| 503 | `STORAGE_UNAVAILABLE` | Backend misconfigured / S3 error after retries |

---

### 2.5 BP-046 + BP-061 — `record.merge` + optional dupe hints

Merge is the **cure** after 041 upsert prevention. It **must** be `POST /client/v1/actions/record.merge` ([ADR-029](../../adr/029-platform-actions.md)). Do not add `/client/v1/merge`.

`lead.convert` and `quote.accept` already prove the catalog. Remainder of BP-061 is this verb + close Phase 5.

#### Registry (BP-061)

On **`core`** (always enabled):

```go
packages.ActionDef{
  APIName:          "record.merge",
  Label:            "Merge Records",
  RequiresPackages: []string{"core"},
  Objects:          []string{"Account", "Contact"}, // describe; runtime = any flexible object
  SyncSafe:         true,
  // input/output JSON Schema below
}
```

Bump `CorePackageVersion`. Document under `docs/modules/core.md` (replace “None in v1”). Register handler in `internal/actions/service.go`. Guest `invokeAction` works with no SDK change.

`GET /client/v1/actions` includes `record.merge` whenever `core` is installed (always). Describe returns schemas.

#### Input / output

| Input | Rules |
|---|---|
| `objectApiName` | Required. Same type for master and duplicates. Kernel objects (`User`) **rejected**. HV objects **rejected** in v1 (Message merge is not a 360 party operation) |
| `masterId` | Required. Surviving record |
| `duplicateIds` | Required array, 1–5 UUIDs, no overlap with master, all same object |
| `keepFromDuplicate` | Optional map `fieldApiName → duplicateId`. Copy those field values from that loser onto master **before** reparent. Default: master wins every field |

Output: `{ objectApiName, masterId, mergedIds, reparentedLookups, alreadyMerged }`.

Idempotency: if a duplicate Id is already gone and master exists, skip that id (count in `mergedIds` only when a row was deleted this call). If **all** duplicates are already absent, return `alreadyMerged: true` without error.

#### Algorithm (one Postgres tx, syncSafe)

1. Load master + each duplicate via DataEngine `Get`. AuthZ: object `read`+`update`+`delete`; `CanViewRecord` + `CanModifyRecord` on **every** involved record (sharing both sides). FLS: must be able to edit every key in `keepFromDuplicate`.
2. Reject mixed objects, self-merge, >5 losers, kernel/HV, missing rows (`VALIDATION_FAILED` / `NOT_FOUND`).
3. External ID: if a loser has `externalId` fields set that the master lacks, copy them (unique index). If both set and **differ**, `409 CONFLICT` / `EXTERNAL_ID_CONFLICT` unless `keepFromDuplicate` names that field (then overwrite master; unique violation → 409).
4. Apply `keepFromDuplicate` patches onto master (`Update`).
5. **Reparent** every `lookup` / `master_detail` / `polymorphic_lookup` whose `reference_to` (or polymorphic parent type) is this object:
   - Query `metadata_fields` (+ `metadata_relationships`) for inbound fields.
   - `UPDATE` `records` and `records_hv`: `jsonb_set` the lookup JSON key from loser Id → master Id where `object_api_name` matches the child object. Bound by indexed lookup projections (those fields are `indexed=true` on managed defs). Cap rows per field per statement; loop keyset if needed.
   - Polymorphic: `ParentId` rewrite only when `ParentType` equals `objectApiName`.
   - Self-lookup (`Account.ParentAccountId`): if master points at a loser, set null (avoid cycle); if children point at loser, point at master.
   - `content_files.parent_record_id` when BP-045 table exists (IF table — merge must not fail if files not migrated yet).
   - `record_access_grants`: rewrite `record_id` loser → master (PK includes object); on conflict keep master’s grant.
6. Hard-delete each loser (`Delete` — fires `afterWrite` delete outbox + audit). Do **not** recycle.
7. Audit details: `{ masterId, mergedIds, reparentedLookups }`. Action name `record.merge` on `audit_log`. Outbox: existing per-row create/update/delete events from step 5–6; do **not** require a custom `record.merged` type in v1 (CDC consumers see child updates + loser deletes). Optional later envelope is BP-042 follow-on.

`master_detail` children of the **loser** are reparented, not cascade-deleted (cascade would destroy the 360 graph).

Custom `__c` fields: only copied when named in `keepFromDuplicate`. Same rule as convert (wrapping automation copies the rest).

#### Optional dupe hints (046 phase B — may ship after merge)

Thin, not MDM. **Not** a sibling of merge.

`POST /client/v1/duplicates/find` `{ objectApiName, fieldApiNames, limit }` (default fields: Contact `Email`, `Phone`; Account `Name` + `AccountNumber` when indexed).

Implementation: SQL `GROUP BY` of `data->>field` on the object partition where value is non-empty and `COUNT(*) > 1`, plus AuthZ visibility in SQL (same as query). Return groups of `{ value, recordIds, fieldApiName }` max 50 groups, 10 ids each.

Do **not** auto-merge. Do **not** clone incumbent matching rules. Callers may also use shipped `POST /search` for interactive find. Skip hints if merge GA needs to land first.

#### Error codes (actions catalog)

Reuse ADR-029 table. Additional: `409 EXTERNAL_ID_CONFLICT`; `400 VALIDATION_FAILED` for mixed types / cap; `403 FORBIDDEN` if any record fails sharing/FLS.

---

## 3. Concrete agentic build plan

Execute in this order. 042 and 045 may parallel after 041 tests exist if agents do not contend on `afterWrite` / migrations numbering — **serialize migrations**.

### Phase A — BP-041 close-out (tests + ingest/datapack remainder)

- **Owner:** `db-backend-perf` + `api-families` (HTTP tests) + `worker-jobs` (ingest chunk/cap)
- **Packages allowed:** `internal/dataengine`, `internal/httpapi`, `internal/datapack`, `internal/worker`, `internal/testutil`, `internal/metadata` (tests only unless projection leftover cleanup), `cmd/one` (datapack Bulk apply). **Forbidden:** `tools/control-ide/**`, new Client families, merge/CDC/files product code
- **Files likely:** `internal/dataengine/ingest.go` (chunk tx, InProgress cap); `internal/datapack/apply.go`; new `internal/httpapi/upsert_ingest_integration_test.go`; `docs/architecture/external-id-upsert-bulk-build-plan.md` status notes only if a remainder checkbox moves
- **Tests:** `go test -p 1 ./internal/dataengine/... ./internal/httpapi/... ./internal/datapack/... ./internal/worker/...` with `DATABASE_URL`
- **Exit:** Idempotent Contact upsert by custom external id under a non-admin PS; 2k-row ingest job `JobComplete` with per-row results; datapack step of >500 rows uses ingest jobs; BP-041 status **Mitigated** (CSV/processingMode/BP-033 ingest class listed as follow-ons in the BP, not open scope)
- **Depends:** BP-003 (AuthZ), BP-005 (claim). Not blocked on 033/042/046

### Phase B — BP-042 CDC poll + outbox extension

- **Owner:** `db-backend-perf` + `api-families` + `worker-jobs` + `authz-security` (FLS/sharing on envelope)
- **Packages allowed:** `migrations/`, `internal/dataengine` (`afterWrite`), `internal/httpapi` (new `client_changes.go`), `internal/db`, `internal/metadata` (`changeFeed`), `internal/worker` (retention docs only; webhook body unchanged), `internal/seed` (Account/Contact vs Message defaults), `internal/testutil`. **Forbidden:** IDE, new bus product, renaming webhook `event_type` values
- **Files likely:** new `migrations/00xx_change_feed.sql`; `internal/dataengine/sync_automations.go`; `internal/httpapi/client_changes.go`; `internal/httpapi/changes_integration_test.go`; `docs/api-families.md` Events vs Changes row; `docs/modules/core.md` / `messages.md` `changeFeed`
- **Tests:** create/update/delete Account → poll envelope + FLS omit; HV Message omitted until opt-in; cursor persist; `410` after retention purge (or simulated min seq); webhook still receives `RecordCreated`
- **Exit:** Warehouse-style client can page `GET /changes` and upsert by `recordId` without `GET /events`. BP-042 **Mitigated**
- **Depends:** 041 remainder preferred (consumers upsert). Search (043) already shipped. Not blocked on 045/046

### Phase C — BP-045 files module + BYO blob

- **Owner:** `db-backend-perf` + `api-families` + `authz-security` + `deploy-ops` (env on Compose/Helm/App Spec — **document knobs**, no IDE Govern UI)
- **Packages allowed:** `migrations/`, `internal/content` (new), `internal/httpapi`, `internal/config`, `internal/seed`, `internal/packages`, `internal/authz` (parent checks only), `deploy/` env examples, `docs/tech-stack.md`, `docs/modules/files.md`, `docs/adr/017` note (file type still deferred). **Forbidden:** product CDN, Knowledge, presigned v1, `file` field type, Control IDE chrome, raising global `REQUEST_BODY_LIMIT` as the only fix
- **Files likely:** `internal/content/store.go`, `s3.go`, `local.go`; `internal/httpapi/client_files.go`; `migrations/00xx_content_files.sql`; `internal/seed/module_files.go`; `deploy/docker-compose.yml` volume + env
- **Tests:** pack off → 409; enable → upload to Case-or-Account parent as non-admin with grants; foreign parent → 403; download bytes match sha256; delete removes local blob; oversize 413. Skip live S3 when env unset; fake backend in unit tests
- **Exit:** Case evidence / Account file round-trip on local backend. BP-045 **Mitigated** (presign/virus/GC follow-ons listed)
- **Depends:** BP-003. Admission knobs should match BP-033 when present (`CONTENT_MAX_BYTES`). Independent of 041/042/046 except merge reparent hook (feature-detect table)

### Phase D — BP-046 DataEngine merge + optional hints

- **Owner:** `db-backend-perf` (+ `authz-security`)
- **Packages allowed:** `internal/dataengine` (`merge.go`), `internal/metadata` (inbound field listing helper), `internal/testutil`. HTTP **only** if Phase E is same PR — still no `/merge` route. **Forbidden:** IDE merge UX, Person Accounts, auto-merge, cross-object party merge
- **Files likely:** `internal/dataengine/merge.go`, `merge_test.go`; optional `internal/httpapi/client_duplicates.go` for hints
- **Tests:** Account merge reparents Contact.AccountId + Opportunity.AccountId (enable `sales`); loser hard-deleted; sharing deny; external-id conflict; self-parent cycle null; HV Message rejected; `keepFromDuplicate` copies Email
- **Exit:** `Service.Merge` is the single implementation used by the action handler. Hints optional
- **Depends:** Phase A (041) preferred. Phase E may land in the same PR. Phase C table reparent if files exist

### Phase E — BP-061 register `record.merge` and close Phase 5

- **Owner:** `api-families` + `db-backend-perf` (+ `worker-jobs` only if extra guest tests)
- **Packages allowed:** `internal/actions`, `internal/packages` (via seed), `internal/seed/packages.go`, `internal/httpapi/actions_test.go`, `internal/dataengine/invoke_action_*_test.go`, `docs/modules/core.md`, ADR-029 reserved table note if needed. **Forbidden:** per-verb route, `ctx.mergeRecords`, PS `actionAccess`
- **Files likely:** `internal/actions/record_merge.go`; `internal/actions/service.go` handler map; `internal/seed/packages.go` `Actions`; `docs/modules/core.md`
- **Tests:** catalog lists `record.merge`; POST happy path; POST unknown still 404; guest `invokeAction` syncSafe in tx; FLS/sharing deny rolls back
- **Exit:** BP-061 Phase 5 complete (`quote.accept` + `record.merge`). BP-061 **Mitigated** (Phase 6 `actionAccess` remains “not started”). BP-046 **Mitigated** when merge + tests land (hints may stay “optional follow-on” in the BP)
- **Depends:** Phase D implementation (same PR OK)

---

## 4. Explicit non-goals

- Operate / Control IDE merge, Files, or CDC UX ([BP-018](../../adr/030-install-agent-runtime.md) frozen)
- Sibling Client routes: `/merge`, `/convertLead`, `/acceptQuote`, `/sobjects/Content` CRUD
- Rewriting [external-id-upsert-bulk-build-plan.md](../external-id-upsert-bulk-build-plan.md) or re-implementing shipped 041 Phases 1–3
- Reopening mitigated [BP-043](../../../backlog/BP-043-cross-object-search-api.md) / [BP-044](../../../backlog/BP-044-billing-module-order-from-quote.md)
- Third-party Bulk/CDC/Files wire clones; Kafka/Kinesis/Pub/Sub as product
- Multi-tenant shared streaming bus or blob fleet (ADR-001)
- SSE/long-poll CDC requirement; sub-second SLA
- Customer-defined event types; PS `actionAccess`
- Canonical `file` field type, Knowledge, DAM, versioning, legal hold, email MIME, virus scan, presigned URLs (v1)
- Product CDN; raising global body limit instead of a files route limit
- Person Accounts / Household graphs; auto-merge; cross-object Account+Contact merge
- Bulk query / SQL export; CSV ingest as a 041 close gate
- External ID on kernel `users` (SCIM)
- Install→install data push via Deploy
- Composite `action` subrequests and MCP `invoke_action` (follow-ons on BP-061 plan Phase 5)

---

## 5. Agentic implementation prompt(s)

Five paste-ready prompts, one per BP, in Finish order.

### Prompt 1 — BP-041 remainder

```text
You are the Majesta One db-backend-perf agent (api-families for Client HTTP tests; worker-jobs for ingest chunking).

Read first:
- docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md §1, §2.2, Phase A
- docs/architecture/external-id-upsert-bulk-build-plan.md (DO NOT rewrite; remainder only)
- docs/architecture/agent-data-architecture.md
- docs/architecture/agent-api-families.md
- backlog/BP-041-record-external-id-upsert-bulk.md

Mission: close BP-041 remainder so status can move to Mitigated. Phases 1–3 already exist (externalId metadata, unique projections, GetByExternalID/Upsert, REST upsert + composite UPSERT, ingest_jobs + /client/v1/jobs/ingest, datapack validate/apply).

Do:
1. Add internal/testutil HTTP+DB tests: externalId write rules; REST upsert AuthZ/FLS/sharing matrix; DUPLICATE_EXTERNAL_ID; ingest job lifecycle including worker ingest.process; composite UPSERT.
2. Enforce max 2 InProgress ingest jobs per install; process NDJSON in 500-row transactions when allOrNone=false (IngestChunkSize is defined but unused).
3. datapack apply: steps with >500 rows use target Client ingest jobs (upsert), not per-row REST.
4. Update BP-041 status to Mitigated; list CSV, processingMode, BP-033 ingest class as follow-ons.

Packages allowed: internal/dataengine, internal/httpapi, internal/datapack, internal/worker, internal/testutil, cmd/one (datapack only), backlog/BP-041, docs only if api-families.md ingest notes need the Bulk apply behavior.
Packages forbidden: tools/control-ide/**, migrations unless a tiny ingest index is required, merge/CDC/files product code.

Tests: go test -p 1 ./internal/dataengine/... ./internal/httpapi/... ./internal/datapack/... ./internal/worker/...
Exit: non-admin Contact upsert by external id; ingest JobComplete with per-row results; large datapack step uses ingest.

Non-goals: CSV ingest, Bulk query, CDC, files, merge, IDE, rewriting the existing 041 plan, users.external_id.
```

### Prompt 2 — BP-042 CDC consumer contract

```text
You are the Majesta One db-backend-perf agent (api-families for Client /changes; worker-jobs for outbox compatibility; authz-security for FLS/sharing).

Read first:
- docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md §2.3, Phase B
- backlog/BP-042-change-feed-cdc-consumer.md
- docs/architecture/agent-api-families.md
- docs/architecture/agent-worker.md
- docs/architecture/agent-authz.md
- docs/adr/009-record-audit-authz-packaging.md
- docs/adr/016-record-sharing.md
- internal/dataengine/sync_automations.go afterWrite
- internal/httpapi/client_extras.go handleListEvents (operator dump — keep)

Mission: first CDC product surface. Poll+cursor v1 over extended outbox. Not a third-party bus clone. Do not rename webhook event_type values (keep RecordCreated|RecordUpdated|RecordDeleted).

Do:
1. Migration: outbox_events.seq identity; actor_id/owner_id/created_by_id snapshots; changed_fields text[]; metadata_objects.change_feed (flexible default true, high_volume default false); change_cursors table.
2. afterWrite populates snapshots + changed_fields from patch keys. HV Message omitted from /changes until changeFeed=true.
3. Client GET /client/v1/changes (cursor, objects, limit) returns envelope type record.created|updated|deleted, FLS-stripped record, seq cursor. POST/GET /client/v1/changes/cursors/{name} per actor. 410 CURSOR_EXPIRED. No /v1 alias. No SSE.
4. GET /events and webhook buildEventBody stay compatible. Skip install.claimed / principal.* on /changes.
5. Tests: Account CRUD poll; FLS; sharing skip; Message default off; cursor; webhook still RecordCreated.
6. Document in docs/api-families.md (Changes vs Events). BP-042 → Mitigated.

Packages allowed: migrations/, internal/dataengine, internal/httpapi, internal/db, internal/metadata, internal/seed, internal/worker (compat only), internal/testutil, docs/api-families.md, docs/modules/core.md, docs/modules/messages.md, backlog/BP-042.
Packages forbidden: tools/control-ide/**, Kafka/SQS, GET /events rewrite, files/merge.

Tests: go test -p 1 ./internal/httpapi/... ./internal/dataengine/... ./internal/worker/...
Non-goals: SSE, sub-second SLA, customer event types, search reindex via CDC, bus products, IDE.
```

### Prompt 3 — BP-045 files / content storage

```text
You are the Majesta One db-backend-perf agent (api-families for Client /files; authz-security for parent ACL; deploy-ops for env knobs only).

Read first:
- docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md §2.4, Phase C
- backlog/BP-045-files-content-storage.md
- docs/adr/017-canonical-field-types.md (do NOT add file type)
- docs/adr/013-high-volume-flexible-storage.md
- docs/adr/011-sales-service-managed-modules.md §9 omit Files
- docs/architecture/agent-api-families.md
- docs/architecture/agent-authz.md
- docs/tech-stack.md
- internal/seed/module_messages.go (polymorphic ParentId/ParentType pattern)

Mission: ADR-shaped files v1. BYO blob (local volume or S3-compatible / DO Spaces). No product CDN. AuthZ via parent record. Optional managed package files.

Do:
1. Kernel content_files + optional package files with kernel describe object Content. Enable gate 409 PACKAGE_NOT_ENABLED.
2. CONTENT_BACKEND=local|s3; prefix INSTALL_ID. Stream upload/download through API. Dedicated CONTENT_MAX_BYTES default 25MiB (do not raise global REQUEST_BODY_LIMIT as the only control).
3. Client: POST/GET/DELETE /client/v1/files and GET …/content. multipart field file + parentType/parentId. List by parent. No /sobjects/Content CRUD. No /v1 alias.
4. AuthZ: parent update+CanModifyRecord for upload/delete; parent read+CanViewRecord for list/download.
5. docs/modules/files.md, tech-stack blob row, ADR-017 note that file type remains deferred. Compose env + volume. BP-045 → Mitigated with presign/virus/GC as follow-ons.

Packages allowed: migrations/, internal/content (new), internal/httpapi, internal/config, internal/seed, internal/packages, internal/authz (parent checks), deploy/ env examples, docs/modules/files.md, docs/tech-stack.md, docs/adr/017, backlog/BP-045.
Packages forbidden: tools/control-ide/**, Knowledge, DAM, email MIME, canonical file field type, product CDN, presigned URLs, virus scanner.

Tests: go test -p 1 ./internal/content/... ./internal/httpapi/... ./internal/seed/...
Non-goals: IDE Files UX, public blob URLs, multi-tenant CDN, file field type, merge UX.
```

### Prompt 4 — BP-046 record merge + optional dupe hints

```text
You are the Majesta One db-backend-perf agent (authz-security for sharing/FLS on both records).

Read first:
- docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md §2.5, Phase D
- backlog/BP-046-record-merge-dedupe.md
- docs/adr/029-platform-actions.md (record.merge reserved on core)
- docs/architecture/platform-actions-build-plan.md Phase 5
- docs/architecture/agent-data-architecture.md
- docs/data-model.md
- internal/actions/lead_convert.go and quote_accept.go (AuthZ + tx patterns)
- internal/dataengine/upsert.go (do not merge on DUPLICATE_EXTERNAL_ID — that stays 409)

Mission: implement DataEngine.Merge (same-object, 1 master + 1–5 losers, reparent lookups/MD/polymorphic, hard-delete losers). HTTP invoke is POST /client/v1/actions/record.merge ONLY — no sibling /merge route. Prefer landing the action handler in the same change set as Prompt 5 if you own both; otherwise expose Service.Merge for the actions package.

Do:
1. internal/dataengine/merge.go: AuthZ update+delete+sharing both sides; keepFromDuplicate; EXTERNAL_ID_CONFLICT; reparent inbound fields on records + records_hv; self-lookup cycle null; record_access_grants rewrite; content_files reparent if table exists; hard-delete losers; audit details.
2. Reject User/kernel and high_volume in v1. Cap duplicateIds at 5. Idempotent missing losers.
3. Optional: POST /client/v1/duplicates/find thin GROUP BY on Email/Phone/Name — no auto-merge. May defer if merge tests are the close gate.
4. Do not add Operate/IDE merge UX (BP-018 frozen).

Packages allowed: internal/dataengine, internal/metadata, internal/actions (handler OK), internal/httpapi (duplicates/find only; still no /merge), internal/testutil, internal/seed (tests enabling sales), backlog/BP-046.
Packages forbidden: tools/control-ide/**, POST /merge, Person Accounts, auto-merge, MDM rule engine.

Tests: go test -p 1 ./internal/dataengine/... ./internal/actions/... ./internal/httpapi/...
Non-goals: IDE, cross-object Account+Contact merge, recycle bin, custom field auto-copy.
```

### Prompt 5 — BP-061 register `record.merge` (remaining verb)

```text
You are the Majesta One api-families agent (db-backend-perf for handler/DataEngine; worker-jobs only if extra guest invokeAction tests).

Read first:
- docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md §2.5, Phase E
- docs/architecture/platform-actions-build-plan.md (Phases 1–4 Done; Phase 5 record.merge pending)
- docs/adr/029-platform-actions.md
- docs/architecture/agent-api-families.md §B2
- backlog/BP-061-platform-actions.md
- backlog/BP-046-record-merge-dedupe.md
- internal/seed/module_lead_marketing.go leadConvertActionDef
- internal/seed/module_sales.go quoteAcceptActionDef
- internal/actions/service.go (handlers: lead.convert, quote.accept only)
- docs/modules/core.md (says None in v1)

Mission: remaining BP-061 Phase 5 verb. quote.accept already shipped. Register record.merge on core. Invoke via POST /client/v1/actions/record.merge and ctx.invokeAction. Do not add a sibling route.

Do:
1. ActionDef on core (RequiresPackages: ["core"], SyncSafe: true) + bump CorePackageVersion. Handler internal/actions/record_merge.go calling dataengine.Merge (implement Merge here if Prompt 4 has not landed).
2. Wire s.handlers["record.merge"]. Catalog tests: always listed (core on). HTTP tests: happy merge, AuthZ deny rollback, EXTERNAL_ID_CONFLICT, alreadyMerged.
3. Guest invokeAction syncSafe test (follow invoke_action_sync_test.go / quote accept guest test).
4. docs/modules/core.md Platform actions table. BP-061 Phase 5 Done; status Mitigated (Phase 6 actionAccess still not started). Update BP-046 if merge GA lands.

Packages allowed: internal/actions, internal/seed, internal/packages (via seed), internal/httpapi (actions tests / thin glue only), internal/dataengine, internal/automation tests, docs/modules/core.md, backlog/BP-061, backlog/BP-046.
Packages forbidden: tools/control-ide/**, POST /merge, ctx.mergeRecords, PS actionAccess, Metadata action definitions table.

Tests: go test -p 1 ./internal/actions/... ./internal/httpapi/... ./internal/seed/... ./internal/dataengine/...
Non-goals: lead.convert/quote.accept rework, MCP invoke_action, composite action subrequests, IDE convert/merge buttons.
```
