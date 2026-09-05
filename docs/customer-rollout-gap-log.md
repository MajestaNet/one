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

## Pre-seeded (confirm on the run)

These were visible in the tree before the campaign. Confirm, then set Outcome.

### G-COMPOSE-SINGLE — default Compose is one `dev` install

- Scenario: A
- Expected path (doc): [self-host.md](./self-host.md) Path B (“start with Prod”)
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `missing-lab-packaging` / `docs-drift`
- BP / ADR: [multi-env-deploy.md](./multi-env-deploy.md)
- Notes: [deploy/docker-compose.yml](../deploy/docker-compose.yml) is a single stack with `INSTALL_ROLE: dev`. Campaign overlay: [deploy/docker-compose.multi-env.yml](../deploy/docker-compose.multi-env.yml). Confirm whether Path B docs mention the overlay after this change.

### G-ENV-EXAMPLE — root `.env.example` missing

- Scenario: A
- Expected path (doc): [README.md](../README.md) / [local-development-mac.md](./local-development-mac.md) `cp .env.example .env`
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `docs-drift`
- BP / ADR: —
- Notes: `.gitignore` keeps `!.env.example`, but no root `.env.example` is tracked. Compose lab does not need it; `make api` local loop does.

### G-PEER-LOCAL — peer `baseUrl` rejects loopback

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md) `POST /deploy/v1/peers` with `baseUrl`
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `by-design` (SSRF) with `docs-drift` if the operator doc does not say localhost is invalid
- BP / ADR: —
- Notes: Expect fail on `http://localhost` / `127.0.0.1`. Workaround: omit `baseUrl`; IDE Add environment. RFC1918 URLs remain allowed.

### G-DOCS-PROMOTE — customizations doc still says promote / CodeCommit

- Scenario: E
- Expected path (doc): [customer-customizations.md](./customer-customizations.md) vs [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `docs-drift`
- BP / ADR: [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md) mitigated
- Notes: Golden rules still say “Promote with Deploy” and “auto-provisioned CodeCommit”. Ship of record is repo → org on any HTTPS Git.

### G-FEATURE-FLAGS — omit vs empty vs MCP dark

- Scenario: F
- Expected path (doc): [customer-connect.md](./customer-connect.md) (production often omits `agents`)
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `docs-drift`
- BP / ADR: [ADR-010](./adr/010-customer-agentic-platform.md)
- Notes: Empty `FEATURE_FLAGS` **enables** agents (`Config.AgentsEnabled`). Marketplace must set a non-empty list that does **not** include `agents` to keep MCP dark. This Compose lab sets `FEATURE_FLAGS=agents` explicitly.

### G-IDE-USERDATA — no dual-IDE recipe in Mac runbook (pre-change)

- Scenario: B
- Expected path (doc): [local-development-mac.md](./local-development-mac.md)
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `missing-lab-packaging`
- BP / ADR: —
- Notes: Confirm whether the dual-process `npx electron --user-data-dir=… .` snippet in the Mac runbook / this campaign is discoverable and actually isolates sessions.

### G-NO-SCRATCH-ORG — second env is a full install

- Scenario: C
- Expected path (doc): testers looking for `one org scratch`
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `known-remainder`
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes: Scratch orgs deferred. Two Compose projects / the multi-env file is the lab stand-in.

### G-CROSS-INSTALL-SSO — one login does not unlock peers

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md) / install connect plan Phase 3
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `by-design` / `authz-confusion` if testers expect SSO across envs
- BP / ADR: [BP-037](../backlog/BP-037-install-claim-customer-sso.md)
- Notes: Each install is its own JWT issuer. Sign in once per URL.

### G-IDE-DEPLOY-GREEN — Deploy test honesty

- Scenario: B / E
- Expected path (doc): IDE Build → Deploy vs `one org deploy --suite`
- Actual steps / time: (fill)
- Outcome: `not-run` (needs display)
- DX (1–5):
- Class: `frozen-chrome-honesty`
- BP / ADR: [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md) WS-0
- Notes: Fail if the GUI marks customer tests Passed on HTTP 200 without reading the report.

