# Control IDE ↔ API version compatibility — build plan

**Status:** Implemented (BP-025 Mitigated; Phase 4 adapters deferred until a real Client wire break)  
**Backlog:** [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md)  
**ADR:** [ADR-025: API revision versioning](../adr/025-api-revision-versioning.md)  
**Playbooks:** [agent-control-ide.md](./agent-control-ide.md) · [agent-api-families.md](./agent-api-families.md) · [agent-deploy.md](./agent-deploy.md)  
**Related:** [control-ide-build.md](../control-ide-build.md) · [release-cicd.md](../release-cicd.md) · [install-ide-connect-build-plan.md](./install-ide-connect-build-plan.md) · [one-cli-build-plan.md](./one-cli-build-plan.md) · [client-experience-build-plan.md](./client-experience-build-plan.md) · [ADR-004](../adr/004-three-api-families.md) · [ADR-012](../adr/012-customer-repo-and-control-ide.md)  
**Domain agents:** `control-ide` (Connect gate + session pin) · `api-families` (`/version`, `/me`, revision middleware) · `deploy-ops` (release coupling / CLI) · `db-backend-perf` only if config/constants need shared helpers (prefer `internal/compat` or `internal/httpapi`)

---

## Thesis

> Clients **pin an API revision**. The install advertises `[min, current]` and keeps those revisions working after product image upgrades. Control IDE / `one` / `sdk/client` migrate pins on their schedule; raising `min` is the forced-migration lever.  
> Separately, clients declare which **product minors** they were *tested against* — soft warn only, never the sole hard Connect SoR.

This amends the earlier “gate on `PRODUCT_VERSION` ± N minors” design. Product semver remains Ops/Deploy SoR; **wire stability** is axis C in [ADR-025](../adr/025-api-revision-versioning.md).

```text
Operator pastes install origin
  → GET /version (unauth)
       productVersion + apiRevision{min,current} + httpApi
  → auth (claim / PKCE / JWT / CC)
  → GET /client/v1/me   # echo productVersion + apiRevision
  → choose / confirm pin (default: min(current, clientPreferred))
  → evaluateRevision(pin, install)     # HARD gate
  → evaluateProductTestedAgainst(...) # SOFT warn
  → ok | warn+continue | block+CTA (+ override for emergencies)
  → persist pin on EnvConnection; send One-API-Revision on all family calls
```

---

## Design recommendation: URL / pin model (locked)

### Problem with “URL v of the install = product version”

Pinning clients to `PRODUCT_VERSION` (or `/v0.12` meaning product 0.12) couples wire behavior to every image roll (kernel, Ops, managed packages). Install upgrades then **break** older IDEs — the opposite of “v12 still works while v14 is available.”

### Correct model

| Axis | Example | Role |
|---|---|---|
| Product version | `PRODUCT_VERSION=0.14.0` | Image / Ops / Deploy ranges |
| HTTP family major | `/client/v1` | ADR-004 rare breaking-family |
| **API revision (pin)** | `12` while install `current=14` | Client-selected wire contract |

**Preferred wire form** (keeps family paths stable):

```http
GET /client/v1/sobjects/Account
One-API-Revision: 12
```

**Allowed alternate** (same semantics, SF-flavored path segment under the family major):

```http
GET /client/v1/r12/sobjects/Account
```

**Rejected:** `/client/v12/...` as replacement for `/client/v1` (conflates family major with revision; route ownership explodes across Client/Metadata/Deploy/Ops).

Discovery:

```json
GET /version
{
  "version": "git-describe-or-ldflag",
  "productVersion": "0.14.0",
  "apiRevision": { "current": 14, "min": 12 },
  "httpApi": {
    "client": "v1",
    "metadata": "v1",
    "deploy": "v1",
    "ops": "v1",
    "auth": "v1"
  },
  "runtime": "go"
}
```

Client lifecycle:

1. Pin `12` in IDE env / CLI org / SDK config.
2. Install upgrades to product `0.15.0` with `apiRevision.current=15`, `min=12` → pin `12` **keeps working**.
3. Vendor publishes sunset: next image sets `min=13` → pin `12` **blocks** with CTA to migrate.
4. Client bumps pin to `14` (or `current`) when ready.

Early Majesta One may set `current ≈ product minor` (0.14.x → 14) as a **convenience only**. The pin SoR is always the integer, never parsed from `PRODUCT_VERSION`.

---

## Version glossary (do not conflate)

| Concept | Example | Role after this amendment |
|---|---|---|
| **HTTP URI family major** | `/client/v1` | Not the pin — ADR-004 |
| **API revision** | `12` / header `One-API-Revision` | **Hard compat SoR** for clients ↔ install wire |
| **Product version** | `PRODUCT_VERSION=0.3.1` | Ops / Deploy / soft “tested-against” for IDE/CLI |
| **IDE build version** | `control-ide-v0.3.0` | Declares preferred revision + tested product window |
| **CLI version** | `one` ldflag | Same as IDE for revision + tested-against |
| **Link-time `version`** | `GET /version` → `version` | Debug/build stamp — not compat SoR |
| **Customer pack format** | `one/v1` | Repo layout — unrelated |
| **Document schemas** | `one.runGraph/v1` | Payload shapes — unchanged |
| **Managed package version** | `package_installs` (BP-007) | Rides product upgrades |

---

## Locked decisions

| Decision | Choice | Rationale |
|---|---|---|
| Hard compat SoR | Install **`apiRevision`** window + client **pin** | Survives product upgrades; matches ADR-025 |
| Soft signal | IDE/CLI `targetProductVersion` + `supportedProductMinors` (default **N = 2**) | Commercial “tested against latest N minors” without inverted hard blocks |
| Pin transport | Header `One-API-Revision` required for implementing clients; optional `/r{N}/` alias under family | Preserves ADR-004 paths |
| Omitted pin | Default to install `apiRevision.current` | Back-compat for raw curl; SDKs/IDE must still send explicit pin |
| Unsupported pin | `400` + code `API_REVISION_UNSUPPORTED` (body includes `min`/`current`/CTA hints) | Stable machine handling; avoid inventing 426 unless later preferred |
| Pre-auth discovery | Unauthenticated **`GET /version`** | Works before JWT |
| Post-auth discovery | Echo `productVersion` + `apiRevision` on **`GET /client/v1/me`** | No Deploy scope required |
| Deploy `/environment` | Keep `productVersion`; add `apiRevision` (+ optional later `minControlIdeVersion`) | Deploy-scoped tooling |
| Connect hard block | Pin outside `[min,current]`; unparsable revision metadata in production | Enforces migration promise |
| Connect soft warn | Product minor outside tested window; pin &lt; current (optional “upgrade available”) | DX without bricking Connect |
| Break-glass | “Connect anyway” / CLI `--force-compat` — stores `compatOverride`; still **should** send a pin when possible | Ops debugging |
| Dev / loopback | Warn-only for product tested-against; revision still validated unless override | Local iteration |
| Multi-revision depth | Client family first; other families accept header + share window; adapters as breaking changes appear | Cost control |
| Capability bits | Keep Deploy `capabilities` / feature flags for additive features | Do not replace revision integers |
| Family `/v2` | Out of scope here | ADR-004 |
| License entitlement | Out of scope | [BP-062](../adr/030-install-agent-runtime.md) · [ADR-030](../adr/030-install-agent-runtime.md) |

### Support-window math

#### A. API revision (hard)

```text
ok    iff pin ∈ [install.min, install.current]
block iff pin < min OR pin > current OR pin/window unparsable (prod)
```

