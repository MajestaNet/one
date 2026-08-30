# BP-048 one CLI — remainder tech design + agentic build plan

**Work-order slot:** 2 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-048](../../../backlog/BP-048-one-cli.md)
**Track:** Finish
**Status of remainder:** Partial (Phases 1–4 of the original plan are in tree; **Phase R1** cross-platform `one` assets are in tree; leftover is auth DX polish, a BP-033 waiter, and deferred scratch orgs)
**Domain agents:** `deploy-ops` (owner). `authz-security` only if a remainder slice calls existing `/auth/v1/token` from the CLI (no kernel grant changes). Not `control-ide`.
**Playbooks:** [agent-deploy.md](../agent-deploy.md) · [agent-routing.md](../agent-routing.md) · [module-map.md](../module-map.md) (customer package format + `cmd/one`)
**Existing plans (do not duplicate):** [one-cli-build-plan.md](../one-cli-build-plan.md) · [customer-dx-build-plan.md](../customer-dx-build-plan.md) · [builder-connect.md](../../builder-connect.md) · [ADR-030](../../adr/030-install-agent-runtime.md) · [ADR-025](../../adr/025-api-revision-versioning.md) · [ide-api-version-compatibility-build-plan.md](../ide-api-version-compatibility-build-plan.md) (CLI pin **shipped**) · [external-id-upsert-bulk-build-plan.md](../external-id-upsert-bulk-build-plan.md) (`datapack` CLI surface **shipped**; engine remainder stays BP-041) · [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) (isolation engine is BP-033, not this BP)

---

## 1. Remainder inventory

Honest read of `cmd/one` vs [one-cli-build-plan.md](../one-cli-build-plan.md): the original four phases are **done**. Do not re-plan auth aliases, `project init`, selective pack, `change create`, `v*` `one` asset (linux), IDE Ship twin, `--baseline-only`, keychain, or builder templates.

