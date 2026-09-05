# Agent playbook: customer-install simulation campaign

How to **execute** (not invent) the second SI campaign: [customer-install-simulation-test-run.md](../customer-install-simulation-test-run.md).

This is a **test-run playbook**. Executor agents score the product; they do **not** “fix as they go” unless a later prompt is explicitly a `Fixes #N` implementation task.

**Domain:** mixed. Lab packaging lives in `deploy/` + `scripts/` + `docs/`. Findings become GitHub `[campaign S-…]` issues, not new `backlog/BP-*.md` files.

## When to use

| Task | This playbook? |
|---|---|
| Run the three-env customer simulation (dev/test/prod, packages, automations at scale, IDE UX) | **Yes** |
| First two-install campaign (cards A–F) | No — [customer-rollout-test-run.md](../customer-rollout-test-run.md) |
| Implement a filed campaign defect | No — open the GitHub issue **Fix-it** section + the named domain playbook |
| Add Electron chrome, Path A as a new product, or customer YAML to `internal/seed` | **Never** |

## Read first (thin slice)

1. Root [`AGENTS.md`](../../AGENTS.md) constraints (product ≠ customer; no new BP from a campaign beat).
2. [customer-install-simulation-test-run.md](../customer-install-simulation-test-run.md) — cards S-A–S-E.
3. [customer-rollout-gap-log.md](../customer-rollout-gap-log.md) — Campaign 2 tables + open `#28` / `#29`.
4. Only the **allowed operator docs** listed in the runbook. Opening Go to finish a card is itself a docs gap (`docs-drift`).

## Plane fence (executors)

| May do | Must not |
|---|---|
| Bring up [docker-compose.dev-test-prod.yml](../../deploy/docker-compose.dev-test-prod.yml) or the native three-DB fallback | Edit `internal/`, `cmd/`, `migrations/` to “make the lab pass” |
| Write fixtures under gitignored `.customer-sandbox/one-acme-sim` via `scripts/customer-install-sim-generate.sh` | Commit that tree into this monorepo |
| Append **Campaign 2** gap-log rows; file `[campaign S-…]` issues | Open a new `backlog/BP-*.md` |
| Launch Control IDE with isolated `--user-data-dir` | Unfreeze chrome; file “please add Operate CRM” |
| Comment on existing #28 / #29 | Re-file those defects |

If a card is blocked by a product bug, **record + file**, then continue the next card. Do not stop the campaign to implement.

## Capture (mandatory)

Same fields as campaign 1. Beat ids are stable (`S-A-COMPOSE`, …). After each beat: one gap-log row; issue only when the tracking table says so.

Helper: `scripts/file-campaign-finding.sh S-C-WORKER-FANOUT "short title" ./body.md`

## Spawn layout (recommended)

Launch **one executor per card**, in order. S-B and S-C share the same sandbox tree; S-D needs S-C’s Git SHA; S-E can overlap S-B/S-C if a display exists, but env-switch honesty is easier after packages exist.

```text
S-A setup  →  S-B packages+object  →  S-C automations  →  S-D ship SHA  →  S-E IDE UX
                 └── generate.sh
```

An IDE-only agent (`control-ide` expertise) should still **not** edit `tools/control-ide` during the run. Implementation of honesty bugs is a follow-on with [agent-control-ide.md](./agent-control-ide.md) + [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md).

## Lab facts (do not rediscover)

| Fact | Value |
|---|---|
| Compose | `docker compose -f deploy/docker-compose.dev-test-prod.yml up --build -d` |
| Ports | prod `:8080`, test `:8081`, dev `:8082` |
| `CUSTOMER_ID` | `acme-sim` |
| Sandbox | `.customer-sandbox/one-acme-sim` |
| Packages (each install) | `catalog` → `sales`; `project_service`; `lead_marketing` |
| Suite | `SiteVisitFromOpportunity` |
| Loopback peer `baseUrl` | Omit (SSRF). Class `by-design` if 403 |
| Default Compose | Single `INSTALL_ROLE=dev` — not this lab |

## After the run (implementers)

Open registry rows only. Stay in the issue’s packages. PR `Fixes #N`. Mark the gap-log beat `closed`. Do not spawn a BP.

---

## Paste-ready executor prompts

Copy **one** prompt per agent. Do not merge S-E into a headless agent. Do not start extra cards “because there was time” without recording them.

### Prompt 0 — campaign coordinator (optional)

