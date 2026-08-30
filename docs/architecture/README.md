# Architecture docs index

Start here for product architecture. Majesta One is API-first, dedicated install (one customer per database), metadata-driven. **Alpha `0.1.0`** — not a CRM; DigitalOcean is the first targeted managed path.

Vendor/agent guidance in this tree (`docs/architecture/`, plus `docs/`, `backlog/`, `.cursor/`) does **not** ship in product images — see [monorepo.md](../monorepo.md).

## Orient first (all agents)

1. **[Glossary](../glossary.md)** — Majesta One, customer, install, org, custom vs managed.
2. **[Module map](./module-map.md)** — concern → packages → key files.
3. **[Agent routing](./agent-routing.md)** — which playbook and domain agent to use.
4. **[Platform overview](../architecture.md)** — runtime components, API families, storage summary.
5. **[Confirmed stack](../tech-stack.md)** — do not invent a parallel stack.
6. **[Backlog](../../backlog/README.md)** — open risks before large changes.

## Domain playbooks

| Playbook | Use when |
|---|---|
| [agent-data-architecture.md](./agent-data-architecture.md) | Data model, seed, storage, query |
| [agent-authz.md](./agent-authz.md) | JWT, scopes, Roles, permission sets, principals |
| [agent-api-families.md](./agent-api-families.md) | Client / Metadata / Deploy / Ops HTTP ownership |
| [agent-deploy.md](./agent-deploy.md) | Promote, peers, multi-env, Ops rolls, packaging |
| [agent-worker.md](./agent-worker.md) | Jobs, outbox, webhook delivery, worker loop |
| [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) | **Shipped (BP-064):** install as agent runtime — job-class harness, builder MCP, skills, hosted loop ([ADR-030](../adr/030-install-agent-runtime.md) mitigated). Remaining install cleanup: [BP-065](../../backlog/BP-065-ide-backend-coupling.md) |
| [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) | **Shipped:** hosted `/client/v1/agents/runs` executes MCP tools as the run actor ([BP-006](../../backlog/BP-006-agent-guardrails.md) mitigated) |
| [agent-control-ide.md](./agent-control-ide.md) | Control IDE (Electron / React / Vitest under `tools/control-ide`) — optional client; **refactor for install cleanup** ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)); no new Electron-only product chrome |
| [agent-public-docs.md](./agent-public-docs.md) | GitHub product docs in this repo; public host is a **separate CMS aggregator** ([BP-067](../../backlog/BP-067-public-docs-site.md)) |

## Read in this order (data architecture agents)

1. **[This folder’s README](./README.md)** (you are here) — map of docs and rules of engagement.
2. **[Module map](./module-map.md)** + **[data playbook](./agent-data-architecture.md)**.
3. **[Core data model](../data-model.md)** — User / Account / Contact, immutability, performance, deferred extensions.
4. **[ADR-008: Core data model](../adr/008-core-data-model.md)** — decisions locked for the managed `core` package.
5. **[Sales & Service data model](./sales-service-data-model.md)** + **[ADR-011](../adr/011-sales-service-managed-modules.md)** + **[ADR-020](../adr/020-cdm-managed-packages.md)** — optional catalog / sales / service / bridge + domain packs.
6. **[Domain attribute mapping](./cdm-mapping.md)** + **[Industry packages](./cdm-industry-packages.md)** — curated field map and industry vertical packs (when changing managed domain defs).
7. **[ADR-009: Record audit + AuthZ packaging](../adr/009-record-audit-authz-packaging.md)** — optional OwnerId, CreatedBy/LastModifiedBy, Role scopes / PS grants.
8. **[ADR-002: Hybrid metadata storage](../adr/002-hybrid-metadata-storage.md)** — kernel DDL vs `records.data` JSONB.
9. **[ADR-003: SQL query engine](../adr/003-sql-query-engine.md)** — Go + Postgres query/index strategy.
10. **[ADR-013: High-volume flexible storage](../adr/013-high-volume-flexible-storage.md)** — shared `records` risks; `storage_mode=high_volume`; LIST/RANGE partitioning ladder. Product `messages` example retired ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md)).
11. **[Customer customizations](../customer-customizations.md)** — managed vs customer; never commit customer metadata into product paths.
12. **[API families / ADR-004](../api-families.md)** — Client / Metadata / Deploy ownership boundaries.
13. **[Backlog](../../backlog/README.md)** — open BP-001 / BP-035 / BP-049 follow-ups before large data-model changes.

