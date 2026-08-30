# Majesta One API families — build plan

Design for three commercial API surfaces on every dedicated install. See [ADR-004](./adr/004-three-api-families.md).

## Goals

1. **Client API** — let apps, integrations, and agents read/update business records and run platform operations that act on data.
2. **Metadata API** — let developers in a given install create and change the model (objects, fields, rules, automations, permission-set definitions).
3. **Deployment API** — migrate **customer-specific** implementation **between any of a customer’s environments** (N test/staging/prod installs, not a fixed single test→prod pair), including test runs; never treat the Majesta One product repo as the promotion unit.

## Non-goals (v1 of this split)

- Multi-tenant control plane or shared SaaS router.
- Forking / distributing Majesta One source per customer.
- GraphQL or UI builders.
- Promoting managed package internals (`core`, `platform`) via Deploy API (those ride product upgrades).

---

## 1. Product codebase vs customer implementation

| Concern | Lives where | How it changes |
|---|---|---|
| Majesta One product (Go API, data-engine, kernel schema, images) | This monorepo / GHCR + GitHub Release | Versioned product release → App Platform / Compose / Helm upgrade on each install |
| Managed metadata (`platform`, `core`) | Seeded/migrated by product version | Product image upgrade + API boot package migrate ([BP-007](./adr/020-cdm-managed-packages.md) mitigated) |
| Customer implementation | Per-environment DBs on that customer’s installs (+ optional customer Git of exported metadata) | Metadata API + customer Git; Deploy **repo→org** validate/apply per install ([BP-032](../backlog/BP-032-customer-dx-validate-deploy.md)) |
| Business data | Per-environment DB | Client API; **not** promoted by default |

**Commercial model implication:** customers buy/install Majesta One. Their “codebase” in the customer-customization sense is **metadata + tests (+ plugins later)** on their AWS instances—not a copy of our monorepo. Optional: customers may store exported customer packages in **their** Git for review; Deploy API applies those packages to an install.

Recommended environment topology per customer:

```
[Customer AWS account]
  one-test  (product vX + customer impl draft)
  one-prod  (product vX + customer impl released)
```

Both run the same product binary. Deploy credentials on Test push bundles to Prod’s Deploy API (or a CI job pulls from Test and posts to Prod).

---

## 2. Client API (`/client/v1`)

**Purpose:** do work on business data under permission sets.

| Area | Endpoints (target) | Notes |
|---|---|---|
| Describe | `GET /describe`, `GET /describe/:object` | Read-only schema for clients |
| Records | `POST|GET|PATCH|DELETE /sobjects/:object[/:id]` | CRUD |
| Query | `POST /query` | SQL-native planner; keyset pagination |
| Search | `POST /search` | Cross-object ranked find on `searchable` fields — **shipped** ([BP-043](../backlog/BP-043-cross-object-search-api.md) / [BP-020](../backlog/BP-043-cross-object-search-api.md); [plan](./architecture/cross-object-search-build-plan.md)). `scope: client`; AuthZ + sharing + FLS snippets |
| Composite | `POST /composite` | Batched client ops |
| Bulk | `POST /bulk/:object` | Sync create-only helper (small batches) |
| Upsert | `PATCH /sobjects/:object/:externalIdField/:externalId`, `POST /sobjects/:object/upsert` | External-ID create-or-update ([BP-041](../backlog/BP-041-record-external-id-upsert-bulk.md)) |
| Ingest jobs | `POST/GET /jobs/ingest…` | Async Bulk 2.0–inspired insert/update/upsert/delete ([build plan](./architecture/external-id-upsert-bulk-build-plan.md)). Datapack apply uses Client ingest jobs (upsert) when a step has **more than 500 rows**; smaller steps stay per-row REST upsert. |
| Events | `GET /events`, webhook delivery status (read) | Consume/observe; config stays Metadata |
| Activity feed | `GET /activity-feed?parentType=&parentId=` | Composed activities read (Task / Appointment / PhoneCall / Email); not a write SoR |
| Agents | `GET /agents/playbooks`, `POST /agents/runs` (+ `stream` SSE), `GET /agents/runs/:id`, `GET .../stream`, `POST .../approve` (JSON job or SSE continue) | Discover safe active AgentSpec summaries and execute with install inference routing (BP-052). Streaming create generates immediately; `require_approval` parks non-stream runs. SSE approve must not also enqueue `agent.run`. Definitions/instructions remain Metadata |
| Automations (runs) | `GET /automations`, `POST /automations/{apiName}/runs`, `GET /automations/runs/{id}` | Invoke **customer** automations as caller; definitions are Metadata ([BP-047](../backlog/BP-047-integrations-callable-oauth.md)) |
| Platform actions | `GET /actions`, `GET/POST /actions/{apiName}` | Product Go verbs (`lead.convert`, …); package-gated; guest `ctx.invokeAction` ([ADR-029](./adr/029-platform-actions.md) · [BP-061](../backlog/BP-061-platform-actions.md)) |
| Audit | `GET /audit` | Optional read for compliance integrations |
| Identity | `CRUD /principals`, credentials, `POST /principals/{id}/password`, `POST /me/password`, `CRUD /integrations` (+ secrets rotate/reveal), `GET /roles`, `POST /roles/assign`, `POST /permissions/assign` | Users/services/agents + Connected Apps (`identity.manage`); password set without email (BP-038); Cognito write-through; Role/PS binding |

