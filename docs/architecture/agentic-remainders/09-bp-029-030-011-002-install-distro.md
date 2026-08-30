# Install distribution remainders (Path A packaging, Ops App Platform roller, Path B upgrades) — remainder tech design + agentic build plan

**Work-order slot:** 9 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-029](../../../backlog/BP-029-app-platform-install.md) · [BP-030](../../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md) · [BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md)
**Track:** Finish
**Status of remainder:** Partial (all four items are Partially mitigated; this doc is remainder-only)
**Domain agents:** `deploy-ops` (primary) · `api-families` (Ops/Deploy HTTP notes only) · `authz-security` (JWT previous-key cutover in BP-002)
**Playbooks:** [agent-deploy.md](../agent-deploy.md) · [agent-api-families.md](../agent-api-families.md) · [agent-authz.md](../agent-authz.md)
**Existing plans (do not duplicate):** [do-app-platform-deploy-api-build-plan.md](../do-app-platform-deploy-api-build-plan.md) (Waves A/B **shipped**) · [deploy-cloud-agnostic-build-plan.md](../deploy-cloud-agnostic-build-plan.md) (host-free `/deploy/v1/cloud/*` **shipped**) · [deploy-cloud-capability-contract.md](../deploy-cloud-capability-contract.md) · [digitalocean-distribution-build-plan.md](../digitalocean-distribution-build-plan.md) · [outbound-otel-build-plan.md](../outbound-otel-build-plan.md) (BP-008 pair) · [ADR-007](../../adr/007-platform-ops-upgrades.md)

---

## 1. Remainder inventory

Honest mark of what is already in tree. Do **not** re-plan shipped Wave A/B, host-free cloud routes, or the memory Ops engine.

| Surface | Shipped (cite packages/tests) | Still open (agent-implementable) | Evidence (path) |
|---|---|---|---|
| Path A App Spec | `deploy/digitalocean/app.yaml` (api + worker, GHCR, instance counts, identity envs, SECRET `DATABASE_URL`) | Spec gaps: no `WEBHOOK_ENCRYPTION_KEY` SECRET; no commented OTEL / social-broker envs; `DEPLOY_PEER_MODE=allowlist` without matching operator note; digest still commented placeholder | `deploy/digitalocean/app.yaml` |
| App Spec validator | `scripts/validate-do-app-spec.go` + CI step | Does not require api `PLATFORM_PUBLIC_URL`; does not require `API_KEYS` / `AUTH_JWT_SIGNING_KEY` / `INSTALL_CLAIM_TOKEN` as `SECRET`; no fixture tests | `scripts/validate-do-app-spec.go`; `.github/workflows/ci.yml` (`Validate DigitalOcean App Spec`) |
| Digest apply helper | `scripts/apply-do-app-digests.sh` | Fragile: only rewrites commented `# digest:` / `REPLACE_WITH_*` and `PRODUCT_VERSION` when value is `"0.1.0"`; does not update an already-set digest; no tests; typo `DIGETS_FILE` | `scripts/apply-do-app-digests.sh` |
| Path A operator runbook | [self-host.md](../../self-host.md) Option A; [deploy/digitalocean/README.md](../../../deploy/digitalocean/README.md) | Upgrade section still points at Deploy `POST …/app/redeploy` as peer of Ops; no copy-paste digest→spec→`doctl apps update` sequence with health + `PlatformSmoke` | `docs/self-host.md` Path A **Upgrade** |
| Marketplace prep | [MARKETPLACE_PREP.md](../../../deploy/digitalocean/MARKETPLACE_PREP.md) | **Out of scope here** (BP-028 deferred) | — |
| Live DO smoke | [LIVE_SMOKE.md](../../../deploy/digitalocean/LIVE_SMOKE.md) (manual) | Operator-run only. Agents must not fake a live team-token run. Optional: keep the checklist current after the Ops roller lands | `deploy/digitalocean/LIVE_SMOKE.md` |
| Deploy cloud verbs | Host-free `/deploy/v1/cloud/*` + DO aliases; `CloudHost`; binding/scale/resize/provision; `POST …/app/redeploy` helper | Redeploy stays a **temporary** Deploy helper (ADR-007). Remainder is **Ops** App Platform roller, not more Deploy verbs | `internal/deploy/cloud.go`, `cloud_host.go`; `internal/httpapi/deploy_cloud_routes.go`; `internal/digitalocean/client.go` |
| DO Apps client | Get/Create/Update app; CreateDeployment; DB CRUD; AccountOK; httptest | No `ListDeployments` / wait-until-ACTIVE; Ops roller needs that | `internal/digitalocean/client.go` (no Wait/ListDeployments) |
| Community AWS CloudHost | Skeleton under `sdk/aws/cloudhost` | **Do not fill** as Path A. Community remainders stay in `sdk/aws` docs, not this work order | `sdk/aws/cloudhost/host.go` |
| Ops upgrade API | `/ops/v1/upgrades` confirm/list/get/rollback; `PlatformSmoke` + optional `PostUpgradeSmoke`; `MemoryRoller` default | Product `cmd/api` always wires `MemoryRoller`. ECS roller lives in **community** `sdk/aws/ops`. **No App Platform roller.** `Roller.Mode` comment still says `"local"` or `"ecs"` | `internal/ops/engine.go`, `roller.go`, `engine_test.go`; `internal/httpapi/ops_routes.go`; `cmd/api/main.go` |
| GHCR `v*` publish | `.github/workflows/release.yml` on tag `v*`; GHCR push; `image-digests-*.txt`; boundary + image-contents asserts | First **public** `v*` tag is an operator action. Remainder = process/docs/scripts so a human can cut it without inventing steps; agents must not `git tag` / publish | `docs/release-cicd.md`; `.github/workflows/release.yml` |
| CIS-adjacent hardening | Distroless `USER nonroot`; Helm `runAsNonRoot` / drop ALL / read-only root / PDB | No operator CIS-ish checklist for Path A/B; Compose runs as image default with no securityContext note; no optional Helm `networkPolicy` | `deploy/Dockerfile`; `deploy/helm/one/templates/deployment-*.yaml`; `deploy/docker-compose.yml` |
| OTEL on installs | `internal/otel` + Helm `OTEL_EXPORTER_OTLP_ENDPOINT`; resource attrs include `PRODUCT_VERSION` | App Spec + Compose have no OTEL wiring comments; Ops confirm/rollback emit **no** upgrade spans/metrics (ADR-007 follow-up; pair BP-008) | `internal/otel/otel.go`; Helm values; `deploy/digitalocean/app.yaml`; `deploy/docker-compose.yml` |
| Social broker on installs | Helm `AUTH_LOGIN_PROVIDERS` + secret keys; Compose comments; ADR-015 shipped | Path A App Spec has no social-broker env comments; self-host has a one-liner, not a Path A redirect-URI runbook | `deploy/helm/one/values.yaml`; `docs/self-host.md` **Auth notes** |
| Path B Compose upgrade | `deploy/docker-compose.yml` (build-from-source, `0.1.0`) | No digest-pin upgrade playbook (`image:` + digest, roll api+worker, health, smoke) | `docs/self-host.md` Compose section; `docs/product-upgrades.md` Path B |
| Path B Helm upgrade | Chart + DOKS/dev overlays; digest takes precedence over tag | One-line `helm upgrade` in self-host; no playbook for pin-by-digest, `rollout status`, health, optional `PostUpgradeSmoke`, rollback | `docs/self-host.md`; `docs/product-upgrades.md` |
| Helm `/ops` roller | None (product default is memory) | Optional in-cluster image PATCH roller **without** adding `k8s.io/client-go` to product `go.mod` | Product `go.mod` has no Kubernetes deps |
| Backup / restore | None in product docs/CI | Backup smoke script + CI gate on kernel schema dump/restore; Path A/B operator notes (Managed DB backups / `pg_dump`) | No `pg_dump` / backup docs under `docs/` |
| Secret rotation | Production length checks; single `AUTH_JWT_SIGNING_KEY` | No previous-key verify window; no Path A/B rotation playbook (JWT, `API_KEYS`, `WEBHOOK_ENCRYPTION_KEY`, DO PAT, DB password) | `internal/authz/jwt.go` `OneSigner` (one key); `internal/config/config.go` |
| Cross-install version inventory | `GET /ops/v1/upgrades/available` is per-install | Optional **operator** script that curls N install `/ops/v1/upgrades/available` — not a SaaS fleet plane | ADR-001 / BP-002 direction |
| Playbook drift | — | `agent-deploy.md` / module-map still cite `internal/ops/ecs.go` (moved to `sdk/aws/ops`) | [agent-deploy.md](../agent-deploy.md); [module-map.md](../module-map.md) |
| IDE Govern (BP-027) | Frozen | **Out of scope** | — |
| Marketplace publish (BP-028) | Deferred | **Out of scope** | — |

