# Customer rollout gap log

Companion to [customer-rollout-test-run.md](./customer-rollout-test-run.md). One row per scenario beat (or per distinct finding). Do not paste customer data, secrets, or exploit detail.

## How to score

| Field | Values |
|---|---|
| Scenario | `A` … `F` plus optional suffix (`A-claim`, `C-peers`, …) |
| Outcome | `pass` · `pass-with-workaround` · `fail` · `blocked-no-display` · `not-run` |
| DX | `1` (unusable) … `5` (copy-paste green) for docs findability, error quality, recovery, time-to-green |
| Class | `docs-drift` · `missing-lab-packaging` · `product-bug` · `authz-confusion` · `frozen-chrome-honesty` · `known-remainder` · `by-design` |

Link a backlog item when the class is honesty or a known remainder: [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md), [BP-048](../backlog/BP-048-one-cli.md), [BP-031](../backlog/BP-031-customer-repo-init-sync.md), [BP-065](../backlog/BP-065-ide-backend-coupling.md), [BP-029](../backlog/BP-029-app-platform-install.md).

Row template:

```text
### G-ID — short title
- Scenario:
- Expected path (doc):
- Actual steps / time:
- Outcome:
- DX (1–5):
- Class:
- BP / ADR:
- Notes:
```

---

## Lab used (2026-09-05)

Headless campaign on a cloud agent VM **without Docker**. Native fallback: Postgres 16 (`one_prod` / `one_test` on `127.0.0.1:5432`), four `go run` processes (API+worker × prod/test), Deno 2.9.3 at `DENO_PATH`. Customer fixtures in gitignored `.customer-sandbox/one-acme-rollout` (not in this repo). Control IDE / Electron was **not** launched (`tools/control-ide` had no `npm ci`; display present but scenario B marked desktop-required). Compose overlay was not executed here.

---

## Pre-seeded (confirm on the run)

These were visible in the tree before the campaign. Confirm, then set Outcome.

### G-COMPOSE-SINGLE — default Compose is one `dev` install

- Scenario: A
- Expected path (doc): [self-host.md](./self-host.md) Path B (“start with Prod”)
- Actual steps / time: Overlay file exists; default [deploy/docker-compose.yml](../deploy/docker-compose.yml) is still one `INSTALL_ROLE=dev` stack. This VM had no Docker, so the overlay was not `up`’d; native two-DB lab used instead.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `missing-lab-packaging` / `docs-drift`
- BP / ADR: [multi-env-deploy.md](./multi-env-deploy.md)
- Notes: Campaign overlay [deploy/docker-compose.multi-env.yml](../deploy/docker-compose.multi-env.yml) is now linked from Path B docs. Everyday Compose remains a single install by design — testers who copy only `docker compose -f deploy/docker-compose.yml` never get a test sibling.

### G-ENV-EXAMPLE — root `.env.example` missing

- Scenario: A
- Expected path (doc): [README.md](../README.md) / [local-development-mac.md](./local-development-mac.md) `cp .env.example .env`
- Actual steps / time: Confirmed absent at campaign start (`.gitignore` keeps `!.env.example`). Native lab exported env by hand from Compose names.
- Outcome: `fail`
- DX (1–5): 2
- Class: `docs-drift`
- BP / ADR: —
- Notes: Restored a root `.env.example` in the same change set as this log so the next native `make api` loop can copy-paste.

### G-PEER-LOCAL — peer `baseUrl` rejects loopback

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md) `POST /deploy/v1/peers` with `baseUrl`
- Actual steps / time: `POST /deploy/v1/peers` with `baseUrl: http://localhost:8081` → **403** `FORBIDDEN` / `Invalid baseUrl: target host "localhost" is blocked`. Same body without `baseUrl` → **201** both ways.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `by-design` (SSRF) with residual `docs-drift` if testers skip the loopback note
- BP / ADR: —
- Notes: Workaround: omit `baseUrl`; IDE Add environment. RFC1918 URLs remain allowed. Operator docs now say localhost is invalid.

### G-DOCS-PROMOTE — customizations doc still says promote / CodeCommit

