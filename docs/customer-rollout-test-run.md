# Customer rollout end-to-end test run

A **scripted, scored customer journey** that a new SI (or vendor agent) can attempt against two sibling installs. The goal is to find **product, docs, and ease-of-use gaps**, not to replace `make test` / `make test-ide`.

Gap log: [customer-rollout-gap-log.md](./customer-rollout-gap-log.md). Fill a row per scenario card. Customer YAML stays under gitignored `.customer-sandbox/` — never product `internal/seed`.

**Status:** alpha lab. First headless run (2026-09-05, native two-install, no Electron) is in [customer-rollout-gap-log.md](./customer-rollout-gap-log.md). Path A DigitalOcean is a follow-on pass with the same cards if DO credentials exist.

## Locked choices

| Choice | Value |
|---|---|
| Lab | Path B Compose, **two** dedicated installs ([docker-compose.multi-env.yml](../deploy/docker-compose.multi-env.yml)) |
| Ship of record | Customer Git → `one org validate` → `one org deploy` ([customer-developer-workflow.md](./customer-developer-workflow.md)) |
| Control IDE | Optional JWT demo client ([ADR-030](./adr/030-install-agent-runtime.md)). Exercise 2+ processes; do **not** treat Electron as Ship GUI of record |
| Agentic path | Install MCP (`POST /mcp`) + `one` CLI ([builder-connect.md](./builder-connect.md)) |
| Hosted loop | `POST /client/v1/agents/runs` is a **second** agent path (Client tools only — no Metadata upsert / `org_*`) |
| Promote | **Forbidden.** Peers are topology hints, not a transfer channel ([multi-env-deploy.md](./multi-env-deploy.md)) |

The everyday single-stack file [deploy/docker-compose.yml](../deploy/docker-compose.yml) remains one install (`INSTALL_ROLE=dev`). This campaign is **prod-first**, then an opt-in test sibling.

## Allowed docs (testers)

Use **only** these first. Opening a build plan or Go source to finish a card is a **docs gap**.

- [self-host.md](./self-host.md)
- [local-development-mac.md](./local-development-mac.md)
- [customer-connect.md](./customer-connect.md)
- [builder-connect.md](./builder-connect.md)
- [customer-developer-workflow.md](./customer-developer-workflow.md)
- [customer-repo.md](./customer-repo.md)
- [multi-env-deploy.md](./multi-env-deploy.md)
- this runbook + the gap log

## Lab topology

Shared `CUSTOMER_ID=acme-rollout`. Each install has its own Postgres, JWT issuer, and claim token.

| Install | `INSTALL_ID` | `INSTALL_ROLE` | API | Postgres host port | Claim token | Bootstrap admin key |
|---|---|---|---|---|---|---|
| Prod | `acme-prod` | `prod` | `http://localhost:8080` | `5432` | `rollout-prod-claim-token-change-me` | `rollout-prod-admin` |
| Test | `acme-test` | `test` | `http://localhost:8081` | `5433` | `rollout-test-claim-token-change-me` | `rollout-test-admin` |

```text
customer Git (.customer-sandbox/one-acme-rollout)
        │  same SHA
        ├─→ one org validate/deploy --alias test   → :8081
        └─→ one org validate/deploy --alias prod   → :8080

Control IDE A ── JWT per install ──► prod + test (env switcher)
Control IDE B ── JWT ──► prod (stays on prod while A works on test)
MCP host + one CLI ── Bearer + One-API-Revision ──► test
```

### Bring the lab up

From the **repo root** (the directory that contains `Makefile`):

```bash
docker compose -f deploy/docker-compose.multi-env.yml up --build -d
```

Wait until each API is ready (seed finishes after listen):

```bash
curl -fsS http://localhost:8080/healthz
curl -fsS --retry 30 --retry-all-errors --retry-delay 2 http://localhost:8080/readyz
curl -fsS http://localhost:8081/healthz
curl -fsS --retry 30 --retry-all-errors --retry-delay 2 http://localhost:8081/readyz
```

