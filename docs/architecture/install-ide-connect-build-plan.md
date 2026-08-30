# Install → Control IDE connect — build plan

Product direction: **default install is a single Prod**, so first Control IDE sign-in is **one public API URL**. Multi-env (dev/test/…) is **customer opt-in** later. Document the DigitalOcean-preferred path and the same first-admin steps for AWS / Azure / generic Kubernetes.

**Playbook:** [agent-deploy.md](./agent-deploy.md) · [agent-control-ide.md](./agent-control-ide.md) · [agent-authz.md](./agent-authz.md)  
**Related:** [self-host.md](../self-host.md) · [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) · [multi-env-deploy.md](../multi-env-deploy.md) · [local-development-mac.md](../local-development-mac.md) · [ADR-006](../adr/006-jwt-auth.md) · [ADR-015](../adr/015-idp-agnostic-social-login.md)  
**Backlog:** [BP-015](../adr/030-install-agent-runtime.md) (IDE download channel) · [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) (DO 1-Click Marketplace deferred) · [BP-029](../../backlog/BP-029-app-platform-install.md) (App Platform packaging) · [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) (Deploy DO cloud) · [BP-027](../adr/030-install-agent-runtime.md) (IDE DO Govern — frozen; not connect)  
**Active DO plan:** [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md)  
**Domain agents:** `deploy-ops` (docs / Helm NOTES); AuthZ wording cites `authz-security` (no AuthZ code in this plan); IDE Connect UX references only (`control-ide`)

---

## Thesis

> After a Majesta One install, the customer claims the install (email + password) via `POST /auth/v1/install/claim` or Control IDE Connect — **no IDE required**. The public URL is a locator only. Bootstrap `API_KEYS` remain break-glass. Then configure customer SSO under Metadata install auth; later humans use SSO (optional JIT) or password.

```text
DO App Platform (Path A default) or Path B Compose/Helm
  → one Prod install + public API URL + INSTALL_CLAIM_TOKEN
  → POST /auth/v1/install/claim (curl or IDE)
  → first SystemAdmin (email + password) + Majesta One JWT
  → PUT /metadata/v1/install/auth (SSO + optional JIT)
  → Control IDE Connect (optional) / password or SSO login
```

---

## Product decisions (locked)

| Decision | Choice | Rationale |
|---|---|---|
| Default topology | **One Prod install** (`INSTALL_ROLE=prod`) | First sign-in = one URL; no sibling discovery required |
| Multi-env | **Opt-in** — extra Helm releases + peers when the customer wants them | Avoids forcing three namespaces/DBs on day 0 |
| How IDE finds the install | Operator pastes **`platformPublicUrl`** (LB / Ingress HTTPS origin) | Dedicated install; no fleet discovery plane |
| Day-0 admin | **Install claim** (`INSTALL_CLAIM_TOKEN` → email + password SystemAdmin); `API_KEYS` break-glass | Works without Control IDE ([BP-037](../../backlog/BP-037-install-claim-customer-sso.md)) |
| First human admin | Claim creates SystemAdmin + password; then configure customer SSO | Social Google/Apple only if customer enables ([ADR-015](../adr/015-idp-agnostic-social-login.md)) |
| Cross-install SSO / “one login unlocks all envs” | **Deferred** (larger build) | Each install has its own Postgres + JWT issuer; peer hints only supply URLs |
| DO Marketplace OAuth as connect | **Non-goal** | BP-027 is day-2 resize, not install discovery |

---

## Phase 0 — Align topology language

**Goal:** Docs stop implying that every customer must provision dev/test/prod on day 0.

| Work | Area | Done when |
|---|---|---|
| This build plan | `docs/architecture/` | Linked from architecture index |
| DO distribution plan multi-env baseline | [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) | Default = one Prod; optional peers |
| Self-host guide | [self-host.md](../self-host.md) | Single release is the happy path; multi-env under Optional |
| Multi-env lead-in | [multi-env-deploy.md](../multi-env-deploy.md) | “Start with one Prod” |
| Helm NOTES / values comments | `deploy/helm/one/` | IDE URL + bootstrap-admin handoff; single-Prod story |

---

## Phase 1 — Default happy path (DigitalOcean App Platform)

**Near-term:** App Platform App Spec ([self-host.md](../self-host.md) Path A). Helm remains Path B for operators who want a cluster. **Later:** Marketplace 1-Click ([BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md)) wraps the same App Spec — connect steps stay the same.

### Steps

1. **Install one Majesta One App Platform app** against one Managed Postgres database. Set `INSTALL_ROLE=prod`, unique `INSTALL_ID`, and `PLATFORM_PUBLIC_URL` to the public HTTPS origin.
2. **Smoke** with the bootstrap key (`curl …/healthz` with `Authorization: Bearer <API_KEYS entry>`).
3. **Download Control IDE** — optional client under `tools/control-ide` ([ADR-012](../adr/012-customer-repo-and-control-ide.md)). Private update CDN / license portal work is frozen ([ADR-030](../adr/030-install-agent-runtime.md)); use the vendor build / Mac local runbook. **Org license activation is not a Connect step.**
4. **Connect** — Settings → Environments → Connect: paste the **one** public API URL (`platformPublicUrl`).
5. **Authenticate** (pick one):
   - **Install claim (preferred day-0):** `POST /auth/v1/install/claim` with `INSTALL_CLAIM_TOKEN` + admin email/password (curl or IDE Claim form) — returns JWT; works without IDE.
   - **Break-glass / ops:** mint a Majesta One JWT via `POST /auth/v1/token` (`client_credentials` with the bootstrap key), paste into Connect.
   - **Ongoing human login:** password grant, customer SSO (`provider=oidc` / token exchange), or optional Google/Apple when enabled. Configure SSO via `PUT /metadata/v1/install/auth` (Govern → Install auth in Control IDE).