| Surface | Shipped (cite packages/tests) | Still open | Evidence (path) |
|---|---|---|---|
| Auth / aliases / default org | `auth login\|logout`, `org list\|use`; `~/.config/one/config.json` + env `ONE_ORG` / `ONE_TOKEN` / `ONE_API_KEY` / `ONE_BASE_URL` | Repo `environments/*.yaml` is **not** used as alias / `--base-url` hints. `one.yaml` `defaultOrg` is packed but CLI resolve ignores it. No `auth status` | `cmd/one/auth.go`, `cmd/one/config.go`, `cmd/one/org.go`; `LoadEnvironments` only called from `internal/datapack/manifest.go` + `internal/customerrepo/dx_test.go` |
| `project init` + builder templates | Embedded scaffold including `AGENTS.md` + `skills/{connect,query,customize,ship,govern}/SKILL.md` + `metadata/tools`; `--from-org` retrieve | None for this BP | `cmd/one/project.go`; `internal/customerrepo/scaffold.go` + `scaffold/`; `TestCreateChangeAndInitProject` |
| Selective pack / manifests | `--metadata` / `--manifest` on `pack`, `validate`, `org validate\|deploy` | None | `cmd/one/pack_cmds.go`; `customerrepo.PackOptions`; `TestSelectivePackIncludePaths` |
| `change create` | Writes `changes/<slug>/CHANGE.yaml`; optional `change/<slug>` git branch; flags after slug bind | None | `cmd/one/project.go` `cmdChangeCreate`; `cmd/one/flags.go` + `flags_test.go` |
| `org validate\|deploy\|retrieve` | Validate-first deploy; `--skip-validate` break-glass; `--suite` **after** apply; `--dry-run`; `--baseline-only`; stderr actionable summary | No httptest covering the HTTP loop. No waiter for future `202` async Deploy (BP-033) | `cmd/one/org.go`, `org_summary.go` + `org_summary_test.go`; engine tests live under `internal/httpapi/validate_local_test.go` (API, not CLI) |
| Release asset `one` | `.github/workflows/release.yml` cross-compiles `./cmd/one` (`CGO_ENABLED=0`) for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64; attaches suffixed names plus unadorned `one` (copy of linux/amd64). CI `one-cli-cross` compile-only matrix. `docs/release-cicd.md` lists the CLI set. | None for this BP (**Phase R1 done**) | `release.yml`; `.github/workflows/ci.yml` `one-cli-cross`; `docs/release-cicd.md`; `docs/examples/customer-dx-ci/` |
| OS keychain | `zalando/go-keyring`; `ONE_CREDENTIAL_STORE=auto\|file\|keychain`; file `0600` fallback | None | `cmd/one/keychain.go` + `keychain_test.go` |
| API revision pin (BP-025) | `GET /version` probe; pin stored on org; `One-API-Revision` on org HTTP **and** datapack `OrgClient`; `--api-revision` / `--force-compat`; exit 3 | None for this BP (pin landed) | `cmd/one/compat.go`; `orgPOST`/`orgGET` in `org.go`; `internal/datapack/apply.go` `ApiRevisionPin`; `TestOrgClientDoJSONSetsAPIRevisionHeader` |
| `datapack validate\|apply` (BP-041 CLI) | Commands exist; `--alias` / `--source-alias` / `--offline` | Engine/CSV/ingest-lane polish is **BP-041**, not this BP. Flag name in the upsert plan (`--org`) differs from shipped `--alias` (docs only) | `cmd/one/datapack.go`; `internal/datapack/` |
| IDE Ship/Repo parity | Done historically; **frozen** twin ([ADR-030](../../adr/030-install-agent-runtime.md)) | Do not extend Electron Ship | Control IDE tree — out of this remainder |
| Scratch-like ephemeral installs | — | **Deferred (Wave D / Deploy cloud)** | [one-cli-build-plan.md](../one-cli-build-plan.md) non-goals; BP-048 mitigation table |
| Isolation-safe tight loop | CLI is a client of Deploy HTTP; BP-033 Phase 1 returns `202` + `DEPLOY_BUSY` | **CLI waiter still missing** (owned here). Isolation engine stays [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) | [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) Phase 1 shipped; `cmd/one/org.go` has no 202 poller |
| Docs / DX copy | Builder-connect golden path; customer workflow; scaffold README CLI-first; `release-cicd.md` lists `one` + platform-suffixed assets; CI samples note linux/amd64 vs Mac/Windows names | Command matrix omits `datapack`; module-map “Entry binaries” omits `cmd/one` | `docs/customer-repo.md` tooling table; `docs/architecture/module-map.md` |

---

## 2. Detailed design (remainder only)

### 2.1 What this remainder is

`one` is already the **Ship path of record** for builders ([ADR-030](../../adr/030-install-agent-runtime.md) §5). Remaining BP-048 work is productization polish on that CLI — not a second DX model, not Control IDE chrome, and not a re-implementation of Phases 1–4.

Locked decisions from the original plan still hold: Go binary only (ADR-005); `~/.config/one`; path allowlist / `manifests/*.yaml` (not `package.xml`); one connected org per invocation; repo → org only; credentials OS keychain with file `0600` fallback.

### 2.2 Cross-platform release assets (primary leftover)

**Problem.** `release.yml` compiles `one` once on `ubuntu-latest` (`CGO_ENABLED=0`) and attaches a single ELF named `one`. Customer CI samples on Linux are fine. macOS / Windows builders cannot download that asset and run it; they must `go build ./cmd/one` from the product tree. That is the largest remaining “productized CLI” gap.

**Contract.**