Logs should show `one-api listening`, then `bootstrap/seed complete`. Tear down:

```bash
docker compose -f deploy/docker-compose.multi-env.yml down -v
```

### Native fallback (no Docker)

Two empty databases on one Postgres 16, unique `INSTALL_ID` / JWT key / claim token / `PORT` per process. Export `DENO_PATH` (or put Deno 2.9.3 on `PATH`) in **both** API and worker shells — suites run Deno in the API; async automations run Deno in the worker.

Start **prod API first**, wait `/readyz`, then its worker; repeat for test. API and worker both call `EnsureKernel`; concurrent first-boot migrate can fail on a non-idempotent kernel SQL file (gap log G-MIGRATE-RACE).

```bash
# example — two DBs on 127.0.0.1:5432
createdb one_prod && createdb one_test
export PATH="$HOME/.deno/bin:$PATH"
export DENO_PATH="$(command -v deno)"
export APP_ENV=development FEATURE_FLAGS=agents PRODUCT_VERSION=0.1.0 CUSTOMER_ID=acme-rollout
# prod
DATABASE_URL=postgres://one:one@127.0.0.1:5432/one_prod PORT=8080 \
  INSTALL_ID=acme-prod INSTALL_ROLE=prod \
  AUTH_JWT_SIGNING_KEY=rollout-prod-jwt-hmac-secret-change-me-32b \
  INSTALL_CLAIM_TOKEN=rollout-prod-claim-token-change-me \
  API_KEYS=rollout-prod-admin+admin PLATFORM_PUBLIC_URL=http://localhost:8080 \
  go run ./cmd/api
# after /readyz — worker with the same DATABASE_URL (no PORT)
# then repeat INSTALL_ID=acme-test PORT=8081 / one_test
```

Copy [`.env.example`](../.env.example) for a single-install `make api` loop. Two-install native labs should **not** share `AUTH_JWT_SIGNING_KEY` or `INSTALL_CLAIM_TOKEN`.

Headless helper: [scripts/customer-rollout-headless.sh](../scripts/customer-rollout-headless.sh) (Compose by default; `SKIP_COMPOSE=1` when the APIs are already up).

## Capture protocol (ease of use)

Score **function and friction**. Each card records:

| Field | What to write |
|---|---|
| Expected path | The operator doc you used first |
| Actual steps | Count, elapsed time, dead ends |
| Outcome | `pass` / `pass-with-workaround` / `fail` |
| DX (1–5) | Docs findability, error quality, recovery, time-to-green |
| Gap class | `docs-drift` · `missing-lab-packaging` · `product-bug` · `authz-confusion` · `frozen-chrome-honesty` ([BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)) · `known-remainder` (BP-048 / BP-031 / BP-065) |

Copy the row template from [customer-rollout-gap-log.md](./customer-rollout-gap-log.md).

## Scenario catalog

### A. Running the backend

**Allowed docs:** [self-host.md](./self-host.md) Path B, this runbook.

1. Confirm two empty databases (fresh `down -v` then `up`).
2. Start **prod first** (`INSTALL_ROLE=prod` on `:8080`).
3. Claim via HTTP — not the bootstrap key as the happy path:

```bash
curl -sS -X POST http://localhost:8080/auth/v1/install/claim \
  -H 'Content-Type: application/json' \
  -d '{"token":"rollout-prod-claim-token-change-me","email":"admin-prod@example.com","password":"choose-a-long-password","displayName":"Prod Admin"}'
```

Save `access_token`. Repeat on `:8081` with the **test** claim token and a **different** email (`admin-test@example.com`). Cross-install “one login unlocks all” is unsupported.

4. Smoke:

```bash
curl -sS http://localhost:8080/version
curl -sS -H "Authorization: Bearer $PROD_JWT" -H "One-API-Revision: 1" \
  http://localhost:8080/client/v1/me
```

Pin `One-API-Revision` from `GET /version` (`apiRevision.recommended`, alias of `current`).