**Auth:** scope `client`. Object/field CRUD still enforced by permission sets. Identity admin requires capability `identity.manage`.

**Does not:** create custom objects/fields; promote environments; mutate managed package definitions.

**Migration from today:** most of current `/v1/sobjects`, `/query`, `/composite`, `/bulk`, `/describe`, agent runs, event reads.

### MCP adapter (not a fourth family)

Install-local Streamable HTTP endpoint `POST /mcp` (+ convenience `GET /mcp/tools`) projects Client/Metadata tools for external agent runtimes ([ADR-010](./adr/010-customer-agentic-platform.md)). Supports `initialize`, `tools/list`, and `tools/call` with **stateless JSON** responses (no SSE sessions in v1). Gated by `FEATURE_FLAGS` including `agents`. AuthZ is identical to the mapped HTTP paths — MCP invents no capabilities. Connect recipes: [customer-connect.md](./customer-connect.md). AgentSpecs: [customer-agents.md](./customer-agents.md).

---

## 3. Metadata API (`/metadata/v1`)

**Purpose:** shape **this install’s** model. Writes are always local to the environment that receives the request.

| Area | Endpoints (target) | Notes |
|---|---|---|
| Objects | `GET|POST|PATCH|DELETE /objects`, `GET /objects/:apiName` | Customer-owned create/update/delete; managed read-only (403 on mutate) |
| Fields | `GET|POST` `/fields`, `PATCH|DELETE /fields/:object/:apiName` | Same ownership rules; `fieldType` must be canonical ([ADR-017](./adr/017-canonical-field-types.md)) |
| Field types | `GET` `/field-types` | Catalog + create rules for Object Manager / tooling |
| Validation | `GET|POST|PATCH /validation-rules` | JSONLogic |
| Automations | `CRUD /automations` | Definitions only |
| Permissions | `CRUD /permission-sets` (definitions) | Object + field + system permissions shape; assignment is Client |
| Install exposure | `GET|PUT /install/exposure`, `POST .../apply` | Edge/WAF path policy (`metadata.network`) |
| Packages | `GET /packages`, `GET /packages/:name`, `POST .../enable`, `POST .../disable` | Optional managed modules (admin); not Deploy. `objects` is the image-registry shape (lookups included) even when the package is not enabled — Control IDE Explorer visualizes from this. |
| Webhooks | `CRUD /webhooks` | Subscription config |
| Agent playbooks (AgentSpecs) | `CRUD /agents/playbooks` | Definitions (`instructions`, tools, scopes, ownership); runs are Client; customer-owned promote via Deploy |
| Projections | `POST /projections/:object/build` | Ops for query indexes driven by metadata |
| Snapshot | `GET /snapshot` | Export customer-owned metadata for Deploy bundles (includes customer fields on managed objects) |

**Auth:** scope `metadata` (often combined with `client` for admin keys).

**Ownership enforcement:**

- Reject mutations that alter `ownership=managed` artifacts (except product seed / module enable migrate).
- Tag new artifacts `ownership=custom`, `namespace` / `packageName` defaulting to customer package id (e.g. `customer.default`).
- Optional managed modules: catalog + enable/soft-disable under `/packages` (defs ship in product image; see [docs/modules/README.md](./modules/README.md)).
- Object delete refuses when fields or validation rules still exist; field delete removes the matching relationship row when present.
- Soft-delete / deprecate rather than hard-delete when referenced (follow-up hardening).

**Migration from today:** `/v1/metadata/*`, automations CRUD, permissions CRUD, webhook registration, playbook definitions, projection build.

---

## 4. Deployment API (`/deploy/v1`)