**Operator-only (not agent prompts except as a “do not fake” checklist):** live `doctl` / team-token smoke; cutting the first public `v*` tag; Vendor Portal; logging into DigitalOcean.

---

## 2. Detailed design (remainder only)

Locked product decisions stay: **Path A = DigitalOcean App Platform only**; **Path B = Compose + Helm**; community AWS is not a second Path A; no managed subscription fleet; no Droplet 1-Click; no IDE Govern UI. Product image rolls stay **`/ops/v1`** ([ADR-007](../../adr/007-platform-ops-upgrades.md)). Deploy cloud verbs stay host-free `/deploy/v1/cloud/*` ([deploy-cloud-capability-contract.md](../deploy-cloud-capability-contract.md)).

### 2.1 Path A packaging remainders (BP-029)

**Goal:** The checked-in App Spec + validator + digest helper are Marketplace-*ready packaging* (BP-028 still deferred). An agent can close CI/docs/script gaps without a DO account.

**App Spec (`deploy/digitalocean/app.yaml`)**

- Keep the in-repo file as a **production-shaped example** with placeholder identity and **commented** digests (validator must still accept that).
- Add SECRET entries (commented or `type: SECRET` without values) for `WEBHOOK_ENCRYPTION_KEY`. Keep `DATABASE_URL`, `API_KEYS`, `AUTH_JWT_SIGNING_KEY`, `INSTALL_CLAIM_TOKEN` as SECRET.
- Comment-only optional blocks (not required by validator): `OTEL_EXPORTER_OTLP_ENDPOINT`, `AUTH_LOGIN_PROVIDERS` + Google/Apple secrets, `DIGITALOCEAN_API_TOKEN` (already commented).
- Do not add an App Platform development database. Do not add Droplet 1-Click YAML.

**Validator**

Fail closed on:

| Check | Applies to |
|---|---|
| Existing shape (name, region, api service, worker, no `:latest`) | Already shipped |
| api component has `PLATFORM_PUBLIC_URL` env | **New** |
| `API_KEYS`, `AUTH_JWT_SIGNING_KEY`, `INSTALL_CLAIM_TOKEN` present as `type: SECRET` on api | **New** |
| `DATABASE_URL` SECRET (already) | Keep |
| Worker has identity + `DATABASE_URL` SECRET (already) | Keep |

Do **not** require image `digest` on the example file. Add `-strict-digest` (or a second invocation on a fixture that *has* digests) for operator copies after `apply-do-app-digests.sh`.

**Digest helper**