## Document map

| Doc | Purpose |
|---|---|
| [glossary.md](../glossary.md) | Customer / install / org / custom vs managed vocabulary |
| [module-map.md](./module-map.md) | Concern → code packages (agent router) |
| [agent-routing.md](./agent-routing.md) | Spawn/focus subagents correctly |
| [architecture.md](../architecture.md) | Product architecture overview |
| [data-model.md](../data-model.md) | **Canonical** core data model + agent rules for changing it |
| [sales-service-data-model.md](./sales-service-data-model.md) | Optional Sales / Service / catalog / bridge architecture |
| [agent-data-architecture.md](./agent-data-architecture.md) | Playbook: data / seed / query |
| [agent-authz.md](./agent-authz.md) | Playbook: AuthN / AuthZ |
| [customization-authz.md](./customization-authz.md) | Principal parity + system capabilities (BP-006 AuthZ — **shipped**; loop is [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md)) |
| [identity-directory-productionization.md](./identity-directory-productionization.md) | Plan: users / Roles / PS assign productionization (BP-017 Phases 1–4 shipped) |
| [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md) | **Shipped:** customer-extendable User + JIT/SCIM provisioning config (BP-058 mitigated) |
| [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md) | Plan: social login broker (Google/Apple), IdP exchange adapters (ADR-015 / BP-013) |
| [install-claim-sso-build-plan.md](./install-claim-sso-build-plan.md) | **Active:** install claim + customer SSO + JIT (ADR-015 amend / BP-037) |
| [refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) | **Active:** opaque refresh tokens + Control IDE silent re-auth (ADR-006 amend / BP-063) |
| [system-alerts-byo-build-plan.md](./system-alerts-byo-build-plan.md) | **Active:** no product mailer; password recovery + BYO webhook intents (BP-038) |
| [auth-adapters.md](../auth-adapters.md) | Operator runbooks: Okta / Entra / Keycloak / Cognito → token exchange; Okta/Entra SCIM UserCustom |
| [scim-provisioning.md](./scim-provisioning.md) | SCIM 2.0 `/scim/v2` connector adapter (BP-017 Users + R1 Groups-as-tags; UserCustom on BP-058 mitigated; bulk/filters remain) |
| [customer-connect.md](../customer-connect.md) | Customer connect paths: UI JWT · service accounts · MCP |
| [customer-automations-build.md](./customer-automations-build.md) | Plan: Deno guest TS automations, PS grants, sync rollback (ADR-014 / BP-009) |
| [platform-actions-build-plan.md](./platform-actions-build-plan.md) | **Active (Phases 1–4 shipped; quote.accept on BP-044):** package-gated Client verbs + guest `invokeAction` (ADR-029 / BP-061) |
| [billing-module-build-plan.md](./billing-module-build-plan.md) | **Active:** optional `billing` Order/OrderLine + `quote.accept` (ADR-031 / BP-044) |
| [outbound-otel-build-plan.md](./outbound-otel-build-plan.md) | **Active:** operator OTEL (BP-008) + customer connectors/secret refs/egress + agent skills (BP-014) |
| [integrations-build-plan.md](./integrations-build-plan.md) | **Active:** Client-callable automations + outbound connector OAuth (BP-047) |
| [external-id-upsert-bulk-build-plan.md](./external-id-upsert-bulk-build-plan.md) | **Active:** external ID metadata, REST upsert, Bulk ingest jobs, data packs (BP-041) |
| [cross-object-search-build-plan.md](./cross-object-search-build-plan.md) | **Mitigated:** Client indexed search (`POST /client/v1/search`) — [BP-043](../../backlog/BP-043-cross-object-search-api.md) |
| [customer-runtime-isolation-build-plan.md](./customer-runtime-isolation-build-plan.md) | **Active:** admission lanes, execution budgets, ExecutionRun/LogEntry debug (BP-033) |
| [client-experience-build-plan.md](./client-experience-build-plan.md) | **Active:** Client Experience — OSS `sdk/client/` kits, Connected Apps defaults (ADR-019 / BP-040); Phases 1–6 landed |
| [cdm-mapping.md](./cdm-mapping.md) | Domain attribute map (ADR-020 / BP-049) |
| [cdm-industry-packages.md](./cdm-industry-packages.md) | Curated industry managed packs customers can enable |
| [inference-build-plan.md](./inference-build-plan.md) | **Active:** BYO + Native DO Inference + agent run SSE streaming (BP-052) |
| [agent-section-harness-build-plan.md](./agent-section-harness-build-plan.md) | **Shipped:** Section harness + Build Agents create wizard (BP-053). Job-class follow-on: [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) |
| [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) | **Shipped (BP-064):** Install as agent runtime — job-class harness, builder MCP, builder skills, freeze vs finish ([ADR-030](../adr/030-install-agent-runtime.md) mitigated). Remaining: [BP-065](../../backlog/BP-065-ide-backend-coupling.md) |
| [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) | **Review:** Control IDE-shaped AuthN, chrome Client routes, `ide.*` caps, and agent coaching on the Go install — **remove with IDE lockstep** ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)) |
| [ide-demo-client-uplift-build-plan.md](./ide-demo-client-uplift-build-plan.md) | **Active:** Control IDE as an honest JWT demo of shipped family APIs (stubs, hosted loop consume, thin admin/builder gaps) — [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md) |
| [agentic-remainders/](./agentic-remainders/README.md) | **Finish work-order remainder designs** + paste-ready build prompts (slots 1–12: BP-065 → CLI → skills → inference → Experience → claim/SSO → directory → isolation → install/distro → API revision → headless Client → OTEL/OSS/automations) |
| [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) | **Shipped:** Hosted agent tool loop — MCP names, `internal/agentloop`, write parking ([BP-006](../../backlog/BP-006-agent-guardrails.md) mitigated; AuthZ already in [customization-authz.md](./customization-authz.md)) |
| [builder-connect.md](../builder-connect.md) | Builder MCP + CLI connect recipes (no Control IDE required) |
| [self-host.md](../self-host.md) | Dual-path install: Path A App Platform / Path B Compose+Helm; community `sdk/` pointer |
| [install-ide-connect-build-plan.md](./install-ide-connect-build-plan.md) | **Active:** single-Prod default → Control IDE connect / first admin (App Platform happy path); multi-env opt-in |
| [ide-api-version-compatibility-build-plan.md](./ide-api-version-compatibility-build-plan.md) | **Implemented:** API revision pin (`One-API-Revision` + `/r{N}/`) + soft product tested-against window ([ADR-025](../adr/025-api-revision-versioning.md) / BP-025 Mitigated) |
| [customer-repo-init-build-plan.md](./customer-repo-init-build-plan.md) | **Active:** initialize customer Git from prod + IDE clone/pull sync (`one/v1`) |
| [customer-dx-build-plan.md](./customer-dx-build-plan.md) | **Active:** repo→org DX — validate-first, then deploy; no peer-to-peer promote in the supported path |
| [one-cli-build-plan.md](./one-cli-build-plan.md) | **Active:** productize `one` CLI — **Ship path of record** for builders (auth, project init, selective deploy, release assets; BP-048). IDE Ship is a frozen twin. |
| [do-app-platform-deploy-api-build-plan.md](./do-app-platform-deploy-api-build-plan.md) | App Platform 1-Click packaging → Deploy API DO cloud (IDE Govern UI is frozen) |
| [deploy-cloud-agnostic-build-plan.md](./deploy-cloud-agnostic-build-plan.md) | **Active:** host-free `/deploy/v1/cloud/*` + `CloudHost` port; DO aliases; community AWS skeleton |
| [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) | **Contract:** host-free Deploy cloud verbs; DO product first; AWS managed profile = community |
| [digitalocean-distribution-build-plan.md](./digitalocean-distribution-build-plan.md) | Strategy: Path A+B + role matrix + community `sdk/`; Marketplace = BP-028 deferred; Deploy API = BP-030 |
| [public-docs-site.md](./public-docs-site.md) | Pointer: `one.majesta.net` is a **separate CMS aggregator**; do not implement the publisher here |
| [public-docs-site-build-plan.md](./public-docs-site-build-plan.md) | **Superseded** in this repo — no in-tree phases ([BP-067](../../backlog/BP-067-public-docs-site.md)) |
| [agent-public-docs.md](./agent-public-docs.md) | Playbook: GitHub `docs/` only; no Astro/Netlify/CMS dependency |
| [agent-api-families.md](./agent-api-families.md) | Playbook: HTTP API families |
| [agent-deploy.md](./agent-deploy.md) | Playbook: Deploy / Ops / packaging |
| [agent-worker.md](./agent-worker.md) | Playbook: worker / jobs / outbox |
| [agent-control-ide.md](./agent-control-ide.md) | Playbook: Control IDE vendor client |
| [control-ide-security-audit.md](./control-ide-security-audit.md) | Control IDE threat model, findings register, and remediation phases |
| [local-development-mac.md](../local-development-mac.md) | Mac local: API + Postgres + Control IDE |
| [tech-stack.md](../tech-stack.md) | Confirmed libraries and non-goals |
| [adr/](../adr/) | Architecture decision records ([index](../adr/README.md)) |
| [customer-customizations.md](../customer-customizations.md) | Product vs customer boundary |
| [customer-developer-workflow.md](../customer-developer-workflow.md) | Best-practice customer DX loop (Git host agnostic; validate → deploy) |
| [ops.md](../ops.md) / [product-upgrades.md](../product-upgrades.md) | How managed packages ride image upgrades |
| [sdk/aws/docs/managed-paas-profile.md](../../sdk/aws/docs/managed-paas-profile.md) | Community AWS managed PaaS analog (opinionated ECS Fargate) — not Path A |
| [sdk/aws/docs/managed-channel.md](../../sdk/aws/docs/managed-channel.md) | Community / non-GA managed-cell notes (not product channel) |
| [sdk/aws/docs/managed-channel-security.md](../../sdk/aws/docs/managed-channel-security.md) | Community horizontal-traversal notes |
| [sdk/aws/README.md](../../sdk/aws/README.md) | Community AWS SDK (optional Path B) |
| [release-cicd.md](../release-cicd.md) | Product publish vs optional channel rolls |

