# DigitalOcean App Platform + Deploy API — active build plan

**Active execution plan** for the next distribution waves. Strategy context stays in [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md). Host-agnostic day-2 verbs: [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) (DO is the product adapter; AWS managed profile stays community).

**Priority (locked):**

1. **Get App Platform 1-Click / Marketplace-ready packaging in shape** (manual App Spec first; live listing still [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) deferred on vendor account).
2. **Design and build Deploy API endpoints** that manage, scale, and provision DigitalOcean App Platform Majesta One installs (install-local DO credentials — not a vendor fleet plane). Implement the [cloud capability contract](./deploy-cloud-capability-contract.md) verbs under `/deploy/v1/cloud/digitalocean/*`.
3. **Kubernetes / multi-cloud Helm enhancements** — backlog only (chart already works; no new K8s 1-Click engineering until BP-028).
4. **Control IDE DO uplift** — frozen ([BP-027](../adr/030-install-agent-runtime.md)); IDE calls these Deploy APIs as a JWT client.
5. **Non-DO managed PaaS adapters** — community `sdk/` only (e.g. [AWS managed profile](../../sdk/aws/docs/managed-paas-profile.md)); **not** a second product Path A in this plan.

**Playbooks:** [agent-deploy.md](./agent-deploy.md) · [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md)  
**Domain agents:** `deploy-ops` (packaging + Deploy/Ops Go); `api-families` when adding routes; `authz-security` for scope/`+admin` gates. Do **not** spawn `control-ide` for this plan.  
**Backlog:** [BP-029](../../backlog/BP-029-app-platform-install.md) · [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md) (deferred) · [BP-027](../adr/030-install-agent-runtime.md) (IDE Govern — frozen) · [BP-011](../../backlog/BP-011-container-marketplace-fargate.md)

---

## Thesis

> Customers install Majesta One on **DigitalOcean App Platform** with pinned GHCR digests + Managed PostgreSQL. Day-2 environment lifecycle (provision peer App installs, scale app instances, resize Postgres) is owned by the **Deploy API on that install**, using a customer-supplied DigitalOcean token. Marketplace 1-Click wraps the same App Spec when the vendor account exists. The IDE stays a client of Deploy/Ops — it does not become the DO control plane.

```text
Customer DO account
  └── App Platform (prod)  ←── doctl / Marketplace 1-Click (BP-029 / BP-028)
        └── Majesta One API (Deploy scope + admin)
              └── DIGITALOCEAN_* token (install-local)
                    ├── provision peer App + Managed DB
                    ├── scale instances / resize DB
                    └── register peer in /deploy/v1/peers
  Control IDE (BP-027 — frozen) ──JWT──► same Deploy API
```

---

## Product decisions (locked for this plan)

| Decision | Choice | Rationale |
|---|---|---|
| Default install shape | App Platform + Managed PostgreSQL 16+ | Lowest friction; matches distribution strategy |
| 1-Click near-term | App Spec + docs + `doctl` path ready; **Marketplace publish deferred** | No Vendor Portal yet — still ship installable packaging |
| Day-2 cloud ops surface | **`/deploy/v1/cloud/digitalocean/*`** | Environment lifecycle + peer provisioning belong with Deploy, not IDE-direct DO calls |
| Product image upgrade | Still **`/ops/v1/upgrades`** (ADR-007) | Digest redeploy of *this* install stays Ops; App Platform roller is an Ops backend later |
| DO credentials | Install-local env/secret (`DIGITALOCEAN_API_TOKEN` or scoped PAT) | ADR-001 — not a One-hosted multi-tenant credential store |
| Billing | Always the **customer** DO account | Majesta One does not meter DO infra. Control IDE seats are **Stripe** (vendor issuer) — [ADR-030](../adr/030-install-agent-runtime.md) |
| K8s enhancements | **Backlog** | Helm path already shipped; no DOKS 1-Click work until BP-028 |
| IDE DO UI / OAuth | **Frozen** (BP-027) | Consumes Deploy API; optional OAuth helper later |

---

## Family ownership (ADR-004 / ADR-007 fence)

| Concern | Family | Notes |
|---|---|---|
| Customer metadata promote / peers / tests | Deploy (existing) | Unchanged |
| Link DO app/DB ids to this install; provision **peer** App+DB; scale app/DB | **Deploy** (new cloud surface) | [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) |
| Confirm / roll **this** install’s product digests; upgrade ledger; rollback | **Ops** | App Platform driver under `internal/ops` (or shared DO client) — not `/deploy/v1/promotions` |
| IDE chrome for the above | Frozen BP-027 | JWT client only |