**Purpose:** treat customer implementation as a releasable artifact between **the same customer’s** environments.

### 4.1 Concepts

| Concept | Meaning |
|---|---|
| **Bundle** | Immutable snapshot: manifest + customer metadata + optional customer tests + checksum |
| **Validation** | Dry-run apply: conflict detection vs target managed+customer state |
| **Test run** | Execute customer-defined tests against an install (usually Test) via Client+Metadata as needed |
| **Promotion** | Apply a validated bundle to a target environment (usually Prod) |
| **Product upgrade** | Out of band — ECS image roll via SSM Automation or `/ops/v1` ([ADR-007](./adr/007-platform-ops-upgrades.md)); Deploy API may *check* `productVersion` compatibility only |

### 4.2 Target endpoints

| Method | Path | Behavior |
|---|---|---|
| `POST` | `/bundles` | Create bundle from current customer snapshot (or uploaded package) |
| `GET` | `/bundles`, `/bundles/:id` | List/get |
| `GET` | `/bundles/:id/artifact` | Download signed artifact (metadata JSON + tests) |
| `POST` | `/bundles/:id/validate` | Validate against **this** install (or declared target profile) |
| `POST` | `/promotions` | Apply bundle to this install (typical: called on Prod) |
| `GET` | `/promotions/:id` | Status, logs, rollback marker |
| `POST` | `/tests` | Register/update customer test suite definitions (or via Metadata) |
| `POST` | `/tests/runs` | Start programmatic test run |
| `GET` | `/tests/runs/:id` | Results |
| `GET` | `/environment` | Product version, `customerId`, install id, free-form role, peer mode, `cloudHost`, capabilities (`cloud` / `digitaloceanCloud` when adapter configured), customer repo URL |
| `POST` | `/packages/pack` | Upload zip/tar of `one/v1` tree → create Deploy bundle |
| `GET` | `/packages/export` | Current customer snapshot (+ tests + managed baseline) as `one/v1` zip |
| `POST` | `/packages/initialize-repo` | Admin+deploy: seed remote `main` from this install (go-git; requires `CUSTOMER_REPO_URL`) |
| `GET` | `/cloud/status` | Cloud adapter configured?; binding; reachability (no credential echo) |
| `PUT` | `/cloud/binding` | Bind this install to opaque `appResourceId` / `databaseResourceId` (**admin**) |
| `GET` | `/cloud/app` | Live app summary for bound app |
| `PATCH` | `/cloud/app/scale` | Scale api/worker instances or `sizeClass` (**admin**) |
| `PATCH` | `/cloud/database/resize` | Resize managed Postgres size class / `numNodes` (**admin**) |
| `POST` | `/cloud/environments` | Provision peer app + DB + peer row (**admin**). Shared `CUSTOMER_ID`, new `INSTALL_ID`. Requires unique `installId`, `apiKeys`, and `authJwtSigningKey`. |
| `GET` | `/cloud/environments` | Peers + provision audit runs |
| `POST` | `/cloud/app/redeploy` | Temporary digest redeploy helper (**admin**; prefer `/ops/v1` long-term) |

**Compatibility aliases:** the same verbs under `/cloud/digitalocean/*` (legacy `appId` / size slugs still accepted).

**Host-agnostic contract:** primary consumer surface is host-free `/deploy/v1/cloud/*`. DigitalOcean is the only **product** adapter today; other hosts map the same verbs via community adapters — see [deploy-cloud-capability-contract.md](./architecture/deploy-cloud-capability-contract.md) and [deploy-cloud-agnostic-build-plan.md](./architecture/deploy-cloud-agnostic-build-plan.md). Product image rolls stay **Ops** (ADR-007), not Deploy cloud.

See [customer-repo.md](./customer-repo.md) and [ADR-012](./adr/012-customer-repo-and-control-ide.md).

### 4.3 What a bundle contains

```json
{
  "manifestVersion": 1,
  "productVersionRange": ">=0.1.0 <0.2.0",
  "sourceInstallId": "cust-a-test",
  "createdAt": "...",
  "artifacts": {
    "metadata": [ "customer objects/fields/rules/automations/permissionSets/webhooks/playbooks" ],
    "tests": [ "customer test definitions" ]
  },
  "excludes": ["managed packages", "business records", "api keys", "audit history"]
}
```

### 4.4 Promote flow (happy path)

Customers may run **any number** of installs under one `CUSTOMER_ID` (e.g. `test-a`, `test-eu`, `staging`, `prod`). **Recommended path:** pack from customer Git and `org validate` / `org deploy` (or Ship **Validate vs org** / **Deploy to org**) against each install URL from the same Git SHA. There is no install→install peer push API.