```text
You are the coordinator for Majesta One campaign 2 (customer-install simulation).

Do NOT start Compose, generate fixtures, or file issues yourself unless a card agent is blocked on docs.

Read:
- docs/customer-install-simulation-test-run.md
- docs/architecture/customer-install-simulation-playbook.md
- docs/customer-rollout-gap-log.md (Campaign 2 section + open #28 #29)

Your job:
1. Confirm the five executor prompts (S-A … S-E) are the work order.
2. If you must spawn subagents, give each exactly one card, the runbook path, and the plane fence (no product code edits; no new BPs).
3. After card agents finish, sanity-check Campaign 2 run-results rows exist for every beat they claim to have run.

Do not implement product fixes. Do not run the lab yourself if card agents are already assigned.
```

### Prompt S-A — three-env setup

```text
Execute Majesta One campaign 2 card S-A only (simulate dev / test / prod). Do not start S-B–S-E.

Read first:
- docs/customer-install-simulation-test-run.md (S-A + lab topology)
- docs/self-host.md (Path B)
- docs/multi-env-deploy.md
- docs/customer-rollout-gap-log.md (Campaign 2 tables; retest #28)

Do:
1. Bring up deploy/docker-compose.dev-test-prod.yml (or the native three-DB fallback if Docker is missing). Do not run it alongside docker-compose.yml or docker-compose.multi-env.yml.
2. Wait /readyz on :8080 :8081 :8082.
3. Claim each install with the tokens in the runbook (distinct emails). Smoke /version and /client/v1/me. Pin One-API-Revision.
4. Register peers both ways without loopback baseUrl.
5. one auth login aliases prod/test/dev (ONE_CREDENTIAL_STORE=file if no keychain). org list / org use dev.
6. If API+worker race on first migrate, comment on GitHub issue #28 — do not file a duplicate.

Record: one gap-log row per beat S-A-COMPOSE, S-A-CLAIM-*, S-A-HEALTH, S-A-MIGRATE-RACE, S-A-PEERS, S-A-CLI-ALIASES. File [campaign S-…] issues only for new fail / actionable pass-with-workaround. Class by-design for loopback peer 403.

Must not: edit cmd/, internal/, migrations/, tools/control-ide; open a new BP; commit .customer-sandbox/.

When finished, commit only gap-log / issue-body docs if you added rows, then stop.
```

### Prompt S-B — packages + custom object

```text
Execute Majesta One campaign 2 card S-B only (packages enabled and linked to SiteVisit__c). Assume S-A JWTs/aliases exist; if the lab is down, run S-A first.

Read first:
- docs/customer-install-simulation-test-run.md (S-B)
- docs/modules/README.md
- docs/customer-repo.md
- docs/customer-rollout-gap-log.md (Campaign 2)

Do:
1. Run scripts/customer-install-sim-generate.sh (writes .customer-sandbox/one-acme-sim). git init that tree only.
2. Negative: POST /metadata/v1/packages/sales/enable on dev before catalog — expect 409. Record the message.
3. Enable catalog, sales, project_service, lead_marketing on ALL three installs (idempotent 200). Confirm describe lists Opportunity, Project, Lead.
4. Do not org-deploy yet if you want a clean “lookup without package” beat: on a sidecar thought, or by briefly using an install before enable, prove SiteVisit__c → Opportunity fails until sales is on. If the generator already ran and you enabled first, still record whether validate would have failed — prefer actually seeing the error once.
5. After packs are on, one org validate then deploy to --alias dev from the sandbox (suite may wait for S-C; objectExists for Opportunity/Project/Lead should pass).
6. Prove custom fields on managed objects (Account.LastSiteVisitId__c, Opportunity.SiteVisitCount__c, Project.EngagementFlag__c) describe as ownership=custom.
7. AuthZ: non-admin without PS grants cannot CRUD Opportunity; claim admin can.

Record beats: S-B-PKG-DEP, S-B-PKG-ENABLE-ALL, S-B-OBJ-SITEVISIT, S-B-FIELD-MANAGED-EXT, S-B-LOOKUP-FAIL-WITHOUT-PKG, S-B-AUTHZ-STUBS.

Must not: put customer YAML in internal/seed or migrations/; enable packs only on one install and call that “multi-env”; open a new BP.

Ship of record is CLI org validate/deploy, not Control IDE (that is S-E).
```

### Prompt S-C — automations at scale