| Asset name | GOOS/GOARCH | Role |
|---|---|---|
| `one` | linux/amd64 (copy of `one-linux-amd64`) | **Compat alias** — existing [github-actions.md](../../examples/customer-dx-ci/github-actions.md) / [gitlab-ci.md](../../examples/customer-dx-ci/gitlab-ci.md) keep working |
| `one-linux-amd64` | linux/amd64 | Canonical Linux CI binary |
| `one-linux-arm64` | linux/arm64 | Path B ARM hosts |
| `one-darwin-amd64` | darwin/amd64 | Intel Mac humans |
| `one-darwin-arm64` | darwin/arm64 | Apple Silicon humans |
| `one-windows-amd64.exe` | windows/amd64 | Windows humans |

Do **not** matrix `one-api` / `one-worker` / `one-migrate` for desktop OS — those remain linux static binaries for self-host. Only the customer DX CLI needs darwin/windows.

Version ldflags stay `-X github.com/MajestaNet/ide/internal/version.Version=${VERSION}`. `CGO_ENABLED=0` for all targets (`zalando/go-keyring` is pure Go).

**CI samples.** Keep curling `.../releases/download/${TAG}/one` on `ubuntu-latest`. Add a one-line note: that name is linux/amd64; Mac/Windows use the suffixed asset. Optional `ONE_CLI_URL` remains air-gap only.

**Docs.** [release-cicd.md](../../release-cicd.md) artifact set must list `one` (it currently does not, even for the linux binary that already ships). [builder-connect.md](../../builder-connect.md) / [local-development-mac.md](../../local-development-mac.md) may mention `go run ./cmd/one` **or** the darwin release asset — not Control IDE Ship.

**PR compile guard.** Cross-compile `./cmd/one` for the matrix in CI (no attach) so darwin/windows build breaks fail before a `v*` tag.

### 2.3 Auth DX gaps (small, CLI-owned)

#### `auth login` client_credentials mint

Today login requires a **pre-minted** JWT (`--token`) or API key (`--api-key`). Builders already mint via `POST /auth/v1/token` (`grant_type=client_credentials`) per [builder-connect.md](../../builder-connect.md) §2. The CLI should do that mint so humans/agents do not paste a one-hour JWT by default.

```text
one auth login --base-url https://<install> \
  --client-id <principalId> --client-secret <secret> [--alias test]
```

- POST `/auth/v1/token` with `grant_type=client_credentials` (JSON body, same as existing API tests). `--client-secret` without `--client-id` may send `client_secret` only, matching bootstrap API-key mint.
- Persist the **access_token** through existing `persistCredential` (keychain / file). Do **not** store refresh tokens: [BP-063](../../../backlog/BP-063-refresh-token-sessions.md) does not issue refresh on `client_credentials`.
- Keep `--token` / `--api-key` as explicit overrides (CI still uses `ONE_TOKEN`).
- Run existing `negotiateCliPin` after mint (same `--api-revision` / `--force-compat` / exit 3).
- No new AuthZ grants. No kernel changes. Scope: `cmd/one` only.

#### Environments YAML as alias hints

[one-cli-build-plan.md](../one-cli-build-plan.md) already says repo `environments/<role>.yaml` may supply `baseUrl` / `installRole` for alias hints; secrets never live there. `customerrepo.LoadEnvironments` exists and is **unused by the CLI**.

| Command | Behavior |
|---|---|
| `org list [-dir .]` | Print saved `~/.config/one` orgs (as today). Then, if `-dir` has `environments/*.yaml`, print a second block of **hints** (`alias`, `installRole`, `baseUrl`) marked `(repo)` — not logged-in |
| `auth login --alias test` without `--base-url` | If `environments/test.yaml` (alias or file stem) has `baseUrl`, use it. Else keep today’s “`--base-url` required” |
| `auth login -dir .` | Optional; default `-dir` `.` only for hint lookup, never writes secrets into YAML |
| Resolve `ONE_ORG` / default org | If flags+env+config still have no alias, and `-dir` is known, `one.yaml` `defaultOrg` may select a **saved** alias. It must not invent credentials |

Secrets never land in `environments/`. Hints are URLs + install identity only ([ADR-012](../../adr/012-customer-repo-and-control-ide.md)).

#### `auth status`