```
Customer Git / local pack              Connected install
───────────────────────              ─────────────────
one org validate  ──────► POST /packages/validate-local
one org deploy    ──────► POST /promotions { bundleId }
(switch base URL; same SHA)  ──────► repeat on next env
```

**Safety rules:**

1. Promote **customer-owned** artifacts only.
2. Refuse apply if target `productVersion` outside bundle range.
3. Refuse clobber of managed apiNames; refuse delete of managed.
4. Prefer transactional apply per artifact group; record promotion audit.
5. Data migration of `records` is **opt-in later**, never default.
6. Deploy scope keys are environment-bound. Cross-env release is repo→org (re-validate/deploy the same Git SHA); install→install artifact transfer is removed.

### 4.5 Programmatic tests

Customer tests are metadata (or Deploy-owned definitions) that exercise Client API behavior, e.g.:

- Assert object/field exists after apply.
- Create fixture records, run validation rules, assert outcomes.
- Call query/composite smoke paths.

Test runner executes inside the install (worker job), reports JUnit-like JSON via Deploy API. CI typically gates promotion on green runs on a chosen source install before posting the artifact to peers. Product image rolls reuse the same runner for **`PlatformSmoke`** (and optional customer **`PostUpgradeSmoke`**) — see [product-upgrades.md](./product-upgrades.md).

---

## 5. Ops API (`/ops/v1`)