- Accept an existing `digest:` line (commented or live) and rewrite it to the release digest.
- Set `tag` and `PRODUCT_VERSION` from the filename semver even when current values are not `0.1.0`.
- Optionally set `API_REVISION_*` if the digests file grows those keys later; until then leave a comment in the script header.
- Fixture test: `testdata/image-digests-1.2.3.txt` + a copy of `app.yaml` → assert digest/tag/version; run from `go test` in `scripts/` or a small `scripts/apply-do-app-digests_test.go` via `exec`.
- CI: keep current validate; add “apply fixture + validate + strict-digest”.

**Docs**

- Path A upgrade in [self-host.md](../../self-host.md) / [product-upgrades.md](../../product-upgrades.md): digest file → `apply-do-app-digests.sh` → `doctl apps update` → `/healthz` `/readyz` → `POST /deploy/v1/tests/runs` `PlatformSmoke`. Point product rolls at `/ops/v1` once the App Platform roller exists (phase 2). Keep Deploy `redeploy` labeled **temporary helper**.
- Append to [LIVE_SMOKE.md](../../../deploy/digitalocean/LIVE_SMOKE.md) an **Ops upgrade** subsection that is still operator-run. Agents must not mark it executed.

### 2.2 Ops App Platform roller (BP-030 remainder — ADR-007)

**Fence**

| Concern | Family |
|---|---|
| Bind / scale / resize / provision peer | Deploy `/deploy/v1/cloud/*` (**shipped**) |
| Roll **this** install’s product digests; ledger; gate; rollback | **Ops** `/ops/v1/upgrades` |
| Customer metadata promote | Deploy bundles — unrelated |

`POST /deploy/v1/cloud/app/redeploy` remains. After the Ops roller ships: document it as a thin helper; do not delete aliases this cycle. Do **not** move product confirm/roll onto Deploy.

**Roller contract (extend existing `ops.Roller`)**

```text
Mode()           → "app_platform" | "local" | (community "ecs" only in sdk/aws)
CaptureCurrent() → opaque refs: "doapp:<appId>:api@sha256:…" and worker
Roll()           → UpdateApp spec (api+worker digest + tag + PRODUCT_VERSION + API_REVISION_* if provided)
                   then CreateDeployment; return new refs
WaitStable()     → poll Apps deployments until ACTIVE or error (new client method)
Rollback()       → UpdateApp previous digests + CreateDeployment
```

Reuse `internal/digitalocean.Client`. Extract digest-patch logic already in `digitalOceanHost.Redeploy` (`internal/deploy/cloud_host.go`) into a shared helper in `internal/digitalocean` (spec mutate) so Deploy helper and Ops roller cannot drift. Ops still owns the **ledger + gate**.

**Persistence**

Reuse `platform_upgrades.previous_*_task_def` / `new_*_task_def` as opaque ref strings (already nullable text). No new migration required unless an operator-facing `roller_mode` column is desired; prefer `GetAvailable().RollerMode` from the in-process roller.

**Binding**

The roller must target **this install’s bound app only**:

1. Prefer `DIGITALOCEAN_APP_ID` when set.
2. Else load `deploy_cloud_*` binding for host `digitalocean` (`appResourceId`).
3. If neither is set, `Mode()` stays `"local"` (today’s MemoryRoller). Confirm still records the run and runs the test gate (`SkipRoll` / local behavior).

Token: install-local `DIGITALOCEAN_API_TOKEN`. Never echo. Map DO 401/403/429 like Deploy cloud errors. Never create a second app (that is Deploy `provisionPeer`).

**AuthZ**

Unchanged: `GET` list/available/get = scope `ops`; `POST` confirm/rollback = `ops` + admin ([ADR-007](../../adr/007-platform-ops-upgrades.md)).

**`cmd/api` wiring**

Today always `&ops.MemoryRoller{}`. Remainder:

```text
if DIGITALOCEAN_API_TOKEN set AND (env app id OR DB binding):
    roller = ops.NewAppPlatformRoller(doClient, appID)
else:
    roller = MemoryRoller
```

Do not import `sdk/aws/ops` into `cmd/api`. Do not register AWS CloudHost in the product binary.

**Wait / failure**

Add `ListDeployments(appID)` (or Get latest) on the DO client. `WaitStable` times out (config; default on the order of minutes, tests inject a fake clock / immediate ACTIVE). On timeout or ERROR phase: engine already calls `rollbackBestEffort`.

**Tests**

`internal/digitalocean` httptest: UpdateApp + CreateDeployment + deployment poll. `internal/ops` unit: AppPlatformRoller Capture/Roll/Rollback against that mock. Engine tests already cover gate + rollback on `FailRoll` — add one case with a stub roller `Mode()=="app_platform"`. **No live DO in CI.**

### 2.3 First public `v*` process + CIS / OTEL / social runbooks (BP-011)

**First public `v*` (process, not a tag from the agent)**

[release.yml](../../../.github/workflows/release.yml) already publishes GHCR + `image-digests-*.txt` on `v*`. Remainder is a **human-run** procedure an agent can write:

1. GHCR packages `one-api` / `one-worker` visibility **public** (org setting — operator).
2. `main` green on `ci.yml`.
3. Annotated tag `vX.Y.Z` from `main` (operator). Workflow must not be changed to auto-tag on merge.
4. Confirm Release asset `image-digests-X.Y.Z.txt` and GHCR digests match.
5. Optional: apply those digests to a **copy** of `app.yaml` and `helm upgrade --dry-run` / Compose pin — still operator for live pull.

Agent-implementable support:

