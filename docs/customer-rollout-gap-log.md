# Customer rollout gap log

Companion to [customer-rollout-test-run.md](./customer-rollout-test-run.md) (campaign 1, beats `G-…`) and [customer-install-simulation-test-run.md](./customer-install-simulation-test-run.md) (campaign 2, beats `S-…`). Executor playbook: [customer-install-simulation-playbook.md](./architecture/customer-install-simulation-playbook.md).

This file is the **run record** and the **issue registry**. Every campaign uses the same tables. Do not paste secrets, customer data, or exploit detail.

## Tracking (read this first)

| Kind | Where | When to open |
|---|---|---|
| **Confirmed defect** | A **GitHub issue** titled `[campaign G-…]` or `[campaign S-…]` | Outcome is `fail`, or `pass-with-workaround` that is **not** `by-design` / `known-remainder` |
| **Architecture remainder** | [`backlog/BP-*.md`](../backlog/README.md) | Already a foreseeable product-design risk. **Do not** open a new BP from a campaign beat |
| **Vulnerability** | [SECURITY.md](../SECURITY.md) only | Never a public issue or a BP body |

Fix PRs cite **`Fixes #<issue>`**. After merge, set the registry row to `closed` and link the PR. Agents pick work from the registry **Open** rows, not from BP campaign notes.

Template: [.github/ISSUE_TEMPLATE/campaign-finding.md](../.github/ISSUE_TEMPLATE/campaign-finding.md). Helper: [scripts/file-campaign-finding.sh](../scripts/file-campaign-finding.sh).

## Record a beat (same fields every time)

After **each** scenario card (campaign 1 `A`–`F`, campaign 2 `S-A`–`S-E`) and each suffix beat, add **one** row to the matching **Run results** table and, if it needs a fix, **one** row to **Issue registry**.

| Field | Values |
|---|---|
| Beat | `G-…` or `S-…` id (stable). Suffix ok (`G-EXEC-A-CLAIM`, `S-A-CLAIM-DEV`) |
| Card | `A` … `F` or `S-A` … `S-E` |
| Outcome | `pass` · `pass-with-workaround` · `fail` · `blocked-no-display` · `not-run` |
| DX | `1`–`5` or `—` |
| Class | `docs-drift` · `missing-lab-packaging` · `product-bug` · `authz-confusion` · `frozen-chrome-honesty` · `known-remainder` · `by-design` |
| Issue | GitHub `#N` · `fixed-in-#27` · `none` (pass / by-design / remainder / not-run) |

Row template (copy):

```text
| YYYY-MM-DD | G-ID or S-ID | X | outcome | DX | class | none or #N | one-line actual |
```

---

## Issue registry (the work queue)

Open rows are what an agent should implement. Closed rows stay for traceability.