**Do not** put multi-customer DO fleet routes in `cmd/api`. The token on install A may create App B in the **same customer DO team**; Majesta One still has no cross-customer control plane.

---

## Wave A — App Platform 1-Click readiness ([BP-029](../../backlog/BP-029-app-platform-install.md))

**Goal:** An operator (or future Marketplace) can create a production-shaped Majesta One App from repo artifacts without inventing YAML.

| # | Work | Area | Done when |
|---|---|---|---|
| A1 | Checked-in App Spec file(s) under `deploy/digitalocean/` (`app.yaml` or equivalent) — api service + worker; digest placeholders; Managed DB guidance | `deploy/digitalocean/` | Spec validates with `doctl apps spec validate` (or documented equivalent) |
| A2 | Operator runbook: create Managed Postgres → set secrets → `doctl apps create --spec` → get HTTPS URL | [self-host.md](../self-host.md) | Copy-paste path works without K8s knowledge |
| A3 | Secrets / identity checklist: `DATABASE_URL`, `API_KEYS`, `AUTH_JWT_*`, `CUSTOMER_ID`, `INSTALL_ID`, `INSTALL_ROLE`, `PLATFORM_PUBLIC_URL` | docs + App Spec comments | Matches [install-ide-connect-build-plan.md](./install-ide-connect-build-plan.md) |
| A4 | Release hygiene: document how `v*` GHCR digests from release assets replace tags in the App Spec (no `:latest`) | `deploy/digitalocean/`, release notes | Digests file → App Spec mapping is explicit |
| A5 | Optional CI lint: schema/validate App Spec on PR when `deploy/digitalocean/**` changes | `.github/workflows/` | Fail closed on invalid spec |
| A6 | Marketplace **prep only** (listing copy draft, screenshots checklist, digest pin rules) — **do not publish** | `deploy/digitalocean/` + BP-028 notes | Ready to submit when Vendor Portal unblocks |

**Explicit non-goals for Wave A:** Vendor Portal submit; Droplet 1-Click; Helm/DOKS feature work; Deploy API code; IDE UI.

---

## Wave B — Deploy API: DigitalOcean App management ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md))

**Goal:** A Prod (or bootstrap) Majesta One install can manage DO App Platform resources for **this customer’s** installs via Deploy scope + admin.

### B0 — Design lock (docs before code)

| Topic | Decision to record |
|---|---|
| Credential model | Env `DIGITALOCEAN_API_TOKEN` (MVP); optional later encrypted store via Metadata/admin — never in product images |
| Resource binding | Persist DO `app_id` / `database_id` / region on this install (DB table or config) so scale/ops cannot target arbitrary DO apps |
| Peer provision | Creates new App + Managed DB with shared `CUSTOMER_ID`, new `INSTALL_ID` / `INSTALL_ROLE`; returns API URL; upserts `/deploy/v1/peers` |
| AuthZ | Scope `deploy` required; mutating cloud routes require `+admin` (same pattern as sensitive Deploy/Ops writes) |
| Errors | Map DO 401/403/429 to Majesta One problem responses; never echo token |
| ADR touch | Short note in [api-families.md](../api-families.md) Deploy section + BP-010 follow-up; ADR-007 unchanged (product rolls stay Ops) |

### B1 — Shared DO client (Go)

| Work | Area |
|---|---|
| Thin Apps + Databases API client (create app from spec, get app, update/scale, resize DB, create deployment) | `internal/digitalocean/` (or `internal/deploy/digitalocean/`) |
| Unit tests with httptest mock; no live DO calls in CI | `*_test.go` |
| Config from env; disabled when token absent (`capabilities` on `GET /deploy/v1/environment`) | wire through `cmd/api` |

### B2 — Deploy HTTP surface (target contract)