- [release-cicd.md](../../release-cicd.md): a **First public `v*`** section; strip leftover “managed subscription / Marketplace subscriber rolls” language that contradicts cancelled fleet (keep “release publishes artifacts; it does not roll customer installs”).
- `scripts/assert-image-digests-file.sh` (or Go): required keys `api_image`, `api_digest`, `worker_image`, `worker_digest`; digests must be `sha256:`.
- Fixture in CI using a sample file (the release job already writes the real file).
- Helm chart `appVersion` stays independent; document that operators pin **image digest**, not Chart.yaml.

**CIS-ish (not a certification product)**

Ship an operator checklist in [security.md](../../security.md) or a short `docs/install-hardening.md` linked from self-host:

| Control | Path A | Path B Compose | Path B Helm |
|---|---|---|---|
| Non-root | Distroless `nonroot` | Same image | `runAsNonRoot` 10001 (shipped) |
| Read-only root / drop caps | Image default | Document | Shipped on containers |
| No `:latest` | Validator | Playbook | Digest-over-tag (shipped) |
| Secrets not in git | App Spec SECRET type | `.env` gitignored | K8s Secrets (shipped) |
| Postgres not App Platform “dev DB” | Docs (shipped) | Compose volume is local-dev only | External Postgres (shipped) |
| Network | DO trusted sources | Host firewall | Optional NetworkPolicy template (**new**, default off) |

Optional Helm `networkPolicy.enabled` (deny ingress except api Service port from Ingress/LB; egress DNS + Postgres + OTLP). Default **false** so DOKS/EKS installs do not break. Compose: comment `user:` / read-only if the distroless image already constrains this — do not fight the image USER.

**OTEL runbook (pair BP-008; do not implement logs exporter)**

- App Spec + Compose: commented `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_SERVICE_NAME`.
- Self-host: copy-paste for Path A (App Platform secret) and Path B (Helm values already exist; Compose env).
- Resource attrs already include `PRODUCT_VERSION` — document that collectors can inventory version **per install** without a fleet plane.
- Upgrade-outcome spans belong in §2.4 (Ops), not a new OTEL SDK.

**Social-broker runbook (ADR-015)**

Path A: redirect URIs `https://<PLATFORM_PUBLIC_URL>/auth/v1/callback/<provider>` (confirm against [idp-agnostic-login-build-plan.md](../idp-agnostic-login-build-plan.md) when implementing). App Spec comments for `AUTH_LOGIN_PROVIDERS`, Google/Apple secrets. Helm already mounts secrets — document the Google Cloud / Apple console settings once for Path A and Path B. Do not make social the product default (`AUTH_LOGIN_PROVIDERS` empty).

### 2.4 Path B upgrades, optional Helm roller, backup, rotation, OTEL hooks (BP-002)

**Compose + Helm playbooks (required)**

Extend [product-upgrades.md](../../product-upgrades.md) and [self-host.md](../../self-host.md) with copy-paste:

1. Read `image-digests-X.Y.Z.txt`.
2. Pin api+worker **digests** (Compose `image: ghcr.io/…@sha256:…`; Helm `--set image.digest` / `worker.image.digest` + `install.productVersion` + `API_REVISION_*`).
3. Roll both components; wait healthy (`compose up -d` / `kubectl rollout status`).
4. `GET /healthz` `/readyz`.
5. Optional: `POST /ops/v1/upgrades` with `skipRoll: true` (or confirm after roll) so the ledger + `PlatformSmoke` run on Compose/Helm. If `skipRoll` stays test-only, add an explicit `recordOnly` JSON field **or** document using local roller (records LastRoll without calling Docker). Prefer documenting: operator-native roll, then `POST /ops/v1/upgrades` with images+version so MemoryRoller + gate run (today MemoryRoller “rolls” in-process only — **change** `MemoryRoller` notes to “ledger + gate; infrastructure is operator-native on Compose/Helm”).
6. Rollback = previous digest pins + same wait/gate.

**Optional Helm `/ops` roller (no client-go)**

Do **not** add `k8s.io/client-go` to product `go.mod`. Optional `K8sRoller`:

- Enabled when `ONE_K8S_ROLL=1` and in-cluster (`KUBERNETES_SERVICE_HOST`) plus namespace + deployment names (defaults `one-api` / `one-worker`).
- CaptureCurrent: GET Deployment, record `spec.template.spec.containers[0].image`.
- Roll: strategic-merge or JSON patch image to `repository@digest` parsed from `RollRequest.APIImage` / `WorkerImage` (require `@sha256:` — reject `:latest`).
- WaitStable: poll `status.conditions` Available / replica readiness via the same REST API (SA token at `/var/run/secrets/kubernetes.io/serviceaccount/`).
- Rollback: patch previous image strings.
- RBAC snippet in `deploy/helm/one/templates/` (optional, default off): Role `get,patch` on the two Deployments, bound to the api ServiceAccount (add SA if missing).

This is **optional**. Path B success does not depend on it. Compose has no Docker-socket roller (avoid mounting docker.sock into the API).

**Backup smoke**

- Script `scripts/backup-restore-smoke.sh`: given `DATABASE_URL`, `pg_dump --schema-only` (or data-less kernel) → restore into a throwaway DB (`one_restore_smoke`) → `go run ./cmd/migrate` or `EnsureKernel` equivalent → exit 0.
- CI: run on `go-test` job’s Postgres service (main + PRs that touch `migrations/**` is enough; not live DO backups).
- Docs: Path A = DigitalOcean Managed Database automated backups + PITR notes (operator); Path B = `pg_dump`/`pg_restore` of the customer DB; restore is last resort vs image rollback ([ADR-007](../../adr/007-platform-ops-upgrades.md) shared-DB rules).

**Secret rotation (zero-downtime JWT cutover)**

`OneSigner` today verifies with a single key. Remainder:

- Env `AUTH_JWT_PREVIOUS_SIGNING_KEY` (optional). Mint **only** with current `AUTH_JWT_SIGNING_KEY`. Verify: try current, then previous.
- Production length checks apply to previous when set.
- Helm/App Spec/Compose: optional env; playbook: (1) set previous=old, current=new, restart api+worker; (2) wait ≥ access JWT TTL (default 1h) **and** refresh idle if refresh tokens are HS256-bound to the same key — confirm against [refresh-token-session-build-plan.md](../refresh-token-session-build-plan.md) when implementing (if refresh HMAC uses the signing key, wait the idle window or revoke families); (3) unset previous.
- `API_KEYS`: additive comma-list; add new, then remove old (already supported).
- `WEBHOOK_ENCRYPTION_KEY`: document fallback to JWT key; rotating encryption key **without** previous cannot decrypt `enc:v1` — remainder is a playbook (“do not rotate encryption key casually”) unless a previous-encryption-key is already supported; do not invent a re-encrypt job this cycle unless `secretcrypt` already has a dual-key read.
- `DIGITALOCEAN_API_TOKEN` / `DATABASE_URL`: operator PAT / DB password rotate in DO + App Spec secret update (docs only).

Owner split: `authz-security` for `OneSigner` + tests; `deploy-ops` for env/Helm/App Spec/docs.

**PRODUCT_VERSION + OTEL upgrade hooks (pair BP-008)**

In `internal/ops` Confirm / Rollback:

- Span `one.ops.upgrade` with attrs `one.product_version.from`, `.to`, `one.ops.roller_mode`, `one.ops.status` (no image registry credentials, no DO token).
- Optional counter `one.ops.upgrade.completed` / `.failed` when `internal/otel` meter is active (no-op otherwise).
- Do **not** implement OTEL logs exporter (BP-008 remainder stays there).
- `GetAvailable` already returns `currentVersion`; document scraping resource attr `service.version` / `one.install_id` as per-operator inventory.

**Cross-install inventory**

A script `scripts/install-version-inventory.sh` that reads `BASE_URL` + bearer from a file and prints `PRODUCT_VERSION` / `rollerMode` from `GET /ops/v1/upgrades/available` (and/or `/deploy/v1/environment`). No new API family. No credentials store in `cmd/api`.

### 2.5 Failure modes

| Failure | Behavior |
|---|---|
| App Spec example missing digest | Validator OK; strict-digest on applied copies only |
| apply-do-app-digests on already-pinned spec | Must replace digest, not leave stale sha256 |
| DO token missing | Ops Mode `local`; Deploy `capabilities.cloud=false` (shipped) |
| Roller UpdateApp 404/403 | Fail run, no rollback of unknown previous; surface problem JSON |
| WaitStable timeout | rollbackBestEffort to CaptureCurrent refs |
| Helm NetworkPolicy default on | Would break Ingress — **default off** |
| JWT previous key unset mid-TTL | Access JWTs minted with old key fail — playbook requires overlap window |
| Backup smoke without `pg_dump` in CI image | Install postgresql-client in that job step |
| Agent “completes” LIVE_SMOKE without DO | Treat as incomplete; checklist only |

### 2.6 Lockstep IDE

None. Do not unfreeze Control IDE Govern ([BP-027](../../adr/030-install-agent-runtime.md)). Do not edit `tools/control-ide/**`.

---

## 3. Concrete agentic build plan

Phases are independently shippable. Prefer 1 → 2 (roller needs a trustworthy digest helper). 3 and 4.1 can proceed in parallel with 1.

### Phase 1 — BP-029 App Spec / validator / digest / CI

- **Owner:** `deploy-ops`
- **Allowed:** `deploy/digitalocean/`, `scripts/validate-do-app-spec.go`, `scripts/apply-do-app-digests.sh`, tests/fixtures under `scripts/` or `deploy/digitalocean/testdata/`, `.github/workflows/ci.yml`, `docs/self-host.md`, `docs/product-upgrades.md`, `deploy/digitalocean/README.md`
- **Forbidden:** Vendor Portal; `tools/control-ide`; `sdk/aws`; live `doctl`; `internal/` product behavior except if validator stays a `scripts/` main
- **Files likely:** `app.yaml`, `validate-do-app-spec.go`, `apply-do-app-digests.sh`, `ci.yml`, self-host Path A upgrade
- **Tests:** Go validator unit tests (table-driven missing SECRET / PLATFORM_PUBLIC_URL); digest-apply fixture; CI runs both
- **Exit criteria:** `go run ./scripts/validate-do-app-spec.go deploy/digitalocean/app.yaml` still green on the example; fixture apply produces spec that passes `-strict-digest`; CI fails if api lacks `PLATFORM_PUBLIC_URL` or bootstrap secrets are not SECRET
- **Depends on:** none (Wave A already shipped)

### Phase 2 — BP-030 Ops App Platform roller

- **Owner:** `deploy-ops` (`api-families` only if `/ops/v1` JSON fields change)
- **Allowed:** `internal/digitalocean`, `internal/ops`, `internal/deploy/cloud_host.go` (extract spec mutate only), `cmd/api/main.go`, `internal/httpapi/ops_routes.go` (if Available notes), `docs/product-upgrades.md`, `docs/ops.md`, `docs/api-families.md` Ops/Deploy fence sentence, `deploy/digitalocean/LIVE_SMOKE.md` (Ops subsection — operator)
- **Forbidden:** Filling `sdk/aws/cloudhost`; new Deploy cloud verbs; moving confirm to Deploy; IDE; live DO
- **Files likely:** `internal/digitalocean/client.go` (+ `*_test.go`), `internal/ops/app_platform.go` (new), `internal/ops/engine.go` (Mode comment), `internal/deploy/cloud_host.go`, `cmd/api/main.go`
- **Tests:** `go test ./internal/digitalocean/... ./internal/ops/...`; httptest deployments poll; roller rollback
- **Exit criteria:** With mocked DO, `POST /ops/v1/upgrades` in app_platform mode updates spec digests, waits ACTIVE, gates `PlatformSmoke`; failure rolls back previous refs; `cmd/api` uses MemoryRoller when token/binding absent; Deploy `redeploy` still works
- **Depends on:** Phase 1 digest helper (docs). Code can land without it. **Not** dependent on BP-028/027