### G-HOSTED-LOOP-SHIP — Operate chat cannot org_deploy

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md) hosted loop v1 subset
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `by-design` or `frozen-chrome-honesty` if the IDE pretends it shipped
- BP / ADR: [BP-006](../backlog/BP-006-agent-guardrails.md) / [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md)
- Notes: Metadata upserts and `org_*` stay on MCP / family HTTP / `one`.

### G-DENO-PATH — suite / worker without Deno

- Scenario: D / E
- Expected path (doc): [local-development-mac.md](./local-development-mac.md) Deno 2.9.3
- Actual steps / time: (fill)
- Outcome: `not-run`
- DX (1–5):
- Class: `docs-drift` if the CLI/worker error is opaque
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes: Product worker image includes Deno. Host `one org deploy --suite` needs Deno on the operator `PATH` when the CLI invokes the unit harness locally. Confirm the actual message.

---

## Execution results

Fill during the campaign. Headless slices (API, claim, CLI, MCP, second install) can run without Electron. Scenario B needs a display.

### G-EXEC-A-HEALTH — prod and test /healthz /readyz

- Scenario: A
- Expected path (doc): this runbook “Bring the lab up”
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: —
- Notes:

### G-EXEC-A-CLAIM — install claim on prod and test

- Scenario: A
- Expected path (doc): [customer-connect.md](./customer-connect.md) / [self-host.md](./self-host.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [BP-037](../backlog/BP-037-install-claim-customer-sso.md)
- Notes:

### G-EXEC-A-ME — `/client/v1/me` + revision pin

- Scenario: A
- Expected path (doc): [builder-connect.md](./builder-connect.md) pin `One-API-Revision`
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [BP-025](../backlog/BP-025-ide-api-version-compatibility.md)
- Notes:

### G-EXEC-B-IDE — two Electron userData sessions

- Scenario: B
- Expected path (doc): [local-development-mac.md](./local-development-mac.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: —
- Notes: Mark `blocked-no-display` when no GUI.

### G-EXEC-C-PEERS — register sibling without loopback baseUrl

- Scenario: C
- Expected path (doc): [multi-env-deploy.md](./multi-env-deploy.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: —
- Notes:

### G-EXEC-C-CLI-ALIAS — `one auth login` prod + test

- Scenario: C
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes:

### G-EXEC-D-INIT — `one project init` into `.customer-sandbox`

- Scenario: D
- Expected path (doc): [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [BP-048](../backlog/BP-048-one-cli.md)
- Notes:

### G-EXEC-D-TEMPLATE-DEPLOY — sample suite on test

- Scenario: D / E
- Expected path (doc): [deploy/customer-repo-template/README.md](../deploy/customer-repo-template/README.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes:

### G-EXEC-D-PROJECT — Project__c + automation on test

- Scenario: D
- Expected path (doc): [customer-repo.md](./customer-repo.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [ADR-014](./adr/014-customer-code-automations.md)
- Notes:

### G-EXEC-E-PROD — same SHA to prod

- Scenario: E
- Expected path (doc): [customer-developer-workflow.md](./customer-developer-workflow.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md)
- Notes:

### G-EXEC-F-MCP — `/mcp/tools` + initialize on test

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [ADR-030](./adr/030-install-agent-runtime.md)
- Notes:

### G-EXEC-F-HOSTED — agents/runs cannot org_deploy

- Scenario: F
- Expected path (doc): [builder-connect.md](./builder-connect.md)
- Actual steps / time:
- Outcome: `not-run`
- DX (1–5):
- Class:
- BP / ADR: [hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md)
- Notes:

---

## Summary (after the run)

| Count | Outcome |
|---|---|
| | pass |
| | pass-with-workaround |
| | fail |
| | blocked-no-display |
| | not-run |

Highest-severity new evidence (update [backlog/README.md](../backlog/README.md) only when a BP is closed or newly evidenced):