Read-only: default alias, base URL, `apiRevisionPin`, credential backend (`keychain` \| `file`), never the token. Exit 1 if no default org. Small DX; folds into the same slice as login mint.

### 2.4 `datapack` pin header (CLI-owned buglet)

`orgPOST` / `orgGET` set `One-API-Revision` from `resolvedOrg.ApiRevisionPin`. `internal/datapack.OrgClient.doJSON` does not. After `auth login` negotiated a pin, `datapack apply` can 400 on revision middleware.

**Fix:** add `ApiRevisionPin int` (or a header hook) on `OrgClient`; `cmd/one/datapack.go` copies `target.ApiRevisionPin` / source pin from `resolveOrgAuth`. No Client upsert semantics change (that remains BP-041). Align docs: shipped flag is `--alias`, not `--org`.

Do not expand datapack engine, CSV, or ingest lanes here.

### 2.5 Org HTTP regression tests

`cmd/one` tests cover config, keychain, flag-after-slug, and validate **summary** text. There is no test that `org validate` posts `validate-local`, that `org deploy` refuses a red validate, or that retrieve honors `--baseline-only`.

Add `httptest.Server` stubs in `cmd/one` (no Postgres, no `internal/testutil` API harness):

- `org validate` → `POST /deploy/v1/packages/validate-local` → exit 0 / 1 from `ValidateLocalResult.OK`
- `org deploy` with `OK=false` never `POST /deploy/v1/promotions`
- `org deploy` green path posts promotions with `bundleId` from validate
- `--skip-validate` creates a bundle then promotes (and prints the break-glass warning)
- `org retrieve --baseline-only` hits `GET /deploy/v1/packages/export` (body may be a tiny zip fixture if extract is exercised; otherwise assert the request)

Auth for these tests: `ONE_CONFIG_DIR` temp + `ONE_BASE_URL` / `ONE_TOKEN` (already used in `config_test.go`).

### 2.6 Async Deploy waiter (depends on BP-033; do not implement isolation)

When [customer-runtime-isolation-build-plan.md](../customer-runtime-isolation-build-plan.md) Phase 1 makes expensive validate/apply **async**, those endpoints return `202` + a job or `ExecutionRun` id, and `429` / `DEPLOY_BUSY` when `JOB_SLOTS_DEPLOY` is saturated.

**CLI contract (implement only after that HTTP exists):**

| Response | CLI |
|---|---|
| `200` / `201` with body as today | Unchanged |
| `202` + `jobId` or `runId` | Poll `GET` until `succeeded` / `failed` / `cancelled` / `throttled`; print the same validate/apply JSON on success |
| `429` `DEPLOY_BUSY` | Retry with `Retry-After` or bounded backoff; then fail with a clear message (do not hammer Client lane) |
| `--wait` (default true) / `--no-wait` | `--no-wait` prints the id and exits 0 (CI that polls elsewhere) |

Do **not** add admission lanes, worker job classes, or `ExecutionRun` objects in this BP. Owner of that HTTP is `api-families` + `worker-jobs` + `deploy-ops` under BP-033. This remainder only specifies how `cmd/one` consumes it.

### 2.7 Scratch orgs — Deferred (Wave D)

Not the primary prompt. When Deploy cloud provision is real ([BP-030](../../../backlog/BP-030-deploy-api-digitalocean-apps.md) / host-free `/deploy/v1/cloud/*`), a later `one org create` / `one org delete` could wrap ephemeral test installs. That is Wave D / cloud, not Finish Ship. Do not scaffold a fake scratch product in `cmd/one` now.

### 2.8 AuthZ, storage, failure modes

Unchanged from the original plan:

- CLI is a JWT/API-key **client**. Deploy still requires `deploy` + promote capability. Datapack apply is Client upsert (existing).
- Config dir mode `0700`; `credentials.json` mode `0600`; keychain omits plaintext.
- Exit 0 / 1 / 2 as in the command matrix; compat hard-block remains exit **3**.
- No multi-org fan-out. No peer promote.