After success, the env selector shows **that one Prod** environment. No second URL.

### Helm post-install copy (Path B)

Chart [NOTES.txt](../../deploy/helm/one/templates/NOTES.txt) must remind operators: set `platformPublicUrl`, point Control IDE at that origin, use bootstrap `API_KEYS` for day-0 admin / JWT mint.

---

## Phase 2 — Other clouds (AWS / Azure / generic Kubernetes)

Same **single-Prod** shape and Connect steps. No cloud marketplace as a GA channel; operators follow [self-host.md](../self-host.md) Path B.

| Step | What |
|---|---|
| Install | Same Helm chart + external Postgres 16+ (EKS / AKS / GKE / on-prem) |
| URL | Cloud LB or Ingress HTTPS origin → `install.platformPublicUrl` |
| Day-0 admin | Same `API_KEYS` `…+admin` bootstrap secret |
| Human admin | Client identity admin (`identity.manage`) creates the user + `SystemAdmin`; Google/Apple **or** Entra / Okta / Keycloak via [auth-adapters.md](../auth-adapters.md) → `POST /auth/v1/token/exchange` |
| IDE | Paste that **one** URL — no AWS/Azure-specific discovery |

Optional community AWS ECS Terraform under [`sdk/aws/deploy/ecs/`](../../sdk/aws/deploy/ecs/) remains a Path B extension only — not product GA.

---

## Phase 3 — Optional multi-env (docs only)

When a customer later wants test/dev (or more):

1. **Second Helm release** — new namespace + **separate** Postgres; share `CUSTOMER_ID`; unique `INSTALL_ID` / `INSTALL_ROLE` ([multi-env-deploy.md](../multi-env-deploy.md)).
2. **Register peers** with `baseUrl` on each side (`POST /deploy/v1/peers`) so promote trust (allowlist) and IDE peer hints work.
3. **In Control IDE** — Env switcher **Add environment…** or peer **Connect…** (URL prefilled when the peer row has `baseUrl`). Sign in **once per install** (each install issues its own JWT).

### Explicit non-goals (this plan)

- Helm / Marketplace seeding three environments by default
- “Connect once → all sibling envs fully authenticated”
- Cross-install SSO / shared session tokens
- Using DigitalOcean (or other cloud) OAuth to discover Majesta One API URLs (BP-027 remains resize-only)
- Embedding Control IDE in product images

Closing cross-install “one login unlocks all” would be a **later, larger** AuthZ + Deploy build (principal sync and/or token exchange across peer issuers). Track separately when multi-env customers demand it.

---

## Connect troubleshooting (API revision)

Control IDE and `one` **pin** `One-API-Revision` after probing unauthenticated `GET /version` ([ADR-025](../adr/025-api-revision-versioning.md)). Product semver skew is a **warning** only.

| Symptom | Cause | What to do |
|---|---|---|
| Connect blocked `API_REVISION_UNSUPPORTED` (pin &lt; min) | Install raised `API_REVISION_MIN` | Update Control IDE / migrate the pin in Settings |
| Connect blocked `API_REVISION_UNSUPPORTED` (pin &gt; current) | IDE newer than install wire | Upgrade the install image (`/ops/v1`) or lower the pin |
| Connect blocked `INSTALL_REVISION_TOO_OLD` | IDE `minApiRevision` &gt; install `current` | Upgrade the install |
| Banner `PRODUCT_OUTSIDE_TESTED` | Product minor outside the IDE test matrix | Soft — Connect is allowed; upgrade when convenient |
| `400` on family calls with `cta` in the body | Header or `/r{N}/` pin outside `[min,current]` | Match `GET /version` → `apiRevision` |
| Break-glass | Ops debugging | IDE “Connect anyway”; CLI `--force-compat` (exit 3 otherwise) |

Self-host env: `API_REVISION_CURRENT` and `API_REVISION_MIN` on every image ([self-host.md](../self-host.md)).

---

## Done when

- [x] This plan exists and is linked from [architecture README](./README.md)
- [x] Default topology language is **one Prod** in distribution + self-host + multi-env lead-in
- [x] DO + other-cloud first-admin paths are written (bootstrap → human SystemAdmin → Connect)
- [x] Optional multi-env path documented; cross-install SSO / auto-fill deferred
- [x] Helm NOTES include IDE URL + bootstrap-admin handoff

---

## Related

- [self-host.md](../self-host.md) — operator install
- [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) — packaging / marketplace strategy
- [customer-ide-ux.md](../customer-ide-ux.md) — post-connect IDE modes
- [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md) — social / exchange shipping status
- [ADR-030](../adr/030-install-agent-runtime.md) — optional org IDE license after Connect; seats via `ide.*`