| Install window | Client pin | Verdict |
|---|---|---|
| `{min:12, current:14}` | `14` | **ok** |
| `{min:12, current:14}` | `12` | **ok** (legacy pin; optional warn “newer revision available”) |
| `{min:12, current:14}` | `11` | **block** `API_REVISION_UNSUPPORTED` |
| `{min:12, current:14}` | `15` | **block** (client newer than install — upgrade install or lower pin) |

#### B. Product tested-against (soft; N = 2)

IDE declares `supportedProductMinors = N` and `targetProductVersion = X.Y.Z` (newest product minor this build was cut/tested against).

```text
same MAJOR as target
AND install.MINOR ∈ [target.MINOR − (N − 1), target.MINOR]
→ warn if outside or skewed; do NOT hard-block once revision gate exists
```

Examples with `N=2`, IDE `targetProductVersion=0.4.2`:

| Install `productVersion` | Soft verdict |
|---|---|
| `0.4.x` / `0.3.x` | ok / mild warn |
| `0.2.x` / `0.5.x` / `1.0.0` | **warn** (“outside tested window”) — Connect allowed if revision pin ok |
| `""` / unparsable | warn on loopback; warn+banner in prod (revision gate still applies) |

Until revision middleware ships, Phase 1 may temporarily keep a **stricter** product gate as a bridge — but Phase 2+ must flip hard SoR to revision (see phases). Do not ship GA Connect hard-block on product semver alone.

### Client declaration shape

Control IDE embeds a manifest (renderer + main):

```json
{
  "ideVersion": "0.4.0",
  "preferredApiRevision": 14,
  "minApiRevision": 12,
  "targetProductVersion": "0.4.0",
  "supportedProductMinors": 2,
  "httpApi": {
    "client": "v1",
    "metadata": "v1",
    "deploy": "v1",
    "ops": "v1",
    "auth": "v1"
  }
}
```

- `preferredApiRevision` — default pin on new Connect (usually equals install `current` capped by IDE `preferredApiRevision`).
- `minApiRevision` — oldest revision this IDE build knows how to speak; if install `current < minApiRevision`, soft/hard policy: **block** (IDE too new for install wire) with CTA to upgrade install or use older IDE.
- `targetProductVersion` / `supportedProductMinors` — soft tested-against only.
- Patch-only IDE desktop fixes: bump `ideVersion` patch; leave revision + target product unless wire knowledge changed.

`one` and `sdk/client` mirror the same fields (ldflags / package const). Prefer shared parse helpers under `internal/compat` (Go) and a small IDE `compat.ts` — one semver dialect for product tested-against; revision math is plain integers.

**Pin selection algorithm (Connect):**

```text
preferred = min(ide.preferredApiRevision, install.apiRevision.current)
if preferred < install.apiRevision.min → block (IDE cannot speak install min)
if ide.minApiRevision > install.apiRevision.current → block (install too old for IDE)
else pin = max(preferred, install.apiRevision.min)  # or offer UI to pick ∈ [max(ide.min, install.min), min(ide.preferred, install.current)]
```

Persist the chosen pin; allow Settings to change pin within the intersection window without re-auth.

---

## Discovery surfaces

### Today (gaps)

| Endpoint | Auth | `productVersion`? | `apiRevision`? | Usable on Connect? |
|---|---|---|---|---|
| `GET /version` | none | Yes | **No** | Yes — enrich |
| `GET /client/v1/me` | client | No | **No** | Always called — enrich |
| `GET /deploy/v1/environment` | deploy | Yes | **No** | Often 403 for human JWTs |

### Target

| Endpoint | Change |
|---|---|
| `GET /version` | Keep `productVersion`. Add `apiRevision: {min,current}`. Add `httpApi` map. |
| `GET /client/v1/me` | Add `productVersion` + `apiRevision` (from config). |
| `GET /deploy/v1/environment` | Add `apiRevision`; keep `productVersion`. Optional later `minControlIdeVersion`. |