5. Confirm workers are running (`docker compose -f deploy/docker-compose.multi-env.yml ps`). Guest automations and `--suite` need Deno **in the worker image** (the product Dockerfile installs 2.9.3). Host `PATH` Deno matters for `one org deploy --suite` when the CLI runs the suite from the operator machine.

6. Score: could you do this from [self-host.md](./self-host.md) alone, or did you need this runbook because the default Compose file is a single `INSTALL_ROLE=dev` stack?

### B. Connecting 2+ Control IDEs

**Allowed docs:** [local-development-mac.md](./local-development-mac.md), [customer-connect.md](./customer-connect.md) Path A.

Session files live in Electron `userData` (`session.bin` / `session.key`). Two processes that share `userData` share the JWT. Isolate them.

Build once, then launch **two** shells (Chromium switch **before** the app path):

```bash
cd tools/control-ide
npm ci
npm run build

# IDE A
npx electron --user-data-dir="${ONE_IDE_A_DATA:-$HOME/.local/share/one-control-ide-a}" .

# IDE B (second terminal)
npx electron --user-data-dir="${ONE_IDE_B_DATA:-$HOME/.local/share/one-control-ide-b}" .
```

`npm run electron:dev` is the single-session loop. Extra args after `--` are appended **after** `.` and may **not** be treated as Chromium switches — use `npx electron --user-data-dir=… .` after `npm run build`.

1. Both IDEs: Settings → Environments → Connect to `http://localhost:8080`. Sign in independently (claim JWT, PKCE, or client credentials). Confirm two `/client/v1/me` subjects and that quitting A does not sign out B.
2. Concurrent use: IDE A Operate List View; IDE B Build → Objects. After a metadata write on B, refresh describe on A (cache epoch).
3. Negatives: expired token; `http://localhost:8081` while thinking it is prod; a JWT without `deploy` on `GET /deploy/v1/environment`.
4. Honesty ([BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)): Deploy must not show customer tests **Passed** on HTTP 200 alone; Operate chat must not auto-green a tool loop that did not run.

**Desktop required.** If this lab has no display, mark B `blocked-no-display` and keep the launch recipe as the procedure.

### C. Spin up and connect a test env besides prod

**Allowed docs:** [multi-env-deploy.md](./multi-env-deploy.md).

The test install is already in the multi-env Compose file (`:8081`). Claim it (scenario A). Then:

1. Register peers **both ways**. `baseUrl` is for IDE env-switcher hints, **not** promote.

```bash
# On prod, register test (omit baseUrl on loopback labs — see gap G-PEER-LOCAL)
curl -sS -X POST http://localhost:8080/deploy/v1/peers \
  -H "Authorization: Bearer $PROD_JWT" -H "One-API-Revision: 1" \
  -H 'Content-Type: application/json' \
  -d '{"installId":"acme-test","installRole":"test","label":"Acme Test"}'

curl -sS -X POST http://localhost:8081/deploy/v1/peers \
  -H "Authorization: Bearer $TEST_JWT" -H "One-API-Revision: 1" \
  -H 'Content-Type: application/json' \
  -d '{"installId":"acme-prod","installRole":"prod","label":"Acme Prod"}'
```

If `POST /deploy/v1/peers` with `"baseUrl":"http://localhost:8081"` fails, record the status and message. Loopback peer URLs are blocked as an SSRF guard; the happy path for a public install is an HTTPS origin. Local workaround: omit `baseUrl` and use IDE **Add environment…** with the URL pasted by hand. Unclear errors here are a DX gap.

2. IDE A: add the test URL; sign in **again** (new issuer). Env switcher shows prod + test; switching changes the JWT used for API calls.
3. IDE B stays on prod while A works on test.
4. CLI aliases:

```bash
go run ./cmd/one auth login --base-url http://localhost:8080 --token "$PROD_JWT" --alias prod
go run ./cmd/one auth login --base-url http://localhost:8081 --token "$TEST_JWT" --alias test
go run ./cmd/one org use test
go run ./cmd/one org list
```