- Scenario: E
- Expected path (doc): [customer-customizations.md](./customer-customizations.md) vs [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time: Golden rules still said “Promote with Deploy” and “auto-provisioned CodeCommit” while the modern DX is repo→org on any HTTPS Git.
- Outcome: `fail`
- DX (1–5): 2
- Class: `docs-drift`
- BP / ADR: [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md) mitigated
- Notes: Golden rules + surface table rewritten in this change set to match [customer-developer-workflow.md](./customer-developer-workflow.md).

### G-FEATURE-FLAGS — omit vs empty vs MCP dark

- Scenario: F
- Expected path (doc): [customer-connect.md](./customer-connect.md) (production often omits `agents`)
- Actual steps / time: Lab set `FEATURE_FLAGS=agents`. MCP catalog served `upsert_object` / `org_*`. Did not A/B empty vs omitted vs `FEATURE_FLAGS=core`.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift`
- BP / ADR: [ADR-010](./adr/010-customer-agentic-platform.md)
- Notes: Empty `FEATURE_FLAGS` **enables** agents (`Config.AgentsEnabled`). Marketplace must set a non-empty list that does **not** include `agents` to keep MCP dark. [customer-connect.md](./customer-connect.md) updated to say that; “omit the flag” is the wrong instruction.

### G-IDE-USERDATA — no dual-IDE recipe in Mac runbook (pre-change)

- Scenario: B
- Expected path (doc): [local-development-mac.md](./local-development-mac.md)
- Actual steps / time: Recipe is now in the Mac runbook (`npx electron --user-data-dir=… .` **before** `.`). Not executed (no `npm ci` / Electron in this lab).
- Outcome: `blocked-no-display`
- DX (1–5): —
- Class: `missing-lab-packaging`
- BP / ADR: —
- Notes: Procedure documented; session isolation unproven on this run.

### G-NO-SCRATCH-ORG — second env is a full install

- Scenario: C
- Expected path (doc): testers looking for `one org scratch`
- Actual steps / time: `one org list` / `org use test` worked against a second full install. No `scratch` subcommand.
- Outcome: `pass`
- DX (1–5): 4 (once you accept a second install)
- Class: `known-remainder`
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes: Scratch orgs deferred. Two Compose projects / the multi-env file / native two-DB lab is the stand-in.

### G-CROSS-INSTALL-SSO — one login does not unlock peers

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md) / install connect plan Phase 3
- Actual steps / time: Distinct SystemAdmin emails on prod vs test (`admin-prod@…` / `admin-test@…`). Distinct JWTs. `one auth login --alias prod` and `--alias test` both required.
- Outcome: `pass`
- DX (1–5): 4
- Class: `by-design` / `authz-confusion` if testers expect SSO across envs
- BP / ADR: [BP-037](../backlog/BP-037-install-claim-customer-sso.md)
- Notes: Each install is its own JWT issuer. Sign in once per URL.

### G-IDE-DEPLOY-GREEN — Deploy test honesty

- Scenario: B / E
- Expected path (doc): IDE Build → Deploy vs `one org deploy --suite`
- Actual steps / time: Electron not launched.
- Outcome: `blocked-no-display`
- DX (1–5): —
- Class: `frozen-chrome-honesty`
- BP / ADR: [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md) WS-0
- Notes: CLI suite status is the honesty baseline for this run. GUI lying-green still untested.

### G-HOSTED-LOOP-SHIP — Operate chat cannot org_deploy

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md) hosted loop v1 subset
- Actual steps / time: `POST /client/v1/agents/runs` with ShipGuide + “deploy my Project__c” returned `status: awaiting_approval` and did **not** call `org_deploy`. MCP catalog on the same install lists `org_deploy`.
- Outcome: `pass`
- DX (1–5): 4
- Class: `by-design` (API). Electron pretence not checked.
- BP / ADR: [BP-006](../backlog/BP-006-agent-guardrails.md) / [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)
- Notes: Metadata upserts and `org_*` stay on MCP / family HTTP / `one`. IDE chat honesty is still BP-066.

### G-DENO-PATH — suite / worker without Deno

- Scenario: D / E
- Expected path (doc): [local-development-mac.md](./local-development-mac.md) Deno 2.9.3
- Actual steps / time: First `--suite CreateAccountFromContact` failed with `deno binary not found (set DENO_PATH or install Deno 2.9.3)` because the **API** process lacked Deno. After `DENO_PATH` on the API, suites passed. Live Account→Project still failed until the **worker** also had `DENO_PATH`. Product images already set `DENO_PATH=/usr/local/bin/deno`.
- Outcome: `pass-with-workaround`
- DX (1–5): 4 (message is clear; split across processes is not)
- Class: `docs-drift` (now documented)
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes: Suites run Deno in the API; `execution: async` automations run Deno in the worker. See G-DENO-API-VS-WORKER.

---

## Execution results

Headless slices (API, claim, CLI, MCP, second install) ran without Electron. Scenario B needs a display.

### G-EXEC-A-HEALTH — prod and test /healthz /readyz

- Scenario: A
- Expected path (doc): this runbook “Bring the lab up”
- Actual steps / time: First boot raced: API+worker both call `EnsureKernel`; migration `0038_hard_delete_no_default` is not idempotent (`DELETE FROM records WHERE deleted_at IS NOT NULL` after the sibling already dropped the column). Retry after 62 migrations applied succeeded. `/readyz` `{"status":"ready"}` on `:8080` and `:8081`.
- Outcome: `pass-with-workaround`
- DX (1–5): 2
- Class: `product-bug`
- BP / ADR: [BP-002](../backlog/BP-002-dedicated-install-fleet-ops.md) (newly evidenced)
- Notes: Workaround: wait for API `/readyz` before starting the worker. See G-MIGRATE-RACE.

### G-EXEC-A-CLAIM — install claim on prod and test

- Scenario: A
- Expected path (doc): [customer-connect.md](./customer-connect.md) / [self-host.md](./self-host.md)
- Actual steps / time: `POST /auth/v1/install/claim` on both installs with distinct emails. Happy path (not bootstrap key).
- Outcome: `pass`
- DX (1–5): 5
- Class: —
- BP / ADR: [BP-037](../backlog/BP-037-install-claim-customer-sso.md)
- Notes: Claim tokens from the overlay (`rollout-prod-claim-token-change-me` / `rollout-test-claim-token-change-me`).

### G-EXEC-A-ME — `/client/v1/me` + revision pin

- Scenario: A
- Expected path (doc): [builder-connect.md](./builder-connect.md) pin `One-API-Revision`
- Actual steps / time: `GET /version` advertised revision `1`. `GET /client/v1/me` with the claim JWT succeeded on both installs.
- Outcome: `pass`
- DX (1–5): 5
- Class: —
- BP / ADR: [BP-025](../backlog/BP-025-ide-api-version-compatibility.md)
- Notes: Pin header required on family routes as documented.

### G-EXEC-B-IDE — two Electron userData sessions

- Scenario: B
- Expected path (doc): [local-development-mac.md](./local-development-mac.md)
- Actual steps / time: Not run. `DISPLAY=:1` existed; Control IDE tree was not installed (`npm ci` not executed).
- Outcome: `blocked-no-display`
- DX (1–5): —
- Class: —
- BP / ADR: [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)
- Notes: Desktop required. Dual `--user-data-dir` remains a documented procedure.

### G-EXEC-C-PEERS — register sibling without loopback baseUrl

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md)
- Actual steps / time: Loopback `baseUrl` 403 (G-PEER-LOCAL). Peers without `baseUrl` 201 both ways.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `by-design`
- BP / ADR: —
- Notes: IDE env switcher with live peer URLs is untested without Electron.

### G-EXEC-C-CLI-ALIAS — `one auth login` prod + test

- Scenario: C
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time: `ONE_CREDENTIAL_STORE=file`; `one auth login --alias prod` / `--alias test`; `one org use test`; `one org list` showed both.
- Outcome: `pass`
- DX (1–5): 5
- Class: —
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes: File store is required when the lab has no OS keychain.

### G-EXEC-D-INIT — `one project init` into `.customer-sandbox`

- Scenario: D
- Expected path (doc): [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time: `one project init -dir .customer-sandbox/one-acme-rollout --customer-id acme-rollout`. Nested git, `customerId: acme-rollout`. Tree remains gitignored.
- Outcome: `pass`
- DX (1–5): 5
- Class: —
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes: Do not commit the sandbox into the product repo.

### G-EXEC-D-TEMPLATE-DEPLOY — sample suite on test

- Scenario: D / E
- Expected path (doc): [deploy/customer-repo-template/README.md](../deploy/customer-repo-template/README.md)
- Actual steps / time: Template `Referral__c` + `CreateAccount_From_Contact` deployed to test. First `--suite` failed without API Deno; second pass green after `DENO_PATH`.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: —
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes: Product worker **image** already includes Deno; native `go run ./cmd/api` does not.

### G-EXEC-D-PROJECT — Project__c + automation on test

- Scenario: D
- Expected path (doc): [customer-repo.md](./customer-repo.md)
- Actual steps / time: Added `Project__c` + `CreateProject_From_Account` + suite in the sandbox. First contract step failed: JSONLogic `true` means **invalid** — runbook sample `"!!": {var: Name}` fired when Name was present. Flipped to `"!": {var: Name}`. Suites then passed. Live Account “Post Deno Worker Account” created Project after worker `DENO_PATH` (worker log: `CreateProject_From_Account created Project for Post Deno Worker Account`).
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift` (JSONLogic polarity)
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes: MCP `upsert_object` not used for this delta (hand-edit YAML). Query body field is `object`, not `objectApiName`. Create sobject returns `Id` (capital I).

### G-EXEC-E-PROD — same SHA to prod

- Scenario: E
- Expected path (doc): [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time: Same Git SHA `one org validate` / `org deploy --suite CreateProjectFromAccount` on prod. `GET /metadata/v1/objects/Project__c` returns custom ownership. Business rows did not copy.
- Outcome: `pass`
- DX (1–5): 4
- Class: —
- BP / ADR: [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md)
- Notes: Pack checksums differ per org because the pack includes `sourceInstallId` — same SHA, different checksums (G-PACK-CHECKSUM-PER-ORG). CLI truncates suite JSON (G-CLI-SUITE-TRUNC).

### G-EXEC-F-MCP — `/mcp/tools` + initialize on test

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time: `GET /mcp/tools` listed `upsert_object`, `org_validate`, `org_deploy`, `pack`, `invoke_skill`, …. `initialize` OK (`server.name=one`, protocol `2025-03-26`). `list_objects_metadata` showed Account/Contact/User + `Project__c` + `Referral__c`.
- Outcome: `pass`
- DX (1–5): 5
- Class: —
- BP / ADR: [ADR-030](./adr/030-install-agent-runtime.md)
- Notes: Streamable HTTP JSON from curl worked; stdio `tools/one-mcp` not needed.

### G-EXEC-F-HOSTED — agents/runs cannot org_deploy

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time: See G-HOSTED-LOOP-SHIP. `awaiting_approval`; no metadata upsert / `org_deploy`.
- Outcome: `pass`
- DX (1–5): 4
- Class: `by-design`
- BP / ADR: [hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md)
- Notes: Testers who try to ship from Operate chat should hit this wall. IDE pretence not checked.

---

## New evidence from this run

### G-MIGRATE-RACE — API + worker concurrent EnsureKernel

- Scenario: A
- Expected path (doc): [local-development-mac.md](./local-development-mac.md) / [agent-worker.md](./architecture/agent-worker.md) (“migrations ride API boot”)
- Actual steps / time: Both `cmd/api` and `cmd/worker` call `pool.EnsureKernel`. First native boot failed on non-idempotent `0038_hard_delete_no_default`.
- Outcome: `fail`
- DX (1–5): 2
- Class: `product-bug`
- BP / ADR: [BP-002](../backlog/BP-002-dedicated-install-fleet-ops.md)
- Notes: Compose `depends_on: service_started` does not wait for `/readyz`, so the overlay can hit the same race. Workaround: start API, wait `/readyz`, then worker.

### G-JSONLOGIC-POLARITY — validation fires when expression is true

- Scenario: D
- Expected path (doc): [customer-repo.md](./customer-repo.md) validation-rules row
- Actual steps / time: `EvaluateValidationRules` treats JSONLogic `true` as invalid (Salesforce-style). Allowed docs did not say that. Campaign YAML `"!!": {var: Name}` rejected named Projects.
- Outcome: `fail`
- DX (1–5): 2
- Class: `docs-drift`
- BP / ADR: [ADR-002](./adr/002-hybrid-metadata-storage.md)
- Notes: Runbook sample and [customer-repo.md](./customer-repo.md) now state polarity. Name-required is `"!": { var: Name }`.

### G-CLI-SUITE-TRUNC — `one org deploy --suite` truncates JSON

- Scenario: E
- Expected path (doc): [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time: CLI printed truncated suite JSON (`objectAp…`). Full report via `GET /deploy/v1/tests/runs/:id`.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `known-remainder`
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes: Does not hide pass/fail of the suite in this run; painful when diagnosing a red step.

### G-QUERY-FIELD — query body uses `object`

- Scenario: D
- Expected path (doc): testers guessing from Metadata `objectApiName`
- Actual steps / time: `POST /client/v1/query` with `objectApiName` → `VALIDATION_ERROR` `object is required`. Correct field is `object`. Create sobject returns `Id`.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift`
- BP / ADR: —
- Notes: Runbook prove-runtime curl now uses `{"object":"Project__c"}`. Family contract lives in Client query docs / `@one/client`.

### G-PACK-CHECKSUM-PER-ORG — same Git SHA, different pack checksum

- Scenario: E
- Expected path (doc): testers comparing pack checksums across orgs
- Actual steps / time: Pack includes `sourceInstallId`, so checksums differ even when Git SHA matches.
- Outcome: `pass`
- DX (1–5): 4
- Class: `by-design`
- BP / ADR: [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md)
- Notes: Compare Git SHA / object describe, not pack checksum, across installs.

### G-DENO-API-VS-WORKER — suites vs async automations

- Scenario: D
- Expected path (doc): “worker image includes Deno” implying live automations work after a green `--suite`
- Actual steps / time: Green `--suite` on the API did **not** imply the worker could run `automation.run`. Second Account after worker restart with `DENO_PATH` created `Project__c`.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift`
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes: Native labs must export `DENO_PATH` (or Deno on `PATH`) in **both** API and worker shells.

### G-NO-DOCKER — Compose helper unusable without Docker

- Scenario: A
- Expected path (doc): [scripts/customer-rollout-headless.sh](../scripts/customer-rollout-headless.sh)
- Actual steps / time: Script requires `docker`. This VM had none. Native Postgres + `go run` used instead.
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `missing-lab-packaging`
- BP / ADR: —
- Notes: Script now supports `SKIP_COMPOSE=1` when APIs are already up. Runbook documents the native two-DB fallback.

---

## Summary (after the run)

| Count | Outcome |
|---|---|
| 11 | pass |
| 12 | pass-with-workaround |
| 4 | fail |
| 3 | blocked-no-display |
| 0 | not-run |

Highest-severity new evidence (update [backlog/README.md](../backlog/README.md) only when a BP is closed or newly evidenced):

- **G-MIGRATE-RACE** — concurrent `EnsureKernel` from API+worker; `0038` not idempotent. Noted on [BP-002](../backlog/BP-002-dedicated-install-fleet-ops.md) Remaining (status stays Partially mitigated).
- **G-JSONLOGIC-POLARITY** / **G-DOCS-PROMOTE** / **G-ENV-EXAMPLE** — docs drift confirmed; patched in this change set (not a BP close).
- **G-CLI-SUITE-TRUNC** / no `one org scratch` — remainder notes on [BP-048](../backlog/BP-048-one-cli.md).
- **BP-066 WS-0** Electron honesty **not** exercised; hosted `/agents/runs` contrast confirmed at HTTP. Status stays Open.
- **BP-048** scratch orgs remain deferred (G-NO-SCRATCH-ORG).