**Config:** introduce `API_REVISION_CURRENT` and `API_REVISION_MIN` (ints). Defaults may derive from a single `API_REVISION` / build-time const with `min = current` until sunset policy exists. Do **not** silently parse these from `PRODUCT_VERSION` in production code paths (tests may assert the convenience mapping in release scripts only).

**Ownership:** discovery fields are **api-families** (`handleVersion`, `handleMe`) reading config — not Deploy-scoped.

---

## Server middleware (API revision)

1. Register middleware on family mux prefixes (`/client/v1`, `/metadata/v1`, `/deploy/v1`, `/ops/v1`, `/auth/v1` as applicable).
2. Resolve pin: `One-API-Revision` header, else `/r{N}/` path prefix rewrite, else default `current`.
3. If pin ∉ `[min,current]` → write `API_REVISION_UNSUPPORTED` and stop.
4. Stash pin on request context for handlers/adapters.
5. Phase A: accept all pins in window with identical behavior (establish plumbing).
6. Phase B+: branch only where behavior actually diverges; prefer adapter functions over copy-paste route trees.

`/version`, `/health`, `/ready` stay revision-agnostic (no header required).

---

## Handshake sequence (Control IDE)

1. **Resolve base URL** (`checkInstallBaseUrl` — unchanged).
2. **Probe** `GET ${base}/version` — parse `apiRevision` + `productVersion`.
3. **Authenticate** — unchanged.
4. **Confirm** `GET /client/v1/me` — prefer echoed fields; fall back to probe.
5. **`selectPin(manifest, apiRevision)`** → pin or block.
6. **`evaluateRevision(pin, apiRevision)`** → hard status.
7. **`evaluateProductTestedAgainst(manifest, productVersion)`** → soft status.
8. **Gate UX**
   - revision block → CTA + optional override (override still records `compatStatus: overridden`).
   - revision ok + product warn → banner, allow Connect.
   - both ok → persist.
9. Optional Deploy `/environment` when scope present — not required for pin.
10. Persist on `EnvConnection`:

```ts
productVersion?: string;
apiRevisionPin?: number;
apiRevisionMin?: number;
apiRevisionCurrent?: number;
compatStatus?: "ok" | "warn" | "block" | "overridden";
compatCode?: string; // API_REVISION_UNSUPPORTED | INSTALL_REVISION_TOO_OLD | PRODUCT_OUTSIDE_TESTED | …
```

Env switcher / Settings badge when `compatStatus !== "ok"`. All `apiFetch` calls attach `One-API-Revision: ${apiRevisionPin}`.

### CTA mapping

| Code | Primary CTA |
|---|---|
| `API_REVISION_UNSUPPORTED` (pin &lt; min) | Migrate client pin / update Control IDE |
| `API_REVISION_UNSUPPORTED` (pin &gt; current) | Upgrade install product image (`/ops/v1`) |
| `INSTALL_REVISION_TOO_OLD` | Upgrade install (IDE `minApiRevision` &gt; install `current`) |
| `PRODUCT_OUTSIDE_TESTED` | Soft: upgrade install or IDE when convenient |
| `UNPARSEABLE_REVISION` | Fix `API_REVISION_*` on install |
| `UNPARSEABLE_PRODUCT` | Fix `PRODUCT_VERSION` (soft) |

---

## CLI + SDK parity

| Client | Behavior |
|---|---|
| `one` | On `auth login` / first org command: probe `/version`, select pin, hard-fail exit **3** on revision block unless `--force-compat`; send header on all calls; `--api-revision N` override within window |
| `sdk/client` (BP-040) | Constructor option `apiRevision`; default from package const; attach header |
| Agents / MCP | Inherit service client pin; document in customer-connect |

`--version` / `version` prints tool version + `preferredApiRevision` + tested product window.

---

## Release coupling

Product tags (`vX.Y.Z`) and IDE tags (`control-ide-vX.Y.Z`) stay **separate**. Coupling is **policy + manifest + revision changelog**:

| Rule | Detail |
|---|---|
| Every product image sets `API_REVISION_CURRENT` / `API_REVISION_MIN` explicitly | Release checklist; never rely on unset defaults in GA |
| Breaking wire change | Bump `current`; keep adapter for old pins until sunset |
| Sunset | Raise `min` with ≥ one IDE/product release notice |
| IDE release | Set `preferredApiRevision` to the newest revision this IDE speaks; set `minApiRevision` to oldest it can speak |
| Product tested-against | Set `targetProductVersion` to newest product minor in the CI matrix for that IDE cut |
| Product minor without IDE | Allowed — revision pin keeps older IDEs working if `min` unchanged |
| Checklist home | [release-cicd.md](../release-cicd.md) + IDE release notes: revision changelog section |

Do **not** require AWS or a managed channel for version discovery.

---

## CI / test matrix

| Layer | What |
|---|---|
| Unit (Go) | Middleware: missing/default/in-range/out-of-range; `/version` shape |
| Unit (IDE) | `selectPin` / `evaluateRevision` / soft product matrix; override; loopback |
| Integration | Live API: `/version` + `/me` fields; header rejected below min; IDE handshake stores pin |
| Compat matrix | Last-N **product** images with overlapping revision windows; IDE + CLI smoke |
| Contract | Connect does **not** require Deploy scope; hard gate is revision not product semver |

Until multiple tags exist, simulate with env overrides of `API_REVISION_*` and `PRODUCT_VERSION` on one binary.

---

## Phased delivery

### Phase 0 — Spec + ADR (this amendment)

| Work | Area | Done when |
|---|---|---|
| ADR-025 three-axis + pin model | `docs/adr/` | Linked from ADR index |
| This build plan amendment | `docs/architecture/` | Thesis + locked decisions match ADR-025 |
| BP-025 scope update | `backlog/` | Hard SoR = revision; product = soft |
| Playbook / index pointers | architecture + control-ide + api-families | Agents find the plan |

### Phase 1 — Discovery + pin plumbing (cross-plane)

| Work | Area | Agent |
|---|---|---|
| Config `API_REVISION_CURRENT` / `API_REVISION_MIN` | `internal/config` | `api-families` |
| Enrich `GET /version` + `GET /client/v1/me` (+ Deploy env) | `internal/httpapi` | `api-families` |
| Middleware accept header + context; default current; reject out of range | `internal/httpapi` | `api-families` |
| IDE `compat.ts` + manifest (`preferredApiRevision`, tested product) | `tools/control-ide` | `control-ide` |
| Persist pin + soft product warn on session | `tools/control-ide` | `control-ide` |
| `apiFetch` sends `One-API-Revision` | `tools/control-ide` | `control-ide` |
| Go + Vitest unit coverage | both planes | matching agents |

### Phase 2 — Connect + CLI hard gates on revision

| Work | Area | Agent |
|---|---|---|
| Connect: probe → selectPin → hard revision / soft product UX | Connect + Env badge | `control-ide` |
| Settings: change pin within intersection window | IDE | `control-ide` |
| `one` pin + `--api-revision` + `--force-compat` + exit 3 | `cmd/one` | `deploy-ops` |
| Docs: self-host `API_REVISION_*`, Connect troubleshooting | `docs/` | `deploy-ops` |

### Phase 3 — Release coupling + CI + SDK

| Work | Area | Agent |
|---|---|---|
| Release checklist: revision changelog + min raise policy | `release-cicd.md` | `deploy-ops` |
| Compat CI (env-override and/or last-N tags) | `.github/workflows` | both |
| `sdk/client` pin option (BP-040 alignment) | `sdk/client` | api-families / experience docs |
| Optional `/client/v1/r{N}/…` path alias | `internal/httpapi` | `api-families` |

### Phase 4 — Adapters + optional server IDE floor

| Work | Notes |
|---|---|
| First real per-revision behavior branch (when a breaking Client change needs it) | Prove adapter pattern |
| `minControlIdeVersion` on `/version` | Refuse ancient IDEs even if pin math is wrong |
| Early-warn copy one release before `min` raise | IDE strings |