`ONE_CREDENTIAL_STORE=file` if the lab has no OS keychain. There is **no** `one org scratch` (BP-048 remainder) — a second env is a full install.

### D. Develop a custom object and automation

Do this **on test**. Compare three authoring surfaces for the **same** delta.

#### D1. Baseline sample (template)

```bash
mkdir -p .customer-sandbox
go run ./cmd/one project init -dir .customer-sandbox/one-acme-rollout --customer-id acme-rollout
# Edit one.yaml customerId if the scaffold still says REPLACE_CUSTOMER_ID
cd .customer-sandbox/one-acme-rollout
git init && git add . && git commit -m "init one/v1"
```

The product sample ([deploy/customer-repo-template](../deploy/customer-repo-template)) ships `Referral__c` + `CreateAccount_From_Contact` + suite `CreateAccountFromContact`. Deploy that to **test** first:

```bash
go run ./cmd/one org validate -dir .customer-sandbox/one-acme-rollout --alias test
go run ./cmd/one org deploy -dir .customer-sandbox/one-acme-rollout --alias test --suite CreateAccountFromContact
```

#### D2. New work — `Project__c` + `CreateProject_From_Account`

Hand-edit in the customer repo (surface 1). Copy-paste shapes. JSONLogic expressions are **error conditions**: `true` means the record is invalid (same polarity as Salesforce validation rules). ADR-014: only `one:automation`.

`metadata/objects/Project__c.yaml`

```yaml
apiName: Project__c
label: Project
pluralLabel: Projects
storageMode: flexible
ownership: custom
packageName: customer.default
features: {}
```

`metadata/fields/Project__c/Name.yaml` — `fieldType: text`, `required: true`.  
`metadata/fields/Project__c/AccountId.yaml` — `fieldType: lookup`, `referenceTo: Account`.  
`metadata/validation-rules/Project__c/Name_Required.yaml`:

```yaml
objectApiName: Project__c
apiName: Name_Required
label: Name required
active: true
errorMessage: Project Name is required
ownership: custom
packageName: customer.default
expression:
  # JSONLogic true => record is invalid (Name missing).
  "!":
    var: Name
```

Automation YAML mirrors `CreateAccount_From_Contact` (`objectApiName: Account`, `triggerEvent: create`, `execution: async`, `entryFile: src/automations/create_project_from_account.ts`). Guest TS: `ctx.createRecord` a `Project__c` with `Name` from the Account and `AccountId: ctx.trigger.recordId`. Suite `CreateProjectFromAccount`: `objectExists` Project__c + `automationUnitPass` + `automationContract` expecting a Project row.

Surfaces 2–3 for the **same** object:

- Control IDE Build → Objects / Automations (if panels are honest; else log BP-066).
- MCP `upsert_object` / `upsert_field` (customize job class) — scenario F.

Prove runtime on test: create an Account → worker runs the automation → query `Project__c`. Client query body field is `object` (not `objectApiName`); create responses use `Id`. Grant `automationAccess` / `canRun` on a permission set; confirm deny without it.

```bash
curl -sS -X POST http://localhost:8081/client/v1/sobjects/Account \
  -H "Authorization: Bearer $TEST_JWT" -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -d '{"Name":"Live Rollout Account"}'
# wait for worker, then:
curl -sS -X POST http://localhost:8081/client/v1/query \
  -H "Authorization: Bearer $TEST_JWT" -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -d '{"object":"Project__c"}'
```

### E. Deploy from source (repo → org)

**Allowed docs:** [customer-developer-workflow.md](./customer-developer-workflow.md).

1. Commit the customer tree. Clean working tree (`git status`).
2. Test: `one org validate` then `one org deploy --suite CreateProjectFromAccount`. Keep `CreateAccountFromContact` green.
3. Prod: `one org use prod`, **same Git SHA**, validate then deploy. `GET /metadata/v1/objects/Project__c` on prod. Business **rows** must **not** copy (data packs are a separate Client path).
4. Twin in IDE A: Build → Deploy → Pack from local HEAD → Validate vs org → Deploy to org. GUI status must match CLI (lying-green is a fail).
5. Anti-patterns (expect failure): peer promote (removed), packing `.one/baseline`, mutating managed `Account` fields.