| Beat | Class | GitHub issue | Status | Fix PR |
|---|---|---|---|---|
| G-MIGRATE-RACE | product-bug | [#28](https://github.com/MajestaNet/one/issues/28) | **open** | |
| G-CLI-SUITE-TRUNC | product-bug | [#29](https://github.com/MajestaNet/one/issues/29) | **open** | |
| G-ENV-EXAMPLE | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-DOCS-PROMOTE | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-JSONLOGIC-POLARITY | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-FEATURE-FLAGS | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-DENO-PATH | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-DENO-API-VS-WORKER | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-QUERY-FIELD | docs-drift | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-NO-DOCKER | missing-lab-packaging | none (fixed in campaign PR) | closed | [#27](https://github.com/MajestaNet/one/pull/27) |
| G-PEER-LOCAL | by-design | none | n/a (SSRF; omit `baseUrl`) | [#27](https://github.com/MajestaNet/one/pull/27) docs |
| G-PACK-CHECKSUM-PER-ORG | by-design | none | n/a | |
| G-HOSTED-LOOP-SHIP | by-design | none | n/a (MCP ships; hosted loop does not) | |
| G-NO-SCRATCH-ORG | known-remainder | none | n/a — [BP-048](../backlog/BP-048-one-cli.md) Wave D | |
| G-CROSS-INSTALL-SSO | by-design | none | n/a — [BP-037](../backlog/BP-037-install-claim-customer-sso.md) | |
| G-IDE-DEPLOY-GREEN | frozen-chrome-honesty | none | not-run — [BP-066](../backlog/BP-066-ide-demo-client-fidelity.md) WS-0 | |
| G-IDE-USERDATA | missing-lab-packaging | none | procedure in Mac runbook; Electron not launched | |
| G-COMPOSE-SINGLE | by-design | none | everyday Compose is one `dev` install; overlay is the two-install lab | |

**Agent prompt (open rows):** open the GitHub issue, follow its **Fix-it** section, stay in the named packages, PR `Fixes #N`, then update this table.

---

## Run results

### 2026-09-05 — native two-install (no Docker, no Electron)

Lab: Postgres 16 (`one_prod` / `one_test` on `127.0.0.1:5432`); four `go run` processes; Deno 2.9.3 via `DENO_PATH`. Fixtures in gitignored `.customer-sandbox/one-acme-rollout`. Compose overlay not executed. `DISPLAY` present; Control IDE not installed.

| Date | Beat | Card | Outcome | DX | Class | Issue | Actual (one line) |
|---|---|---|---|---|---|---|---|
| 2026-09-05 | G-EXEC-A-HEALTH | A | pass-with-workaround | 2 | product-bug | [#28](https://github.com/MajestaNet/one/issues/28) | Concurrent `EnsureKernel`; retry after 62 migrations |
| 2026-09-05 | G-MIGRATE-RACE | A | fail | 2 | product-bug | [#28](https://github.com/MajestaNet/one/issues/28) | `0038` not idempotent; API+worker both migrate |
| 2026-09-05 | G-EXEC-A-CLAIM | A | pass | 5 | — | none | `POST /auth/v1/install/claim` both installs |
| 2026-09-05 | G-EXEC-A-ME | A | pass | 5 | — | none | `/version` pin `1`; `/client/v1/me` ok |
| 2026-09-05 | G-ENV-EXAMPLE | A | fail | 2 | docs-drift | fixed-in-#27 | Root `.env.example` was missing |
| 2026-09-05 | G-COMPOSE-SINGLE | A | pass-with-workaround | 3 | by-design | none | Default Compose is one `dev` stack |
| 2026-09-05 | G-NO-DOCKER | A | pass-with-workaround | 3 | missing-lab-packaging | fixed-in-#27 | No Docker; native fallback + `SKIP_COMPOSE=1` |
| 2026-09-05 | G-EXEC-B-IDE | B | blocked-no-display | — | — | none | Electron not launched |
| 2026-09-05 | G-IDE-USERDATA | B | blocked-no-display | — | missing-lab-packaging | none | `--user-data-dir` recipe documented, unproven |
| 2026-09-05 | G-IDE-DEPLOY-GREEN | B | blocked-no-display | — | frozen-chrome-honesty | none | GUI honesty not exercised |
| 2026-09-05 | G-PEER-LOCAL | C | pass-with-workaround | 3 | by-design | none | Loopback `baseUrl` 403; omit `baseUrl` → 201 |
| 2026-09-05 | G-EXEC-C-PEERS | C | pass-with-workaround | 3 | by-design | none | Peers both ways without `baseUrl` |
| 2026-09-05 | G-EXEC-C-CLI-ALIAS | C | pass | 5 | — | none | `one auth login` prod+test; `org use test` |
| 2026-09-05 | G-NO-SCRATCH-ORG | C | pass | 4 | known-remainder | none | Full second install; no `one org scratch` |
| 2026-09-05 | G-CROSS-INSTALL-SSO | C | pass | 4 | by-design | none | Distinct claims / JWTs per install |
| 2026-09-05 | G-EXEC-D-INIT | D | pass | 5 | — | none | `one project init` into `.customer-sandbox` |
| 2026-09-05 | G-EXEC-D-TEMPLATE-DEPLOY | D | pass-with-workaround | 3 | docs-drift | fixed-in-#27 | `--suite` needed Deno on **API** |
| 2026-09-05 | G-DENO-PATH | D | pass-with-workaround | 4 | docs-drift | fixed-in-#27 | Clear `DENO_PATH` error |
| 2026-09-05 | G-DENO-API-VS-WORKER | D | pass-with-workaround | 3 | docs-drift | fixed-in-#27 | Suites=API Deno; async automations=worker Deno |
| 2026-09-05 | G-JSONLOGIC-POLARITY | D | fail | 2 | docs-drift | fixed-in-#27 | JSONLogic `true` = invalid; `"!!"` was wrong |
| 2026-09-05 | G-QUERY-FIELD | D | pass-with-workaround | 3 | docs-drift | fixed-in-#27 | Query body field is `object` |
| 2026-09-05 | G-EXEC-D-PROJECT | D | pass-with-workaround | 3 | docs-drift | fixed-in-#27 | `Project__c` + live Account→Project after worker Deno |
| 2026-09-05 | G-EXEC-E-PROD | E | pass | 4 | — | none | Same Git SHA on prod; rows did not copy |
| 2026-09-05 | G-CLI-SUITE-TRUNC | E | pass-with-workaround | 3 | product-bug | [#29](https://github.com/MajestaNet/one/issues/29) | CLI `truncate(..., 200)` on suite JSON |
| 2026-09-05 | G-PACK-CHECKSUM-PER-ORG | E | pass | 4 | by-design | none | Pack includes `sourceInstallId` |
| 2026-09-05 | G-DOCS-PROMOTE | E | fail | 2 | docs-drift | fixed-in-#27 | Customizations doc said promote / CodeCommit |
| 2026-09-05 | G-FEATURE-FLAGS | F | pass-with-workaround | 3 | docs-drift | fixed-in-#27 | Empty `FEATURE_FLAGS` enables agents |
| 2026-09-05 | G-EXEC-F-MCP | F | pass | 5 | — | none | `/mcp/tools` + `initialize` on test |
| 2026-09-05 | G-HOSTED-LOOP-SHIP | F | pass | 4 | by-design | none | `/agents/runs` → `awaiting_approval`; no `org_deploy` |
| 2026-09-05 | G-EXEC-F-HOSTED | F | pass | 4 | by-design | none | Same as G-HOSTED-LOOP-SHIP |

| Count | Outcome |
|---|---|
| 11 | pass |
| 12 | pass-with-workaround |
| 4 | fail |
| 3 | blocked-no-display |
| 0 | not-run |

Open defects from this run: **#28**, **#29**.

---

## Campaign 2 — customer-install simulation (dev / test / prod)

Runbook: [customer-install-simulation-test-run.md](./customer-install-simulation-test-run.md). Lab: [docker-compose.dev-test-prod.yml](../deploy/docker-compose.dev-test-prod.yml). Fixtures: `scripts/customer-install-sim-generate.sh`.

**Not started.** Executors paste prompts from [customer-install-simulation-playbook.md](./architecture/customer-install-simulation-playbook.md). Re-test #28 / #29; do not re-file.

### Beat catalog (record each)

| Card | Beat ids |
|---|---|
| S-A setup | `S-A-COMPOSE` · `S-A-CLAIM-PROD` · `S-A-CLAIM-TEST` · `S-A-CLAIM-DEV` · `S-A-HEALTH` · `S-A-MIGRATE-RACE` · `S-A-PEERS` · `S-A-CLI-ALIASES` |
| S-B packages + object | `S-B-PKG-DEP` · `S-B-PKG-ENABLE-ALL` · `S-B-OBJ-SITEVISIT` · `S-B-FIELD-MANAGED-EXT` · `S-B-LOOKUP-FAIL-WITHOUT-PKG` · `S-B-AUTHZ-STUBS` |
| S-C automations at scale | `S-C-NAMED` · `S-C-IMPORT-BAN` · `S-C-STUBS` · `S-C-PACK-TIME` · `S-C-WORKER-FANOUT` · `S-C-SUITE` · `S-C-CLI-TRUNC` |
| S-D same SHA | `S-D-DEPLOY-TEST` · `S-D-DEPLOY-PROD` · `S-D-NO-ROW-COPY` · `S-D-PKG-DRIFT` |
| S-E Control IDE | `S-E-SIGNIN` · `S-E-ENV-SWITCH` · `S-E-OPERATE` · `S-E-BUILD-OBJECTS` · `S-E-BUILD-PACKAGES` · `S-E-BUILD-AUTOMATIONS` · `S-E-BUILD-AGENTS` · `S-E-BUILD-TOOLS` · `S-E-BUILD-REPO` · `S-E-BUILD-DEPLOY` · `S-E-BUILD-INSPECT` · `S-E-GOVERN` · `S-E-SETTINGS` · `S-E-THEME` · `S-E-HONESTY` · `S-E-FROZEN` |

### Run results

```text
| YYYY-MM-DD | S-ID | S-X | outcome | DX | class | none or #N | one-line actual |
```

| Date | Beat | Card | Outcome | DX | Class | Issue | Actual (one line) |
|---|---|---|---|---|---|---|---|
| — | — | — | not-run | — | — | none | Campaign 2 not started |

---

## Next run (copy)

1. Duplicate the matching **Run results** table header; new date; same Beat ids where the card is the same.
2. File a GitHub issue only for new or still-open `fail` / actionable `pass-with-workaround` rows. Campaign 2 titles use `[campaign S-…]`.
3. Re-test #28 / #29 if still open; do not re-file duplicates — comment on the existing issue.
4. Do not append campaign essays to `backlog/BP-*.md`.