```text
Execute Majesta One campaign 2 card S-C only (guest TypeScript automations at scale). Lab + packages from S-A/S-B must be up. Do not skip Band 2.

Read first:
- docs/customer-install-simulation-test-run.md (S-C)
- docs/automation-sdk.md
- docs/customer-rollout-gap-log.md (retest #29)

Do:
1. Ensure scripts/customer-install-sim-generate.sh has run (9 named automations + STUB_COUNT scale stubs, default 48).
2. Import-ban negative: copy _negative/forbidden_lodash_import.ts into src/automations/ with a matching YAML, org validate — must fail. Revert.
3. one org validate then one org deploy --alias dev --suite SiteVisitFromOpportunity. Time both. If CLI truncates suite JSON, comment on #29 (do not duplicate).
4. Live: create Opportunity on dev; wait for worker; query SiteVisit__c. Create Project; confirm TimeEntry fan-out (3 rows). Update a Lead to Status=Qualified; confirm lead.convert (sales must be enabled for createOpportunity).
5. Sync: attempt SiteVisit__c create without OpportunityId — expect rollback/error from validation and/or Reject_Missing_Opportunity.
6. Band 2: POST ~20 ScalePing__c rows on dev. Record job completion, failures, and whether /client/v1/me still works during the burst. Queued jobs under BP-033 = known-remainder unless the API wedges (product-bug).

Record beats: S-C-NAMED, S-C-IMPORT-BAN, S-C-STUBS, S-C-PACK-TIME, S-C-WORKER-FANOUT, S-C-SUITE, S-C-CLI-TRUNC.

Must not: add npm imports to “make TS nicer”; edit internal/automation; open BP-033 from a queue; commit the sandbox.

Deno must be in API (suites) and worker (async). Product images set DENO_PATH; native go run needs it in both shells.
```

### Prompt S-D — same SHA to test and prod

```text
Execute Majesta One campaign 2 card S-D only (repo → org on test and prod from the same Git SHA). Dev deploy from S-C must already be green.

Read first:
- docs/customer-install-simulation-test-run.md (S-D)
- docs/customer-developer-workflow.md
- docs/multi-env-deploy.md

Do:
1. Confirm a clean git commit in .customer-sandbox/one-acme-sim. Note the SHA.
2. Enable the same package set on test and prod if S-B did not (catalog, sales, project_service, lead_marketing).
3. one org use test; validate; deploy --suite SiteVisitFromOpportunity. Same SHA.
4. one org use prod; validate; deploy. GET /metadata/v1/objects/SiteVisit__c on prod. Query SiteVisit__c — business rows from dev must not appear.
5. Package drift: on one sibling, skip or disable a required pack and show deploy/describe failure; re-enable after. Do not call peer promote (removed APIs).
6. Anti-patterns: packing .one/baseline; mutating managed Account fields via Metadata.

Record beats: S-D-DEPLOY-TEST, S-D-DEPLOY-PROD, S-D-NO-ROW-COPY, S-D-PKG-DRIFT.

Must not: POST /deploy/v1/bundles/:id/push; copy rows with a homemade SQL dump as “promote”; edit product Go.
```

### Prompt S-E — Control IDE full-surface UX

```text
Execute Majesta One campaign 2 card S-E only (Control IDE UI/UX of every shipped function). Desktop/display required. If DISPLAY is missing, record the whole card blocked-no-display and write the launch recipe — do not invent a headless Electron pass.

Read first:
- docs/customer-install-simulation-test-run.md (S-E table)
- docs/customer-ide-ux.md
- docs/local-development-mac.md
- docs/architecture/agent-control-ide.md (what ships; freeze vs honesty)
- backlog/BP-066-ide-demo-client-fidelity.md (lying greens)

Do:
1. cd tools/control-ide && npm ci && npm run build
2. Launch with isolated userData: npx electron --user-data-dir="$HOME/.local/share/one-control-ide-sim-a" .
   Optional second process: ...-sim-b
3. Walk EVERY row in the S-E table: Sign in, Environments (all three installs), Operate (graph, List View, find, ToolSpec), Build (Objects, Packages, Automations, Agents, Tools, Repo, Deploy, Query/Monitor/Explorer), Govern, Settings, theme + 2-slice workspace + agent dock.
4. Honesty: Deploy must not show Passed on HTTP 200 alone; Operate chat must not auto-green a tool loop that did not run; Monitor empty-state if ExecutionRun is unseeded is known-remainder / frozen-chrome-honesty, not a new BP.
5. Packages panel: enable something on dev only; switch env to test; confirm test did not magically enable.
6. Dual-write: Object Manager changes must land in the customer sandbox metadata/, never .one/baseline.

Record one beat per S-E-* id in the runbook. File product-bug / frozen-chrome-honesty issues for real lies or broken panels. Do NOT file requests for license UX, update CDN, Operate-as-CRM, BoardHandoff, in-IDE agent host, or peer promote GUI.

Must not: edit tools/control-ide to “fix UX while testing”; edit Go; unfreeze chrome.

If you cannot complete a panel, outcome=fail or blocked-no-display with the blocker in the one-line actual.
```

---

## Related

- [customer-install-simulation-test-run.md](../customer-install-simulation-test-run.md)
- [customer-rollout-test-run.md](../customer-rollout-test-run.md)
- [agent-routing.md](./agent-routing.md)
- [agent-control-ide.md](./agent-control-ide.md)
- [agent-deploy.md](./agent-deploy.md)
- [customer-automations-build.md](./customer-automations-build.md)