## Hard rules (summary)

- **Core package** = `User` (kernel) + `Account` + `Contact` (flexible managed). Nothing else is always-on seed.
- **Optional Sales/Service/Billing** = `catalog` / `sales` / `service` / `crm_bridge` / `billing` per [ADR-011](../adr/011-sales-service-managed-modules.md) / [ADR-031](../adr/031-billing-managed-module.md); enable via Metadata only (no customer DDL).
- **Do not** put Lead, OpportunityLineItem, Order, or sales Contract into `sales` v1; keep `catalog` free of CPQ objects. Order ships in optional `billing`.
- **Do not** reintroduce legacy CRM objects into always-on `core` seed.
- **Account↔Contact** is optional; customers may use one or both.
- Opportunity (`sales`): must link **Contact or Account** (at least one).
- Custom fields = Metadata API + JSONB — never customer DDL; never put customer schema in `internal/seed` or `migrations/`.
- Managed metadata ships via product image + `InstallCore` / package enable; Deploy promotes **customer** artifacts only.
- Prefer `/client/v1`, `/metadata/v1`, `/deploy/v1`, `/ops/v1`; keep `/v1` aliases for compatibility only.
- Integrity verbs (Lead convert, merge, Quote accept) are **platform actions** on `/client/v1/actions/{apiName}` ([ADR-029](../adr/029-platform-actions.md)) — not per-verb routes, not locked package TypeScript. Guest TS calls `ctx.invokeAction` only.