### F. Agentic development

Point the agent at **test**, in parallel with the IDEs.

1. `FEATURE_FLAGS` includes `agents` on this Compose lab. Empty `FEATURE_FLAGS` also **enables** agents (`Config.AgentsEnabled`). To keep MCP dark, set a **non-empty** list that does not include `agents` — omitting the variable is not enough.
2. Create an agent principal (claim JWT / admin key), Roles with `client` + `metadata` + `deploy` (SystemAdmin is enough for the lab), issue a credential, mint `grant_type=client_credentials`.
3. Catalog:

```bash
curl -sS http://localhost:8081/mcp/tools \
  -H "Authorization: Bearer $AGENT_JWT" -H "One-API-Revision: 1"
```

Expect `upsert_object`, `org_validate`, `org_deploy`, `pack`, `invoke_skill`, `install_version`, …

4. MCP host config ([builder-connect.md](./builder-connect.md)):

```json
{
  "mcpServers": {
    "one": {
      "url": "http://localhost:8081/mcp",
      "headers": {
        "Authorization": "Bearer <agent_jwt>",
        "One-API-Revision": "1"
      }
    }
  }
}
```

JSON-RPC smoke (`Accept: application/json, text/event-stream`):

```bash
curl -sS -X POST http://localhost:8081/mcp \
  -H "Authorization: Bearer $AGENT_JWT" \
  -H "One-API-Revision: 1" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"rollout","version":"0"}}}'
```

5. Agent task: add `Project__c` + automation + suite, validate, deploy to test. Record whether customer-repo `skills/customize` and `skills/ship` are enough without vendor `.cursor/`.
6. Contrast hosted loop: `POST /client/v1/agents/runs` **cannot** upsert metadata or `org_deploy`. If Operate chat pretends otherwise, that is BP-066.
7. Stdio fallback [`tools/one-mcp`](../tools/one-mcp) only if the host cannot speak Streamable HTTP.

## Dual-IDE and CLI cheat sheet

```bash
# Isolated Electron (after npm run build in tools/control-ide)
npx electron --user-data-dir="$HOME/.local/share/one-control-ide-a" .
npx electron --user-data-dir="$HOME/.local/share/one-control-ide-b" .

# one aliases (file store when no keychain)
export ONE_CREDENTIAL_STORE=file
go run ./cmd/one auth login --base-url http://localhost:8080 --token "$PROD_JWT" --alias prod
go run ./cmd/one auth login --base-url http://localhost:8081 --token "$TEST_JWT" --alias test
go run ./cmd/one org use test
go run ./cmd/one org validate -dir .customer-sandbox/one-acme-rollout
go run ./cmd/one org deploy  -dir .customer-sandbox/one-acme-rollout --suite CreateProjectFromAccount
go run ./cmd/one org use prod
# same SHA
go run ./cmd/one org validate -dir .customer-sandbox/one-acme-rollout
go run ./cmd/one org deploy  -dir .customer-sandbox/one-acme-rollout --suite CreateProjectFromAccount
```

Headless API/CLI/MCP slices (no Electron): [scripts/customer-rollout-headless.sh](../scripts/customer-rollout-headless.sh). Pass `SKIP_COMPOSE=1` when prod `:8080` and test `:8081` are already up.

## Explicit non-goals

- New Control IDE chrome (frozen list in [agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md))
- Client Experience / end-user CRM ([BP-040](../backlog/BP-040-client-experience-oss-kits.md))
- Path A as the first venue
- Automated GUI E2E in product CI
- Checking customer YAML into the product tree

## Related

- [customer-rollout-gap-log.md](./customer-rollout-gap-log.md)
- [customer-developer-workflow.md](./customer-developer-workflow.md)
- [ci-customer-tests.md](./ci-customer-tests.md)
- [install-ide-connect-build-plan.md](./architecture/install-ide-connect-build-plan.md)