### Phase 3 — BP-011 first `v*` process + CIS/OTEL/social runbooks

- **Owner:** `deploy-ops`
- **Allowed:** `docs/release-cicd.md`, `docs/self-host.md`, `docs/security.md` (or `docs/install-hardening.md`), `docs/ops.md`, `deploy/digitalocean/app.yaml` comments, `deploy/docker-compose.yml` comments, `deploy/helm/one` optional NetworkPolicy + values, `scripts/assert-image-digests-file.sh`, `.github/workflows/ci.yml` / `release.yml` (validate digest file format only)
- **Forbidden:** `git tag v*`; GHCR visibility clicks; Marketplace listing; AWS Path A; IDE chrome
- **Files likely:** release-cicd first-tag section; hardening checklist; Helm `templates/networkpolicy.yaml`; App Spec OTEL/social comments
- **Tests:** digest-file assert on a fixture; `helm template` if Helm is installed in CI (optional job — skip if not worth the action); `scripts/assert-product-boundary.sh` still green
- **Exit criteria:** A human can follow **First public `v*`** without inventing GHCR/digest steps; CIS-ish checklist exists for A/B; Path A OTEL + social redirect notes exist; NetworkPolicy default false
- **Depends on:** Phase 1 for digest apply in the first-tag dry-run docs

### Phase 4 — BP-002 playbooks + optional K8s roller + backup + rotation + OTEL hooks

Split if needed; one PR is OK if tests stay tight.

#### 4.1 Compose + Helm upgrade playbooks + inventory script

- **Owner:** `deploy-ops`
- **Allowed:** `docs/product-upgrades.md`, `docs/self-host.md`, `docs/ops.md`, `scripts/install-version-inventory.sh`, Helm NOTES upgrade blurb
- **Forbidden:** Fleet control plane in `cmd/api`
- **Tests:** none required beyond markdown accuracy; inventory script smoke with a mocked HTTP server optional
- **Exit criteria:** Copy-paste Path B upgrade + rollback exists; inventory script documented as operator-side
- **Depends on:** none

#### 4.2 Optional K8s roller

- **Owner:** `deploy-ops`
- **Allowed:** `internal/ops/k8s.go` (stdlib HTTP), `cmd/api` gate on `ONE_K8S_ROLL`, optional Helm SA/Role templates default off
- **Forbidden:** `k8s.io/client-go` in product `go.mod`; Docker socket roller
- **Tests:** httptest kube API PATCH/GET; skipped when env unset
- **Exit criteria:** MemoryRoller remains default; with fake kube API, Roll/Rollback patch images
- **Depends on:** Phase 2 only for shared WaitStable style; can land after or before

#### 4.3 Backup smoke

- **Owner:** `deploy-ops`
- **Allowed:** `scripts/backup-restore-smoke.sh`, `.github/workflows/ci.yml` (postgres service), `docs/ops.md` / `docs/product-upgrades.md`
- **Tests:** CI step green against empty kernel
- **Exit criteria:** Dump/restore/migrate round-trip in CI; operator backup notes for DO Managed DB + `pg_dump`
- **Depends on:** none

#### 4.4 JWT previous-key + rotation playbooks

- **Owner:** `authz-security` (signer) + `deploy-ops` (packaging/docs)
- **Allowed:** `internal/authz/jwt.go`, `internal/config/config.go`, Helm/App Spec/Compose env, `docs/security.md`, `docs/product-upgrades.md`
- **Forbidden:** Re-encrypt-all job; Cognito; IDE
- **Tests:** `go test ./internal/authz/... ./internal/config/...` — mint with new, verify old during overlap
- **Exit criteria:** Overlap window works; playbook covers JWT, API_KEYS, DO PAT, DB password, encryption-key warning
- **Depends on:** none (may coordinate refresh-token HMAC with BP-063 docs)

#### 4.5 Ops OTEL upgrade spans

- **Owner:** `deploy-ops` (pair [BP-008](../../../backlog/BP-008-production-packaging.md) / [outbound-otel-build-plan.md](../outbound-otel-build-plan.md))
- **Allowed:** `internal/ops/engine.go`, tests
- **Forbidden:** OTEL logs exporter; Control IDE dashboards
- **Tests:** span/exporter no-op when endpoint unset (existing otel tests + ops confirm still passes)
- **Exit criteria:** Confirm/rollback create `one.ops.upgrade` when OTLP configured; attributes have versions/status only
- **Depends on:** Phase 2 nice-to-have (roller_mode attr)

---

## 4. Explicit non-goals

- DigitalOcean Marketplace / Vendor Portal publish ([BP-028](../../../backlog/BP-028-digitalocean-marketplace-listing.md))
- Control IDE DO Govern UI / OAuth ([BP-027](../../adr/030-install-agent-runtime.md) — frozen)
- A second product Path A on AWS/GCP/Azure; filling `sdk/aws/cloudhost` ECS/RDS calls
- Managed subscription fleet / vendor multi-tenant control plane in `cmd/api` (ADR-001)
- Droplet 1-Click / AMI / EC2
- Kubernetes Marketplace 1-Click; Helm/DOKS provision APIs as Deploy cloud verbs
- Equating ECS Fargate with App Platform
- Moving product image rolls off `/ops/v1` onto Deploy `redeploy`
- Adding `k8s.io/client-go` (or Docker SDK) to the product module
- Agent-executed live DO smoke or cutting `v*` tags
- OTEL logs exporter (BP-008)
- Dual-path **customer** promote changes (repo→org already shipped)

