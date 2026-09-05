# Customer-install simulation test run

A **second, heavier SI campaign** after [customer-rollout-test-run.md](./customer-rollout-test-run.md). The first campaign proved two sibling installs, one custom object, and one automation. This run simulates a **customer topology** (dev / test / prod), **managed-package enablement linked to a new custom object**, **guest TypeScript automations at scale**, and a **full Control IDE surface walk**.

Gap log: [customer-rollout-gap-log.md](./customer-rollout-gap-log.md) (same tables; beat prefix **`S-`**). Customer YAML stays under gitignored `.customer-sandbox/one-acme-sim` — never product `internal/seed`.

**Do not start this campaign from a docs-only PR.** This file is the runbook. Paste-ready executor prompts: [customer-install-simulation-playbook.md](./architecture/customer-install-simulation-playbook.md).

**Status:** alpha lab. Depends on first-campaign defects still open: [#28](https://github.com/MajestaNet/one/issues/28) (migrate race), [#29](https://github.com/MajestaNet/one/issues/29) (suite truncation). Re-test those; do not re-file duplicates.

## Locked choices

| Choice | Value |
|---|---|
| Lab | Path B Compose, **three** dedicated installs ([docker-compose.dev-test-prod.yml](../deploy/docker-compose.dev-test-prod.yml)) |
| Identity | Shared `CUSTOMER_ID=acme-sim`; unique `INSTALL_ID` / JWT issuer / claim token per role |
| Ship of record | Customer Git → `one org validate` → `one org deploy` ([customer-developer-workflow.md](./customer-developer-workflow.md)) |
| Packages | Enable **per install** via Metadata (`POST /metadata/v1/packages/{name}/enable`). Managed packs are **not** in the customer pack |
| Custom object | `SiteVisit__c` with lookups onto **enabled** managed objects (`Opportunity`, `Project`) plus `Account` / `Contact` |
| Automations | Guest Deno TS only (`one:automation`). Named set + generated scale stubs. No `npm:` / JSR / std |
| Control IDE | Optional JWT demo client ([ADR-030](./adr/030-install-agent-runtime.md)). Exercise **every shipped panel**. Do **not** treat Electron as Ship GUI of record |
| Promote | **Forbidden.** Peers are topology hints ([multi-env-deploy.md](./multi-env-deploy.md)) |
| Fixtures | `scripts/customer-install-sim-generate.sh` → `.customer-sandbox/one-acme-sim` |

Do **not** run this overlay at the same time as [docker-compose.multi-env.yml](../deploy/docker-compose.multi-env.yml) or [docker-compose.yml](../deploy/docker-compose.yml) — ports `8080` / `5432` collide.

## Allowed docs (testers)

Use **only** these first. Opening a build plan or Go source to finish a card is a **docs gap**.

- [self-host.md](./self-host.md)
- [local-development-mac.md](./local-development-mac.md)
- [modules/README.md](./modules/README.md) (enable API + DAG)
- [customer-connect.md](./customer-connect.md)
- [builder-connect.md](./builder-connect.md)
- [customer-developer-workflow.md](./customer-developer-workflow.md)
- [customer-repo.md](./customer-repo.md)
- [multi-env-deploy.md](./multi-env-deploy.md)
- [automation-sdk.md](./automation-sdk.md)
- [customer-ide-ux.md](./customer-ide-ux.md)
- this runbook + the gap log + the first campaign runbook

## Lab topology

```text
CUSTOMER_ID=acme-sim
├── acme-dev   role=dev    :8082  (day-to-day DX)
├── acme-test  role=test   :8081  (gate)
└── acme-prod  role=prod   :8080  (release)

customer Git (.customer-sandbox/one-acme-sim)
        │  same SHA
        ├─→ one org validate/deploy --alias dev    → :8082
        ├─→ one org validate/deploy --alias test   → :8081
        └─→ one org validate/deploy --alias prod   → :8080
```

| Install | `INSTALL_ID` | `INSTALL_ROLE` | API | Postgres host port | Claim token | Bootstrap admin key |
|---|---|---|---|---|---|---|
| Prod | `acme-prod` | `prod` | `http://localhost:8080` | `5432` | `sim-prod-claim-token-change-me` | `rollout-prod-admin` |
| Test | `acme-test` | `test` | `http://localhost:8081` | `5433` | `sim-test-claim-token-change-me` | `rollout-test-admin` |
| Dev | `acme-dev` | `dev` | `http://localhost:8082` | `5434` | `sim-dev-claim-token-change-me` | `rollout-dev-admin` |

Claim emails must differ (`admin-prod@example.com`, `admin-test@example.com`, `admin-dev@example.com`). Cross-install “one login unlocks all” is unsupported.

### Bring the lab up

From the **repo root**:

```bash
docker compose -f deploy/docker-compose.dev-test-prod.yml up --build -d
```

Wait until **each** API is ready (seed finishes after listen). Start order does not matter for Compose; for the native fallback, start **one API at a time** and wait `/readyz` before its worker ([#28](https://github.com/MajestaNet/one/issues/28)).

```bash
for p in 8080 8081 8082; do
  curl -fsS --retry 30 --retry-all-errors --retry-delay 2 "http://localhost:${p}/readyz"
done
```

Tear down:

```bash
docker compose -f deploy/docker-compose.dev-test-prod.yml down -v
```

### Native fallback (no Docker)

Three empty databases on one Postgres 16. Unique `INSTALL_ID` / JWT key / claim token / `PORT` per process. Export `DENO_PATH` in **both** API and worker shells.

```bash
createdb one_sim_prod && createdb one_sim_test && createdb one_sim_dev
export PATH="$HOME/.deno/bin:$PATH"
export DENO_PATH="$(command -v deno)"
export APP_ENV=development FEATURE_FLAGS=agents PRODUCT_VERSION=0.1.0 CUSTOMER_ID=acme-sim
# prod :8080 / one_sim_prod / acme-prod — wait /readyz, then worker
# test :8081 / one_sim_test / acme-test
# dev  :8082 / one_sim_dev  / acme-dev
```

Do **not** share `AUTH_JWT_SIGNING_KEY` or `INSTALL_CLAIM_TOKEN` across installs.

### Generate the customer tree

```bash
scripts/customer-install-sim-generate.sh
# optional: STUB_COUNT=48 scripts/customer-install-sim-generate.sh
```

Then `git init` inside `.customer-sandbox/one-acme-sim` (that tree is gitignored from **this** repo).

## Capture protocol (same as campaign 1)

Score **function and friction**. After **every** card S-A–S-E (and each listed beat), do these three steps:

1. **Add one row** to the gap log **Campaign 2 — Run results** table.
2. **File or skip an issue** using [customer-rollout-gap-log.md](./customer-rollout-gap-log.md). Template: [.github/ISSUE_TEMPLATE/campaign-finding.md](../.github/ISSUE_TEMPLATE/campaign-finding.md). Helper: `scripts/file-campaign-finding.sh`. Title prefix `[campaign S-…]`.
3. **Do not** open a new `backlog/BP-*.md` item.

| Field | What to write |
|---|---|
| Expected path | The operator doc you used first |
| Actual | One line in the run table |
| Outcome | `pass` / `pass-with-workaround` / `fail` / `blocked-no-display` / `not-run` |
| DX (1–5) | Docs findability, error quality, recovery, time-to-green |
| Class | `docs-drift` · `missing-lab-packaging` · `product-bug` · `authz-confusion` · `frozen-chrome-honesty` · `known-remainder` · `by-design` |
| Issue | `#N` / `none` / existing `#28`/`#29` |

## Scenario catalog

### S-A. Simulate setup of dev / test / prod

**Allowed docs:** [self-host.md](./self-host.md) Path B, [multi-env-deploy.md](./multi-env-deploy.md), this runbook.

1. Confirm three empty databases (fresh `down -v` then `up`).
2. Claim each install via HTTP (not the bootstrap key as the happy path):

```bash
curl -sS -X POST http://localhost:8080/auth/v1/install/claim \
  -H 'Content-Type: application/json' \
  -d '{"token":"sim-prod-claim-token-change-me","email":"admin-prod@example.com","password":"choose-a-long-password","displayName":"Prod Admin"}'
```

Repeat on `:8081` / `:8082` with the matching claim token and a **different** email. Save three JWTs.

3. Smoke `/version` + `/client/v1/me` on all three. Pin `One-API-Revision` from `GET /version`.
4. Register peers **without** loopback `baseUrl` (SSRF guard — first campaign G-PEER-LOCAL). Triangle: prod↔test, prod↔dev, test↔dev (six POSTs).
5. CLI aliases:

```bash
export ONE_CREDENTIAL_STORE=file
go run ./cmd/one auth login --base-url http://localhost:8080 --token "$PROD_JWT" --alias prod
go run ./cmd/one auth login --base-url http://localhost:8081 --token "$TEST_JWT" --alias test
go run ./cmd/one auth login --base-url http://localhost:8082 --token "$DEV_JWT"  --alias dev
go run ./cmd/one org use dev
go run ./cmd/one org list
```

6. Confirm workers are up. Guest automations and `--suite` need Deno in the **API** (suites) and **worker** (async).
7. Score: could you do this from [self-host.md](./self-host.md) + [multi-env-deploy.md](./multi-env-deploy.md) alone, or did you need this runbook because the default Compose file is a single `INSTALL_ROLE=dev` stack?

**Beats:** `S-A-COMPOSE` · `S-A-CLAIM-*` · `S-A-HEALTH` · `S-A-MIGRATE-RACE` (retest #28) · `S-A-PEERS` · `S-A-CLI-ALIASES`

### S-B. Packages enabled and linked to a new custom object

**Allowed docs:** [modules/README.md](./modules/README.md), [customer-repo.md](./customer-repo.md).

Managed packages are **install-local**. The customer pack only carries `ownership=custom` artifacts. Lookups onto `Opportunity` / `Project` / `Lead` **fail describe/deploy** until those packs are enabled **on that install**.

Enable **the same set on all three installs** (admin + `metadata` scope):

```text
catalog  →  sales          (sales depends on catalog)
project_service            (depends on core only)
lead_marketing             (Lead + lead.convert)
```

```bash
enable() {
  local base=$1 jwt=$2 name=$3
  curl -sS -X POST "${base}/metadata/v1/packages/${name}/enable" \
    -H "Authorization: Bearer ${jwt}" -H "One-API-Revision: 1"
}
# Negative: sales before catalog → expect 409 CONFLICT (dependency not installed)
enable http://localhost:8082 "$DEV_JWT" sales
# Happy path
for name in catalog sales project_service lead_marketing; do
  enable http://localhost:8082 "$DEV_JWT" "$name"
  enable http://localhost:8081 "$TEST_JWT" "$name"
  enable http://localhost:8080 "$PROD_JWT" "$name"
done
```

Idempotent re-enable should 200. `GET /metadata/v1/packages` should show enabled=true. `GET /client/v1/describe` (or object describe) should list `Opportunity`, `Project`, `Lead`.

**Custom object (generated):** `SiteVisit__c`

| Field | Type | Links |
|---|---|---|
| `Name` | text, required | |
| `Status` | picklist Planned / In Progress / Completed / Cancelled | |
| `OpportunityId` | lookup **Opportunity** (`sales`) | required |
| `ProjectId` | lookup **Project** (`project_service`) | optional |
| `AccountId` | lookup Account (`core`) | |
| `ContactId` | lookup Contact (`core`) | |

**Custom fields on managed objects** (still `ownership=custom` — they ship in the customer pack):

| Field | On | Type |
|---|---|---|
| `LastSiteVisitId__c` | Account | lookup `SiteVisit__c` |
| `SiteVisitCount__c` | Opportunity | number |
| `EngagementFlag__c` | Project | boolean |

**Negatives (expect failure, record the error):**

1. `one org validate` / deploy `SiteVisit__c` to an install that has **not** enabled `sales` — lookup target missing.
2. Enable `sales` without `catalog` (already above).
3. Mutate managed `Opportunity` field definitions via Metadata (rejected).
4. Soft-disable `sales` after the custom object exists — Client CRUD / automations on Opportunity; score the message.

**AuthZ:** after enable, non-admin permission sets get **deny stubs** on new managed objects. Prove a non-admin JWT cannot CRUD `Opportunity` until a PS grant; Admin claim JWT can.

**IDE twin (card S-E also):** Build → Packages enable `sales` on **dev** only, then env-switch to test and confirm test is still disabled until you enable there. That is the “packages are not Git” lesson.

**Beats:** `S-B-PKG-DEP` · `S-B-PKG-ENABLE-ALL` · `S-B-OBJ-SITEVISIT` · `S-B-FIELD-MANAGED-EXT` · `S-B-LOOKUP-FAIL-WITHOUT-PKG` · `S-B-AUTHZ-STUBS`

### S-C. Deploy automations at scale (TypeScript)

**Allowed docs:** [automation-sdk.md](./automation-sdk.md), [customer-repo.md](./customer-repo.md).

Two bands. Both live only under `.customer-sandbox/`.

#### Band 1 — named (must be real)

The generator writes **nine** guest TS automations + unit tests + suite `SiteVisitFromOpportunity`.

| apiName | Trigger | Exec | What it proves |
|---|---|---|---|
| `CreateSiteVisit_From_Opportunity` | Opportunity create | async | Custom object from managed `sales` row |
| `StampAccount_LastVisit` | SiteVisit__c create | async | Custom field on managed Account |
| `CreateProjectTask_From_Visit` | SiteVisit__c create | async | Managed `ProjectTask` (`project_service`) |
| `ConvertLead_When_Qualified` | Lead update | async | `ctx.invokeAction({ apiName: "lead.convert" })` — needs `lead_marketing` + `sales` for `createOpportunity` |
| `Reject_Missing_Opportunity` | SiteVisit__c create | **sync** | `{ ok: false }` rolls back the triggering write |
| `Fanout_TimeEntries` | Project create | async | 3 child `TimeEntry` rows (small fan-out) |
| `CloseVisit_When_Opp_Closed` | Opportunity update | async | Query + update custom rows |
| `CreateQuote_When_Won` | Opportunity update | async | Managed `Quote` |
| `Expense_When_Visit_Complete` | SiteVisit__c update | async | Managed `Expense` |

ADR-014: only `one:automation`. JSONLogic on `Name_Required` is an **error** condition (`true` = invalid).

Live path on **dev** (worker must be running):

```bash
# Opportunity create → wait → query SiteVisit__c
curl -sS -X POST http://localhost:8082/client/v1/sobjects/Opportunity \
  -H "Authorization: Bearer $DEV_JWT" -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -d '{"Name":"North Plant","StageName":"Prospecting","CloseDate":"2099-12-31"}'
```

Client query body field is `object` (not `objectApiName`); create responses use `Id`.

#### Band 2 — generated stubs (pack + worker stress)

Default `STUB_COUNT=48` automations `ScaleStub_01`… on `ScalePing__c` create (async, `ctx.log` + `{ ok: true }`). Then create **20** `ScalePing__c` rows on **dev** (48 × 20 = 960 `automation.run` jobs). Record:

- `one org validate` wall time
- `one org deploy --suite SiteVisitFromOpportunity` wall time and whether the CLI truncates suite JSON ([#29](https://github.com/MajestaNet/one/issues/29))
- Worker backlog / failed jobs / whether Client traffic still answers `/client/v1/me` during the fan-out ([BP-033](../backlog/BP-033-customer-runtime-isolation.md) remainder — do **not** open a new BP if jobs queue; class `known-remainder` unless the API wedges)

#### Import-ban negative

Copy `.customer-sandbox/one-acme-sim/_negative/forbidden_lodash_import.ts` into `src/automations/` and add a matching YAML, then `org validate` — expect pack/validate reject. Revert before the happy deploy.

**Beats:** `S-C-NAMED` · `S-C-IMPORT-BAN` · `S-C-STUBS` · `S-C-PACK-TIME` · `S-C-WORKER-FANOUT` · `S-C-SUITE` · `S-C-CLI-TRUNC` (retest #29)

### S-D. Same SHA to test and prod

**Allowed docs:** [customer-developer-workflow.md](./customer-developer-workflow.md).

1. Commit the customer tree. Clean working tree.
2. Dev already green from S-C.
3. Test: `one org use test`, **same Git SHA**, validate then deploy `--suite SiteVisitFromOpportunity`.
4. Prod: same SHA, validate then deploy. `GET /metadata/v1/objects/SiteVisit__c` on prod. Business **rows** must **not** copy.
5. **Package drift:** skip enabling `project_service` on a throwaway fourth thought-experiment — instead, on a **fresh** claim of one install, deploy before enabling `project_service` and record the error (or disable after enable if disable is what you have). The lesson: Git SHA is not enough if sibling installs have different enabled packs.
6. Anti-patterns: peer promote (removed), packing `.one/baseline`, mutating managed Account fields.

**Beats:** `S-D-DEPLOY-TEST` · `S-D-DEPLOY-PROD` · `S-D-NO-ROW-COPY` · `S-D-PKG-DRIFT`

### S-E. Control IDE — every shipped function (UI/UX)

**Allowed docs:** [customer-ide-ux.md](./customer-ide-ux.md), [local-development-mac.md](./local-development-mac.md), [customer-connect.md](./customer-connect.md).

**Desktop required.** If there is no display, mark the card `blocked-no-display` and keep the launch recipe as the procedure — same as campaign 1 card B.

Build once, isolate `userData`:

```bash
cd tools/control-ide
npm ci
npm run build
npx electron --user-data-dir="${HOME}/.local/share/one-control-ide-sim-a" .
# second process (optional concurrency)
npx electron --user-data-dir="${HOME}/.local/share/one-control-ide-sim-b" .
```

Walk **every** shipped surface. Score UX (findability, labels, errors, lying greens) — not “please add chrome”. Frozen list: [agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md#freeze-vs-finish). Honesty remainders: [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md). Brand paint: [BP-068](../backlog/BP-068-ide-brand-visual.md) (do not file “wrong navy” unless the shipped tokens are broken).

| Mode | Tools to open | What “pass” means |
|---|---|---|
| Sign in | Claim, browser SSO/password, client credentials, paste JWT | Session on `/client/v1/me`; failed auth is a real error, not a hang |
| Settings → Environments | Connect prod, then Add environment for test + dev | Three issuers; env switcher changes the JWT; quitting A does not sign out B |
| Operate | Graph home, List View, command-bar find (`Ctrl/Cmd+K`), drop a ToolSpec if any published | Graph seeds; search hits `POST /client/v1/search`; no auto-green hosted loop ([BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)) |
| Build → Objects | Describe `SiteVisit__c`; create a throwaway field on **dev** | Dual-write into customer repo `metadata/` — never `.one/baseline` |
| Build → Packages | Enable/disable catalog view; enable `notes` on **dev** only | Test remains disabled; Explorer may show catalog objects that are not enabled — score honesty |
| Build → Automations | Open a named automation; Monaco TS | Honest save vs Metadata; import-ban error if you paste `npm:lodash` |
| Build → Agents | List AgentSpecs; create wizard (harness) | No in-IDE coding-agent host (frozen) |
| Build → Tools | List ToolSpecs | Declarative only |
| Build → Repo | Choose `.customer-sandbox/one-acme-sim`; init/sync | Does not vendor the tree into product Git |
| Build → Deploy | Pack HEAD → Validate vs org → Deploy to org on **dev** | Status matches CLI; **lying-green on HTTP 200 alone is fail** |
| Build → Query / Monitor / Explorer | Query `SiteVisit__c`; Explorer graph; Monitor | Monitor empty-state is honest if ExecutionRun is not seeded ([BP-033](../backlog/BP-033-customer-runtime-isolation.md)) — class `known-remainder` / `frozen-chrome-honesty`, not a new BP |
| Govern | Users, Integrations, Permissions, outbound connectors | Live Client/Metadata; Hosting admin UI is frozen — do not file “missing DO console” |
| Settings | Account, Hosting, Inference, Environments | Inference test chat if BYO configured; otherwise honest empty |
| Chrome | Dark/light toggle, 2-slice workspace, agent dock, change status bar | Theme persists; max 1 tool + 1 agent |

**Negatives:** expired token; connect to `:8081` while believing it is prod; JWT without `deploy` on Deploy panel.

**Do not file defects asking for:** license UX, update CDN, Operate-as-CRM, BoardHandoff, new Query/Monitor/Explorer chrome, in-IDE agent host, peer promote GUI.

**Beats:** `S-E-SIGNIN` · `S-E-ENV-SWITCH` · `S-E-OPERATE` · `S-E-BUILD-*` · `S-E-GOVERN` · `S-E-SETTINGS` · `S-E-THEME` · `S-E-HONESTY` · `S-E-FROZEN`

## Dual-IDE and CLI cheat sheet

```bash
export ONE_CREDENTIAL_STORE=file
go run ./cmd/one auth login --base-url http://localhost:8082 --token "$DEV_JWT" --alias dev
go run ./cmd/one org use dev
go run ./cmd/one org validate -dir .customer-sandbox/one-acme-sim
go run ./cmd/one org deploy  -dir .customer-sandbox/one-acme-sim --suite SiteVisitFromOpportunity
# same SHA
go run ./cmd/one org use test && go run ./cmd/one org validate -dir .customer-sandbox/one-acme-sim
go run ./cmd/one org deploy  -dir .customer-sandbox/one-acme-sim --suite SiteVisitFromOpportunity
go run ./cmd/one org use prod && go run ./cmd/one org validate -dir .customer-sandbox/one-acme-sim
go run ./cmd/one org deploy  -dir .customer-sandbox/one-acme-sim --suite SiteVisitFromOpportunity
```

## Explicit non-goals

- Starting the test run from this docs PR
- New Control IDE chrome (frozen list)
- Client Experience / end-user CRM ([BP-040](../backlog/BP-040-client-experience-oss-kits.md))
- Path A DigitalOcean as the first venue (follow-on pass with the same cards if credentials exist)
- Automated GUI E2E in product CI
- Checking customer YAML into the product tree
- Opening a new BP from a campaign beat

## Related

- [customer-install-simulation-playbook.md](./architecture/customer-install-simulation-playbook.md) — spawn rules + paste-ready prompts
- [customer-rollout-test-run.md](./customer-rollout-test-run.md) — first campaign (prod + test)
- [customer-rollout-gap-log.md](./customer-rollout-gap-log.md)
- Open campaign defects: [#28](https://github.com/MajestaNet/one/issues/28), [#29](https://github.com/MajestaNet/one/issues/29)