### 2.9 Lockstep IDE

**None.** IDE Ship/Repo parity is frozen ([ADR-030](../../adr/030-install-agent-runtime.md)). Do not prompt Electron Ship, New change chrome, or env-strip work. BP-065 lockstep is unrelated to this remainder.

---

## 3. Concrete agentic build plan

### Phase R1 — Cross-platform `one` on `v*` releases

- **Status:** Done
- **Owner domain agent:** `deploy-ops`
- **Packages allowed:** `.github/workflows/release.yml`, optionally `.github/workflows/ci.yml` (compile-only matrix); `docs/release-cicd.md`; `docs/examples/customer-dx-ci/*`; `docs/builder-connect.md` and/or `docs/local-development-mac.md` (install sentence only). `Makefile` `build` may keep host-only `bin/one`.
- **Packages forbidden:** `tools/control-ide/**`; `internal/deploy` engine; `cmd/api`; scratch-org commands; datapack engine
- **Files likely to change:** `.github/workflows/release.yml`; `.github/workflows/ci.yml`; `docs/release-cicd.md`; `docs/examples/customer-dx-ci/github-actions.md`; `docs/examples/customer-dx-ci/gitlab-ci.md`
- **Tests to add or extend:** CI job that `GOOS`/`GOARCH`-builds `./cmd/one` for the matrix (assert `file`/`go version -m` where available). No `make test-ide`
- **Exit criteria:** A `v*` workflow (or dry-run of the same script locally) produces the six names in §2.2; unadorned `one` remains linux/amd64; `release-cicd.md` lists `one`; linux CI samples still curl `one`
- **Dependencies:** None. Independent of R2

### Phase R2 — Auth DX + environments hints + datapack pin + org httptest

- **Owner domain agent:** `deploy-ops` (CLI client of `/auth/v1/token` — do not edit grant logic)
- **Packages allowed:** `cmd/one/**`; tiny `internal/datapack` header field on `OrgClient` only; docs that describe `auth login` / `org list` (`builder-connect.md`, `customer-developer-workflow.md`, `one-cli-build-plan.md` command matrix, `customer-repo.md` tooling table). Optional `internal/customerrepo` only if hint helpers belong there (prefer using `LoadEnvironments` as-is)
- **Packages forbidden:** `tools/control-ide/**`; `internal/httpapi` grant handlers; `internal/authz`; `migrations/`; BP-041 upsert/Bulk; BP-033 admission
- **Files likely to change:** `cmd/one/auth.go`, `config.go`, `org.go`, `datapack.go`, `main.go` (usage); `internal/datapack/apply.go`; new `cmd/one/org_http_test.go` (name flexible); maybe `cmd/one/auth_login_test.go`
- **Tests to add or extend:** `go test ./cmd/one` — httptest token mint + stored JWT (no secret in `credentials.json` when keychain); `org list -dir` shows repo hints; login without `--base-url` uses environments YAML; datapack client sets `One-API-Revision`; org validate/deploy httptest per §2.5
- **Exit criteria:** `one auth login --client-id --client-secret` stores a minted access token; `org list -dir` shows `environments/*.yaml` hints; `datapack apply` sends the pin; org HTTP tests fail if validate-first is broken
- **Dependencies:** None on R1. Uses existing `/auth/v1/token` and ADR-025 middleware

### Phase R3 — Async waiter (after BP-033 Phase 1 HTTP)

- **Owner domain agent:** `deploy-ops`
- **Packages allowed:** `cmd/one` wait/poll helper; docs for `--wait` / `--no-wait` / `DEPLOY_BUSY`
- **Packages forbidden:** Isolation engine (`internal/httpapi` admission, worker classes, `ExecutionRun` seed) — those are BP-033
- **Files likely to change:** `cmd/one/org.go` (and shared HTTP helper if extracted in R2)
- **Tests to add or extend:** httptest `202` then terminal GET; `429` retry; `--no-wait` exits 0 with id
- **Exit criteria:** Against a stub that returns `202`, `org validate` / `org deploy` still print a final report or a clear busy error. Live isolation behavior is proven on the BP-033 branch
- **Dependencies:** [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) Phase 1 contract (`202` + id, `DEPLOY_BUSY`). Do not start this phase speculatively