---

## Explicit non-goals

- Using `PRODUCT_VERSION` as the client pin or hard Connect SoR (superseded by ADR-025).
- Replacing family majors with `/client/v12` paths.
- Changing HTTP family URI majors or removing flat `/v1` aliases (ADR-004 track).
- Embedding Control IDE in product images.
- License / seat checks (BP-015).
- Requiring private update CDN to *discover* versions (CTA may link BP-015 feed).
- GraphQL; MCP/SCIM path versioning under this BP (SCIM stays `/scim/v2`).
- Versioning customer metadata *shapes* via URL (use describe).
- Cross-install “one compat decision unlocks all peers” — each env evaluated on Connect / org use.

---

## File / symbol map (implementation cheat sheet)

| Concern | Path |
|---|---|
| `/version`, `/me` | `internal/httpapi/server.go` (`handleVersion`, `handleMe`) |
| Revision middleware | `internal/httpapi/revision.go` + `middleware.go` (`One-API-Revision`, `/r{N}/`) |
| Shared revision helpers | `internal/compat` |
| `PRODUCT_VERSION` / new revision env | `internal/config` |
| Deploy env SoR | `internal/deploy.EnvironmentInfo` / `GetEnvironment` |
| Existing product semver helpers | `internal/deploy/types.go` (`parseSemver`, `productVersionSatisfies`) — reuse for soft tested-against only |
| Shared revision helpers | `internal/compat` |
| Connect | `tools/control-ide/src/renderer/govern/ConnectSection.tsx` |
| Session | `tools/control-ide/src/renderer/session.ts` |
| API fetch | `tools/control-ide/src/renderer/api.ts` |
| IDE version / manifest | `tools/control-ide/package.json` + generated/hand `compat` module |
| CLI | `cmd/one` |
| SDK | `sdk/client` (Phase 3) |
| Live IDE API tests | `tools/control-ide/src/renderer/api.integration.test.ts` |

---

## Done when (BP-025 close criteria)

- [x] ADR-025 accepted and indexed
- [x] Install advertises `apiRevision.{min,current}` on `/version` and `/client/v1/me`
- [x] Middleware enforces pin ∈ window; clients send `One-API-Revision`
- [x] IDE declares `preferredApiRevision` / `minApiRevision` + soft product tested-against window
- [x] Connect hard-gates on **revision**; product skew is warn-only (override documented)
- [x] Session persists pin + compat status; UI surfaces warn/block; Settings can change pin in-window
- [x] `one` applies the same hard/soft rules (exit 3 / `--force-compat` / `--api-revision`)
- [x] Release docs cover revision bump + `min` sunset; CI covers unit + env-simulated matrix
- [x] `sdk/client` pin option documented or landed (Phase 3)
- [x] Optional `/r{N}/` path alias under family majors
- [x] Packaging sets `API_REVISION_*` (Compose / Helm / App Spec / `.env.example`)
- [x] BP-025 status → Mitigated; backlog table updated

**Deferred (Phase 4 — do not fake):** first real per-revision handler adapter; optional `minControlIdeVersion` on `/version`. Land when a breaking Client change needs a branched pin.

---

## Plane fence reminder

| Change | Edit | Do not edit from that agent |
|---|---|---|
| IDE gate / session / UX / header on fetch | `tools/control-ide/**` | `cmd/`, `internal/` unless cross-plane PR cites both playbooks |
| `/version` `/me` middleware / config | `internal/httpapi`, `internal/config` (+ tests) | `tools/control-ide/**` |
| CLI gate | `cmd/one`, optional `internal/compat` | IDE tree |
| SDK pin | `sdk/client/**` | product image COPY allowlist unchanged |

Cross-plane PRs should land Phase 1 API discovery + middleware **before or with** the IDE gate that consumes `apiRevision`.