**Purpose:** orchestrate **product** container upgrades on **this** install (confirm → ECS roll → test gate → rollback). Does not promote customer metadata.

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/upgrades/available` | Current product version + ECS target config |
| `POST` | `/upgrades` | Confirm upgrade (images + version); create run |
| `GET` | `/upgrades`, `/upgrades/:id` | List/get upgrade runs |
| `POST` | `/upgrades/:id/rollback` | Roll back to previous task definition |

**Auth:** scope `ops`. Confirm and rollback also require **admin**. See [ADR-007](./adr/007-platform-ops-upgrades.md).

---

## 6. Auth model (ADR-006 / BP-013)

| Scope | Typical principal (unified `users` row) |
|---|---|
| `client` | Human (Slack/UI), integration `service`, `agent` with Client Role |
| `metadata` | Developer / `agent` with Metadata Role on this install |
| `deploy` | CI `service` / release bot |
| `ops` | Install AWS admin / break-glass upgrade principal |

**Shipped (P1):** `POST /auth/v1/token` (client credentials → Majesta One JWT), Bearer Majesta One JWT on family routes, bootstrap API keys still accepted. See [ADR-006](./adr/006-jwt-auth.md). **Shipped:** opaque `refresh_token` on interactive human grants + `grant_type=refresh_token` / `POST /auth/v1/revoke` ([BP-063](../backlog/BP-063-refresh-token-sessions.md) · [refresh-token-session-build-plan.md](./architecture/refresh-token-session-build-plan.md)).

**Transitional:** encode scopes in API key env entries (`key:client+metadata+deploy+ops`) and optional OIDC JWT claims/groups. Cognito is not the long-term product default.

Admin privilege does **not** bypass missing family scopes (ADR-004).

---

## 7. Kernel / package changes

| Change | Why |
|---|---|
| `ownership` + `package_name` on metadata objects/fields (and related) | Gate Deploy inclusion |
| `package_installs` (+ `enabled`) | Managed core + optional module versions / soft-disable |
| Tables: `deploy_bundles`, `deploy_promotions`, `customer_tests`, `customer_test_runs` | Deploy API persistence |
| Tables: `platform_upgrades` | Ops product upgrade runs (ADR-007) |
| Install config: `CUSTOMER_ID`, `INSTALL_ID`, `INSTALL_ROLE`, `PRODUCT_VERSION`, `API_REVISION_CURRENT`, `API_REVISION_MIN`, `DEPLOY_PEER_MODE`, optional `ECS_*` | Multi-env identity + Ops ECS drive + client-pinnable API revision ([ADR-025](./adr/025-api-revision-versioning.md)) |
| `role_api_scopes` includes `ops` | Ops family scope |
| Router: `/client/v1`, `/metadata/v1`, `/deploy/v1`, `/ops/v1` + deprecated `/v1` | Commercial surfaces |
| Client pin: `One-API-Revision` (optional `/r{N}/` under family) | Graduated wire compat inside family majors ([BP-025](../backlog/BP-025-ide-api-version-compatibility.md)) |

---

## 7a. API revision (inside family majors)

Do **not** conflate product image semver with the wire pin. Clients send `One-API-Revision: N`; the install advertises `{min,current}` on `GET /version`. Family paths stay `/client/v1` (etc.); rare `/v2` remains an ADR-004 breaking-family event. Full design: [ADR-025](./adr/025-api-revision-versioning.md) · [build plan](./architecture/ide-api-version-compatibility-build-plan.md).

---

## 8. Implementation phases

### Phase A — Surface split (low risk) ✅

- Mount three Hono routers; move existing routes into Client vs Metadata.
- Add scope checks on API keys (`key:client+metadata+deploy`).
- Keep `/v1` aliases.
- Update OpenAPI stub + tests for new paths (aliases still tested).
- Deploy family exposes `GET /deploy/v1/environment` stub only.

### Phase B — Ownership model ✅

- Persist `ownership` / `packageName` on metadata; seed managed packages correctly.
- Metadata API rejects managed mutations.
- `GET /metadata/v1/snapshot` returns customer-only export.

### Phase C — Bundles on one install ✅

- Implement `deploy-engine` snapshot → bundle store.
- Validate + apply bundle **to the same** install (round-trip) for tests.
- Kernel tables + migrations (`deploy_bundles`, `deploy_promotions`).
- Deploy API: `POST/GET /bundles`, `…/validate`, `POST/GET /promotions`.

### Phase D — Programmatic tests ✅

- Customer test definitions + runner (`objectExists`, `fieldExists`, `createRecord`, `assertValidation`, `query`).
- `POST /deploy/v1/tests`, `POST /deploy/v1/tests/runs` (+ results GET); async via `customer.test.run` worker job.
- CI example: [ci-customer-tests.md](./ci-customer-tests.md).
- Kernel tables: `customer_tests`, `customer_test_runs`.

### Phase E — Cross-environment promote ✅

- Multi-environment: shared `CUSTOMER_ID`, free-form `INSTALL_ROLE`, N test/staging installs allowed.
- Repo→org DX: `POST /deploy/v1/packages/validate-local` then `POST /deploy/v1/promotions` with `{ bundleId }` on the **connected** install ([BP-032](../backlog/BP-032-customer-dx-validate-deploy.md)).
- Optional peer registry (`POST/GET /peers`) for IDE env topology only.
- Peer push and inbound `{ artifact }` promote **removed**.
- Docs: [multi-env-deploy.md](./multi-env-deploy.md).

### Phase F — Product upgrade orchestration (ADR-007) ✅

- ECS deployment circuit breaker + SSM Automation `One-ProductUpgrade`.
- `/ops/v1/upgrades` + scope `ops`; product `PlatformSmoke` suite seed.
- Docs: [product-upgrades.md](./product-upgrades.md).

---

## 9. Mapping of current `/v1` routes

| Current | Target family |
|---|---|
| `/describe`, `/sobjects`, `/query`, `/search`, `/composite`, `/bulk` | Client |
| `/events` (read), `/agents/runs` | Client |
| `/audit` | Client (read) |
| `/metadata/*`, `/automations`, `/permissions`, `/webhooks` | Metadata |
| `/agents/playbooks` | Metadata |
| `/projections/*/build` | Metadata |
| *(new)* bundles, promotions, test runs | Deploy |
| *(new)* product upgrades | Ops |

---

## 10. Success criteria

- A customer can customize any source install via Metadata API without forking Majesta One.
- CI can create a bundle, run tests, and promote **only customer changes** to any peer install of the same customer (including multiple test envs → staging/prod).
- Product upgrades remain ECS/Fargate image rolls (SSM Automation or `/ops/v1`) and do not require Deploy promotions.
- Client integrations never need Metadata, Deploy, or Ops scopes.
- Docs and backlog clearly state product vs customer-implementation separation; BP-010 mitigated.

## Related

- [ADR-004](./adr/004-three-api-families.md)
- [ADR-006](./adr/006-jwt-auth.md)
- [ADR-007](./adr/007-platform-ops-upgrades.md)
- [ADR-025](./adr/025-api-revision-versioning.md)
- [BP-010](../backlog/BP-010-three-api-families.md)
- [BP-013](../backlog/BP-013-jwt-unified-principals.md)
- [BP-007 Package versioning](./adr/020-cdm-managed-packages.md)
- [BP-025 IDE / API revision compatibility](../backlog/BP-025-ide-api-version-compatibility.md)
- [ADR-001](./adr/001-dedicated-install.md)