---

## 5. Agentic implementation prompt(s)

Paste after this docs PR merges. One prompt per BP. Operator live-smoke / first public tag is a checklist the agent **must not fake**.

### 5.1 BP-029 — App Spec, validator, digest helper, CI

```text
You are the Majesta One deploy-ops agent. Implement BP-029 packaging remainders only (no live DigitalOcean, no Marketplace publish).

Read first:
- docs/architecture/agent-deploy.md
- docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md §2.1 and Phase 1
- backlog/BP-029-app-platform-install.md
- docs/architecture/do-app-platform-deploy-api-build-plan.md Wave A (already shipped — do not redo)
- deploy/digitalocean/app.yaml, README.md
- scripts/validate-do-app-spec.go
- scripts/apply-do-app-digests.sh
- .github/workflows/ci.yml (Validate DigitalOcean App Spec step)
- docs/self-host.md (Path A), docs/product-upgrades.md

Do:
1. App Spec: add WEBHOOK_ENCRYPTION_KEY as SECRET; comment-only optional OTEL and AUTH_LOGIN_PROVIDERS / Google-Apple secrets. Keep example identity placeholders. Do not require live digests in the checked-in example. Do not add a development database.
2. Validator: require api PLATFORM_PUBLIC_URL; require API_KEYS, AUTH_JWT_SIGNING_KEY, INSTALL_CLAIM_TOKEN as type SECRET on api. Add -strict-digest for copies that must pin sha256. Table-driven Go tests.
3. apply-do-app-digests.sh: rewrite existing digest lines (not only comments / REPLACE_WITH); set tag + PRODUCT_VERSION from image-digests-X.Y.Z.txt even when current version is not 0.1.0. Add a fixture test. Fix the DIGETS_FILE typo.
4. CI: validate example spec; apply fixture + validate + strict-digest.
5. Docs: Path A upgrade copy-paste (digest file → apply script → doctl apps update → healthz/readyz → PlatformSmoke). Label POST /deploy/v1/cloud/app/redeploy as a temporary helper; product rolls stay /ops/v1.

Do not:
- Log into DigitalOcean, run doctl against a real team, or claim LIVE_SMOKE.md executed
- Submit Vendor Portal / BP-028
- Edit tools/control-ide (BP-027 frozen)
- Fill sdk/aws/cloudhost or add a second Path A
- Implement the Ops App Platform roller (that is BP-030 prompt 5.2)

Tests: go test on validator/apply fixtures you add; go run ./scripts/validate-do-app-spec.go deploy/digitalocean/app.yaml; bash scripts/assert-product-boundary.sh
Update BP-029 remaining bullets when packaging CI/docs land (live smoke stays operator).
```

### 5.2 BP-030 — Ops App Platform roller (not community AWS)

```text
You are the Majesta One deploy-ops agent. Implement the BP-030 remainder: Ops App Platform roller for product upgrades (ADR-007). Do not fill community AWS CloudHost as Path A.

Read first:
- docs/architecture/agent-deploy.md
- docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md §2.2 and Phase 2
- backlog/BP-030-deploy-api-digitalocean-apps.md
- docs/adr/007-platform-ops-upgrades.md
- docs/architecture/deploy-cloud-capability-contract.md (redeploy is temporary; product rolls are Ops)
- docs/architecture/do-app-platform-deploy-api-build-plan.md (Wave B shipped; this is the Ops follow-up)
- internal/ops/engine.go, roller.go, engine_test.go
- internal/httpapi/ops_routes.go
- cmd/api/main.go (always MemoryRoller today)
- internal/digitalocean/client.go
- internal/deploy/cloud_host.go (digitalOceanHost.Redeploy — extract shared spec mutate)
- docs/product-upgrades.md, docs/ops.md, docs/api-families.md

Do:
1. Extend internal/digitalocean.Client with list/get latest deployment sufficient to WaitStable (httptest, no live DO).
2. Extract digest/tag/PRODUCT_VERSION/API_REVISION spec mutation shared by Deploy Redeploy and Ops.
3. New ops.AppPlatformRoller implementing ops.Roller: Mode app_platform; CaptureCurrent/Roll/WaitStable/Rollback against the bound app only (DIGITALOCEAN_APP_ID or deploy_cloud binding appResourceId). Reject :latest. Reuse platform_upgrades *task_def columns as opaque refs.
4. cmd/api: use AppPlatformRoller only when DIGITALOCEAN_API_TOKEN and a bound app id exist; otherwise MemoryRoller. Do not import sdk/aws/ops.
5. Keep POST /deploy/v1/cloud/app/redeploy; document it as temporary. Confirm/rollback stay /ops/v1 (scope ops; admin on writes).
6. Docs + LIVE_SMOKE.md Ops subsection as an operator checklist. Do not mark live smoke done.

Do not:
- Implement sdk/aws/cloudhost ECS/RDS API calls
- Add Deploy provision/scale/resize (already shipped)
- Edit tools/control-ide
- Call api.digitalocean.com from CI
- Build a multi-install fleet plane

Tests: go test ./internal/digitalocean/... ./internal/ops/... (mocked HTTP). After Dockerfile/.dockerignore changes only: scripts/assert-product-boundary.sh
Update BP-030 remaining: Ops roller shipped; live DO smoke still operator; AWS CloudHost still community.
```