Prefix: `/deploy/v1/cloud/digitalocean` (names may refine in B0; keep under Deploy family).

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/status` | Token configured?; linked `appId` / `databaseId`; DO account reachability (masked) |
| `PUT` | `/binding` | Bind this install to existing DO app + database ids (tag/ownership check) |
| `GET` | `/app` | Live App Platform summary for the **bound** app (instances, region, digests) |
| `PATCH` | `/app/scale` | `instance_count` / `instance_size_slug` for api and/or worker |
| `PATCH` | `/database/resize` | Managed Postgres size / `num_nodes` (HA standby) |
| `POST` | `/environments` | Provision peer: App Spec + Managed DB + secrets bootstrap + peer registration |
| `GET` | `/environments` | List peer envs this install knows about (peers ∪ DO-linked) |
| `POST` | `/app/redeploy` | Optional thin helper: push new digests to **bound** app — **or** defer entirely to Ops App Platform roller |

**Ops coordination:** Prefer implementing product upgrade confirm/roll for App Platform under `/ops/v1` once a DO roller exists. If Wave B ships `POST …/app/redeploy`, document it as a temporary operator helper and migrate to Ops in a follow-up so ADR-007 stays clean.

### B3 — Persistence + trust

| Work | Notes |
|---|---|
| Migration for cloud binding / provision audit (install-local) | No `customer_id` column — one DB = one install |
| Provision writes peer row compatible with existing trust (`CUSTOMER_ID`, optional allowlist + HMAC) | Reuse `internal/deploy` peer APIs |
| Reject provision if token missing or binding points outside tagged Majesta One resources | Defense in depth vs runaway DO spend |

### B4 — Docs + tests

| Work | Area |
|---|---|
| Document cloud routes in [api-families.md](../api-families.md) | Deploy section |
| Self-host: “enable DO management token” | [self-host.md](../self-host.md) |
| Integration tests via `internal/testutil` + mocked DO HTTP | Prefer no live DO in CI |

---

## Wave C — Marketplace publish (deferred)

Tracked only in [BP-028](../../backlog/BP-028-digitalocean-marketplace-listing.md). Depends on Wave A artifacts + Vendor Portal. App Platform–first listing; optional K8s 1-Click remains backlog packaging, not active engineering.

---

## Explicit backlog (do not schedule in this plan)

| Item | Why deferred |
|---|---|
| Helm/DOKS feature enhancements, EKS/AKS deep network guides beyond current self-host notes | Power path already usable; not the 1-Click default |
| Kubernetes Marketplace 1-Click engineering | BP-028 optional track after App Platform listing |
| Control IDE “Connect DigitalOcean” UI, OAuth, provision/scale panels | [BP-027](../adr/030-install-agent-runtime.md) — **frozen** ([ADR-030](../adr/030-install-agent-runtime.md)) |
| AWS/GCP/Azure cloud provisioners | Out of scope for v1 DO focus |
| Droplet 1-Click / AMI | Non-goal |
| Vendor managed-subscription fleets | ADR-001 / BP-002 |

---

## Suggested sequencing

```text
Wave A (BP-029)  App Spec + doctl runbook + digest mapping + Marketplace prep notes
       │
       ▼
Wave B0–B4 (BP-030)  Deploy API DO client + binding + scale + provision peer
       │
       ├──► Ops App Platform roller (product upgrades) — follow-up under BP-002 / ADR-007
       │
       └──► BP-028 when Vendor Portal ready (uses Wave A artifacts)
              BP-027 IDE Govern frozen (ADR-030)
```

---

## Checklist — Wave A done

- [x] `deploy/digitalocean/app.yaml` (or equivalent) checked in and validated  
- [x] Self-host App Platform section is the default happy path  
- [x] Digest pin instructions from GHCR release assets  
- [x] BP-028 still deferred (prep notes only)  
- [x] No IDE / K8s enhancement scope creep  

## Checklist — Wave B done

- [x] `GET /deploy/v1/environment` advertises `digitaloceanCloud: true|false`  
- [x] Binding + scale + resize + provision peer routes shipped with tests  
- [x] Scope `deploy` + `+admin` on mutating cloud routes  
- [x] Peers registered on provision; same-`CUSTOMER_ID` trust preserved  
- [x] api-families + self-host updated; BP-030 status → Partially mitigated  
- [x] BP-027 Frozen — IDE Govern is not a product track  

---

## Related

- [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) — strategy  
- [install-ide-connect-build-plan.md](./install-ide-connect-build-plan.md) — first admin / Connect  
- [api-families.md](../api-families.md) · [ADR-004](../adr/004-three-api-families.md) · [ADR-007](../adr/007-platform-ops-upgrades.md)  
- [self-host.md](../self-host.md) · [multi-env-deploy.md](../multi-env-deploy.md)  
- DO [App Spec](https://docs.digitalocean.com/products/app-platform/reference/app-spec/) · [Apps API](https://docs.digitalocean.com/products/app-platform/reference/api/)