### Phase D — Scratch-like ephemeral installs (Deferred)

- **Owner domain agent:** `deploy-ops` when Wave D is opened
- **Packages allowed (then):** `cmd/one` wrappers around host-free `/deploy/v1/cloud/*` provision/teardown
- **Forbidden now:** Any `org create` / scratch command, fake local Docker “scratch org”, or Electron twin
- **Exit criteria:** N/A until cloud provision is a product path
- **Dependencies:** BP-030 / deploy-cloud capability; not Finish Ship

---

## 4. Explicit non-goals

- Re-planning shipped Phases 1–4 (auth file store, project init, selective pack, change create, linux `one` asset, IDE Ship parity, `--baseline-only`, keychain, builder templates)
- Control IDE Ship / Repo / New change / env-strip work ([ADR-030](../../adr/030-install-agent-runtime.md) — frozen twin)
- BP-025 revision handshake redesign (CLI pin already ships in `cmd/one/compat.go`)
- BP-041 upsert / Bulk / pack-engine remainder (`datapack` commands already exist)
- BP-033 admission lanes, async Deploy **server**, job classes, `ExecutionRun` objects
- BP-063 refresh tokens on CLI login (`client_credentials` must not mint refresh)
- Peer-to-peer promote; Git as apply SoR; `package.xml`; multi-org fan-out
- Homebrew / npm installers; attaching darwin `one-api` server binaries
- Scratch-org product in this Finish slice (Wave D only)
- Checksum-stability investigation from the 2026-08 spin (pack identity lives in `internal/customerrepo` / Deploy, not a new CLI command)

---

## 5. Agentic implementation prompt(s)

### Slice R1 — Cross-platform `one` release assets — **Keep (in tree)**

Do not paste this prompt. R1 binaries and CI matrix shipped. Next executable remainder is **Slice R2**.

### Slice R1 — Cross-platform `one` release assets (historical)

```text
You are the Majesta One deploy-ops agent. Implement BP-048 remainder Phase R1 only: cross-platform customer DX CLI binaries on product v* releases.

Read first:
- docs/architecture/agentic-remainders/02-bp-048-one-cli.md (§2.2, Phase R1)
- docs/architecture/one-cli-build-plan.md (do not re-plan shipped phases)
- docs/architecture/agent-deploy.md
- docs/adr/030-install-agent-runtime.md (CLI is Ship path of record; do not touch Control IDE)
- .github/workflows/release.yml
- docs/release-cicd.md
- docs/examples/customer-dx-ci/github-actions.md
- docs/examples/customer-dx-ci/gitlab-ci.md
- backlog/BP-048-one-cli.md

Edit scope (only):
- .github/workflows/release.yml — CGO_ENABLED=0 cross-compile ./cmd/one for:
  linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  Attach: one-linux-amd64, one-linux-arm64, one-darwin-amd64, one-darwin-arm64,
  one-windows-amd64.exe, plus unadorned `one` as a copy of linux/amd64 (compat).
  Keep one-api / one-worker / one-migrate as linux-only host binaries.
  Same version ldflags as today.
- Optional: .github/workflows/ci.yml compile-only GOOS/GOARCH matrix for ./cmd/one
  (do not attach binaries from CI).
- docs/release-cicd.md — list `one` (and platform-suffixed names) in the artifact set;
  today it omits the CLI even though linux `one` already ships.
- docs/examples/customer-dx-ci/* — keep curling `one` on ubuntu-latest; note that
  name is linux/amd64 and Mac/Windows use suffixed assets. ONE_CLI_URL remains air-gap.
- Optional one sentence in docs/builder-connect.md or docs/local-development-mac.md
  on where to get the darwin binary vs `go run ./cmd/one`.

Tests:
- The release script / a local reproduction of the cross-compile loop must produce
  all six names. CI compile-only matrix if you add it.
- Do not run make test-ide. go test ./cmd/one is enough if you touch no Go.

Out of scope:
- tools/control-ide/** (IDE Ship is frozen; no Electron parity)
- cmd/one command behavior, auth, datapack, scratch orgs
- Cross-compiling one-api / one-worker / one-migrate for darwin/windows
- BP-033 isolation, BP-041 engine, BP-025 pin redesign
- backlog/README.md, docs/architecture/README.md

When done: keep BP-048 status Partially mitigated; note R1 in the remainder doc
only if you close the slice (optional). Commit Go/docs/workflow only.
```