### 5.3 BP-011 — First public v* process, CIS/OTEL/social runbooks

```text
You are the Majesta One deploy-ops agent. Implement BP-011 remainders that an agent can land in docs/scripts/Helm: first public v* process, CIS-ish hardening checklist, OTEL and social-broker install runbooks. Do not cut a git tag. Do not publish Marketplace (BP-028).

Read first:
- docs/architecture/agent-deploy.md
- docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md §2.3 and Phase 3
- backlog/BP-011-container-marketplace-fargate.md
- docs/architecture/digitalocean-distribution-build-plan.md
- docs/release-cicd.md
- .github/workflows/release.yml
- docs/self-host.md, docs/security.md, docs/ops.md
- deploy/Dockerfile, deploy/docker-compose.yml, deploy/helm/one, deploy/digitalocean/app.yaml
- docs/architecture/outbound-otel-build-plan.md (pair BP-008 — do not build logs exporter)
- docs/architecture/idp-agnostic-login-build-plan.md / ADR-015 (social broker already shipped in Go)

Do:
1. docs/release-cicd.md: "First public v*" human checklist (GHCR public packages, main green, annotated tag, confirm image-digests-*.txt). State clearly that agents must not git tag unless the operator explicitly asked. Remove/reword leftover managed-subscription fleet language; release still must not roll customer installs.
2. scripts/assert-image-digests-file.sh (or Go): require api_image/api_digest/worker_image/worker_digest; sha256: only. CI fixture. Optionally hook release.yml to validate the file it just wrote.
3. CIS-ish operator checklist (security.md or docs/install-hardening.md linked from self-host): non-root, no :latest, secrets, Postgres not App Platform dev DB, optional Helm NetworkPolicy default false. Distroless/Helm drop-ALL already shipped — document, do not regress.
4. OTEL: commented env on App Spec + Compose; self-host copy-paste for Path A/B. Helm already has values — document. Do not implement BP-008 logs exporter or Ops upgrade spans (prompt 5.4).
5. Social broker: Path A redirect URI runbook + App Spec comments; Path B points at existing Helm secret keys. Leave AUTH_LOGIN_PROVIDERS empty by default.

Do not:
- git tag v* / docker push / change GHCR visibility
- Live DO smoke or Vendor Portal
- AWS Marketplace / second Path A / Droplet 1-Click
- tools/control-ide
- Fill sdk/aws/cloudhost

Tests: digest-file fixture; scripts/assert-product-boundary.sh; helm template if you add NetworkPolicy (default off)
Update BP-011 remaining checkboxes for process/docs; leave Marketplace deferred and live App Spec smoke as operator.
```

### 5.4 BP-002 — Compose/Helm playbooks, optional Helm roller, backup, rotation, OTEL hooks

```text
You are the Majesta One deploy-ops agent (authz-security for JWT previous-key only). Implement BP-002 remainders: Path B upgrade playbooks, optional Helm /ops roller without client-go, backup smoke in CI, secret-rotation playbooks, PRODUCT_VERSION+OTEL upgrade hooks. No managed subscription fleet.

Read first:
- docs/architecture/agent-deploy.md
- docs/architecture/agentic-remainders/09-bp-029-030-011-002-install-distro.md §2.4 and Phase 4
- backlog/BP-002-dedicated-install-fleet-ops.md
- docs/adr/007-platform-ops-upgrades.md
- docs/product-upgrades.md, docs/ops.md, docs/self-host.md, docs/security.md
- internal/ops/engine.go, roller.go, cmd/api/main.go
- internal/authz/jwt.go, internal/config/config.go
- internal/otel/otel.go
- backlog/BP-008-production-packaging.md (pair spans only — no logs exporter)
- docs/architecture/refresh-token-session-build-plan.md (JWT overlap vs refresh HMAC)

Do:
1. Compose + Helm upgrade playbooks: pin digests from image-digests-*.txt, roll api+worker, healthz/readyz, optional PlatformSmoke / PostUpgradeSmoke, rollback = previous digests. Operator-native roll is the Path B default; MemoryRoller remains ledger+gate when ECS/App Platform/K8s rollers are unset.
2. Optional K8s roller: stdlib HTTPS to in-cluster kube API when ONE_K8S_ROLL=1; PATCH deployment images; no k8s.io/client-go; no docker.sock. Helm RBAC templates default off.
3. scripts/backup-restore-smoke.sh + CI against the job Postgres (pg_dump/restore/migrate). Docs: DO Managed DB backups (operator) + pg_dump Path B. Restore is last resort vs image rollback.
4. AUTH_JWT_PREVIOUS_SIGNING_KEY: mint with current, verify current then previous. Helm/App Spec/Compose optional env. Playbooks for JWT overlap, API_KEYS additive rotate, DO PAT, DB password; warn that WEBHOOK_ENCRYPTION_KEY rotate without dual-read breaks enc:v1.
5. Ops Confirm/Rollback: span one.ops.upgrade with from/to version, roller_mode, status (redact tokens). No-op when OTLP unset.
6. Optional scripts/install-version-inventory.sh curling N installs' GET /ops/v1/upgrades/available — not a control plane in cmd/api.

Do not:
- Multi-tenant fleet ops in cmd/api
- Import sdk/aws/ops into product api
- Implement App Platform roller unless it is missing and you are also scoped for prompt 5.2 (prefer separate PR)
- tools/control-ide; Marketplace; Droplet 1-Click
- Fake live backup of a DO cluster

Tests: go test ./internal/ops/... ./internal/authz/... ./internal/config/... ; backup-restore-smoke.sh in CI; assert-product-boundary.sh if Dockerfile/Helm templates that affect the image boundary change
Update BP-002 remaining bullets to match what landed.
```