### Slice R2 — Auth DX, environment hints, org httptest, 202 waiter

```text
You are the Majesta One deploy-ops agent. Implement BP-048 remainder Phase R2 only:
CLI auth DX polish. Do not implement scratch orgs or async Deploy waiters.

Read first:
- docs/architecture/agentic-remainders/02-bp-048-one-cli.md (§2.3–2.5, Phase R2)
- docs/architecture/one-cli-build-plan.md
- docs/builder-connect.md
- docs/architecture/agent-deploy.md
- docs/adr/030-install-agent-runtime.md
- docs/adr/025-api-revision-versioning.md (pin already ships in cmd/one/compat.go)
- backlog/BP-048-one-cli.md
- cmd/one/auth.go, config.go, org.go, datapack.go, compat.go, main.go
- internal/customerrepo/dx.go (LoadEnvironments — use it; do not reinvent)
- internal/datapack/apply.go (OrgClient.doJSON)
- internal/httpapi/auth_routes.go (client_credentials shape — call it, do not edit)

Edit scope:
- cmd/one/** 
- internal/datapack OrgClient only: send One-API-Revision from resolved pin
- Docs that describe login / org list: docs/builder-connect.md,
  docs/customer-developer-workflow.md, docs/architecture/one-cli-build-plan.md
  command matrix (add datapack + auth status if you add the command),
  docs/customer-repo.md tooling table (may mention datapack)

Implement:
1. `one auth login --client-id --client-secret` (and secret-only bootstrap, matching
   existing /auth/v1/token). POST grant_type=client_credentials. Store access_token
   via persistCredential. Do not persist refresh tokens. Keep --token / --api-key.
   Still run negotiateCliPin; exit 3 on compat block unless --force-compat.
2. `one auth status` — alias, baseUrl, pin, backend; never print the secret.
3. `org list [-dir .]` — existing saved orgs, plus environments/*.yaml hints via
   customerrepo.LoadEnvironments (no secrets). `auth login --alias X` without
   --base-url uses matching environments YAML baseUrl when present.
4. datapack apply/validate HTTP must send One-API-Revision from resolveOrgAuth.
   Docs: flag is --alias (not --org).
5. httptest in cmd/one: org validate honors OK; org deploy does not promote on
   failed validate; skip-validate still warns; pin header on datapack client;
   login mint does not write JWT plaintext when ONE_CREDENTIAL_STORE=keychain
   (use existing memKeyring test backend).

Tests: go test ./cmd/one  (and go test ./internal/datapack if you touch OrgClient).
No make test-ide. No product image.

Out of scope:
- tools/control-ide/**  (do not prompt or edit Electron Ship/Repo)
- internal/httpapi, internal/authz, migrations (no new grants)
- BP-041 upsert/Bulk/CSV; BP-033 async Deploy / DEPLOY_BUSY waiter (Phase R3)
- Scratch orgs / `one org create`
- Release.yml cross-compile (that is Slice R1)
- Peer promote, package.xml, multi-org fan-out
- backlog/README.md, docs/architecture/README.md

Do not re-plan shipped CLI phases. Commit when tests pass.
```
