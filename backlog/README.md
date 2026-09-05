# Backlog — foreseeable problems

Known structural risks relative to the Majesta One product plan: a **secure open-source CRM backend**; **dual-path install** — **Path A** DigitalOcean App Platform (**only** product managed PaaS) + **Path B** self-install from image (Compose + Helm); **Deploy API** for day-2 cloud ops via host-agnostic verbs ([deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md); DO adapter [BP-030](./BP-030-deploy-api-digitalocean-apps.md)); Marketplace publish deferred ([BP-028](./BP-028-digitalocean-marketplace-listing.md)); **install as agent runtime** ([ADR-030](../docs/adr/030-install-agent-runtime.md) / [BP-064](./BP-064-install-agent-runtime.md)) — MCP + `one` are builders; Control IDE chrome is **frozen**; community cloud SDKs under [`sdk/`](../sdk/README.md) are optional Path B extensions (**not** product GA).

**How to use (agents):** Read this file first, then open individual items before proposing work. Prefer high-severity Finish items when the task is open-ended. When you resolve or de-risk an item, update its status and this table.

Each BP header includes an **`Area:`** field — preferred code scope. Resolve Area → packages via [docs/architecture/module-map.md](../docs/architecture/module-map.md). Pick the playbook and domain agent from [docs/architecture/agent-routing.md](../docs/architecture/agent-routing.md).

**Remainder designs** (paste-ready Finish prompts): [docs/architecture/agentic-remainders/](../docs/architecture/agentic-remainders/README.md). Start at slot 1 ([BP-065](./BP-065-ide-backend-coupling.md)).

| ID | Severity | Status | Title |
|---|---|---|---|
| [BP-001](./BP-001-jsonb-query-scale.md) | High | Mitigated | JSONB query path — follow-on [BP-035](./BP-035-records-list-partition-covering.md) |
| [BP-002](./BP-002-dedicated-install-fleet-ops.md) | High | Partially mitigated | Dedicated install local upgrades (self-host) |
| [BP-003](./BP-003-enterprise-auth.md) | High | Mitigated | Enterprise AuthZ (sharing + deny-by-default FLS) |
| [BP-006](./BP-006-agent-guardrails.md) | High | Mitigated | Principal AuthZ + hosted `/agents/runs` loop |
| [BP-008](./BP-008-production-packaging.md) | Medium | Partially mitigated | Observability (OTEL) gaps — optional logs exporter shipped |
| [BP-009](./BP-009-no-in-kernel-language.md) | Medium | Mitigated | Customer code automations (Deno guest TS Phases 0–7; ExecutionRun debug stays BP-033) |
| [BP-010](./BP-010-three-api-families.md) | High | Mitigated | Three API families (Client / Metadata / Deploy) |
| [BP-011](./BP-011-container-marketplace-fargate.md) | High | Partially mitigated | OSS images; App Platform default; Helm multi-cloud |
| [BP-013](./BP-013-jwt-unified-principals.md) | High | Partially mitigated | Majesta One JWT issuer + unified principals |
| [BP-014](./BP-014-agent-outbound-integrations.md) | Medium | Partially mitigated | Customer outbound integrations — **Keep** `invoke_skill`; deferred OAuth/ExecutionRun |
| [BP-017](./BP-017-identity-directory-productionization.md) | High | Partially mitigated | Identity directory (SCIM Users + Groups-as-tags shipped; bulk/filters remain) |
| [BP-022](./BP-022-client-access-ide-device.md) | High | Partially mitigated | Client access mode / `azp` / Connected Apps (device/mTLS frozen) |
| [BP-025](./BP-025-ide-api-version-compatibility.md) | High | Mitigated | API revision pin + soft tested product window — pin gaps remain |
| [BP-026](./BP-026-oss-security-public-backlog.md) | Medium | Partially mitigated | OSS security process & public backlog hygiene |
| [BP-028](./BP-028-digitalocean-marketplace-listing.md) | Medium | Open (deferred) | DO Marketplace (App Platform first + optional K8s 1-Click) |
| [BP-029](./BP-029-app-platform-install.md) | High | Partially mitigated | DigitalOcean App Platform install path |
| [BP-030](./BP-030-deploy-api-digitalocean-apps.md) | High | Partially mitigated | Deploy API — DO App Platform manage / scale / provision |
| [BP-031](./BP-031-customer-repo-init-sync.md) | Medium | Partially mitigated | Customer repo initialize from prod |
| [BP-032](./BP-032-customer-dx-validate-deploy.md) | Medium | Mitigated | Repo→org validate/deploy (GitHub-agnostic) |
| [BP-033](./BP-033-customer-runtime-isolation.md) | High | Partially mitigated | Admission + async Deploy shipped; execution budgets / ExecutionRun remain |
| [BP-035](./BP-035-records-list-partition-covering.md) | High | Partially mitigated | LIST-partition `records` + covering projections |
| [BP-036](./BP-036-canonical-field-types.md) | High | Mitigated | Canonical field types |
| [BP-037](./BP-037-install-claim-customer-sso.md) | High | Partially mitigated | Install claim, customer SSO, and JIT |
| [BP-038](./BP-038-no-product-mailer-byo-alerts.md) | Medium | Partially mitigated | No product mailer; BYO webhooks/connectors |
| [BP-040](./BP-040-client-experience-oss-kits.md) | High | Partially mitigated | Client Experience + OSS client kits |
| [BP-041](./BP-041-record-external-id-upsert-bulk.md) | High | Mitigated | External ID, upsert, Bulk jobs, data packs |
| [BP-042](./BP-042-change-feed-cdc-consumer.md) | High | Open | Change feed / CDC consumer contract |
| [BP-043](./BP-043-cross-object-search-api.md) | High | Mitigated | Cross-object indexed search (Client API) |
| [BP-044](./BP-044-billing-module-order-from-quote.md) | High | Mitigated | Billing module — Order from accepted Quote |
| [BP-045](./BP-045-files-content-storage.md) | Medium | Open | Files / content storage |
| [BP-046](./BP-046-record-merge-dedupe.md) | Medium | Open | Record merge and duplicate detection |
| [BP-047](./BP-047-integrations-callable-oauth.md) | Medium | Partially mitigated | Client-callable automations + connector OAuth |
| [BP-048](./BP-048-one-cli.md) | Medium | Partially mitigated | `one` CLI productization (Ship of record) |
| [BP-049](./BP-049-cdm-managed-packages.md) | Medium | Partially mitigated | Managed domain packages |
| [BP-050](./BP-050-run-mode-toolspec.md) | High | Mitigated | Control IDE Run mode + ToolSpec (no chrome expansion) |
| [BP-052](./BP-052-customer-inference.md) | High | Partially mitigated | Customer inference — BYO + DO native; SSE reconnect/cancel remains |
| [BP-053](./BP-053-agent-section-harness.md) | High | Mitigated | Agent section harness (compat; job class is BP-064) |
| [BP-054](./BP-054-agent-conversation-preferences.md) | High | Mitigated | Agent conversations + preferences |
| [BP-055](./BP-055-run-personal-graph.md) | High | Mitigated | Run personal graph (refs-only) |
| [BP-056](./BP-056-run-graph-crm-interactions.md) | High | Mitigated | Run graph Pin / Wire / Apply |
| [BP-058](./BP-058-user-identity-extension.md) | High | Mitigated | Customer-extendable User + JIT/SCIM config |
| [BP-060](./BP-060-operate-graph-surface.md) | High | Mitigated | Operate graph surface (no chrome expansion) |
| [BP-061](./BP-061-platform-actions.md) | High | Partially mitigated | Platform actions; `record.merge` remains |
| [BP-063](./BP-063-refresh-token-sessions.md) | High | Partially mitigated | Opaque refresh tokens |
| [BP-064](./BP-064-install-agent-runtime.md) | High | Mitigated | Install as agent runtime |
| [BP-065](./BP-065-ide-backend-coupling.md) | Medium | Partially mitigated | Neutralize Control IDE coupling on the Go install (Phase 1 AuthN landed; Phases 2–4 remain) |
| [BP-066](./BP-066-ide-demo-client-fidelity.md) | Medium | Open | Control IDE as an honest JWT demo of shipped APIs |
| [BP-067](./BP-067-public-docs-site.md) | Medium | Open (CMS extracted) | Public docs site (`one.majesta.net`) — separate CMS aggregator; this repo does not implement |
| [BP-068](./BP-068-ide-brand-visual.md) | Medium | Open (implementation PR) | Control IDE brand restyle (navy/gold tokens + logo; not chrome expansion) |

Severity guide: **High** = likely to block real customer load, security, or the install agent-runtime / Ship path; **Medium** = will hurt before GA if ignored. Control IDE commercial delivery is **not** a High driver while chrome is frozen ([ADR-030](../docs/adr/030-install-agent-runtime.md)).

## Frozen Control IDE chrome (retired trackers)

Do **not** add license UX, private update CDN, Operate-as-CRM, BoardHandoff, Query/Monitor/Explorer chrome, Hosting admin UI, four-tile IA work, collection-node remainders, or in-IDE agent hosts unless a later task explicitly unfreezes them.

Historical IDs (files removed; decisions live in [ADR-030](../docs/adr/030-install-agent-runtime.md) and the [freeze table](../docs/architecture/agent-runtime-build-plan.md#freeze-vs-finish)): BP-015, BP-016, BP-018, BP-019, BP-020 (merged into BP-043), BP-021, BP-023, BP-024, BP-027, BP-034, BP-039 (superseded by ADR-021), BP-051, BP-057, BP-059, BP-062. Closed foundation history also dropped: BP-004 (metadata epoch cache), BP-005 (job/outbox SKIP LOCKED), BP-007 (package migrate), BP-012 (Go runtime — [ADR-005](../docs/adr/005-go-runtime.md)).

BP-066 may change existing panels so they call shipped family routes honestly. BP-068 may retoken the dual theme and replace typed brand text with sourced SVG marks. Neither is a license to reopen the frozen list.

## Alignment (reviewed vs product goals)

**Finish — agent runtime:** [BP-065](./BP-065-ide-backend-coupling.md), [BP-048](./BP-048-one-cli.md), [BP-052](./BP-052-customer-inference.md), [BP-040](./BP-040-client-experience-oss-kits.md). [BP-014](./BP-014-agent-outbound-integrations.md) `invoke_skill` is Keep (deferred OAuth/ExecutionRun on BP-047/033).

**Finish — optional IDE demo client:** [BP-066](./BP-066-ide-demo-client-fidelity.md), [BP-068](./BP-068-ide-brand-visual.md) (paint existing chrome; does not unfreeze the list above)

**Finish — identity / install / distro:** BP-017, BP-013, BP-037, BP-063, BP-033, BP-029, BP-030, BP-011, BP-002, BP-025

**Finish — headless Client:** BP-042, BP-045, BP-046, BP-061. [BP-041](./BP-041-record-external-id-upsert-bulk.md) upsert/Bulk is Mitigated (Keep).

**Finish — public docs:** [BP-067](./BP-067-public-docs-site.md) (external CMS aggregator; not this repo’s `make ci`)

**Open product risk (non-IDE):** BP-008, BP-026, BP-028, BP-031, BP-035, BP-038, BP-047, BP-049

**Keep (mitigated — do not reopen for IDE expansion):** BP-001, BP-003, BP-006, BP-009, BP-010, BP-032, BP-036, BP-043, BP-044, BP-050, BP-053, BP-054, BP-055, BP-056, BP-058, BP-060, BP-064

Priority: **BP-065** Phase 2 → **BP-048** → **BP-052** → **BP-040** → identity/claim → **BP-017** → **BP-033** → install/distro → **BP-025** pin gaps → headless Client → OTEL/OSS/automations. BP-066 may run in parallel with BP-065 lockstep; it does not outrank install Finish work. BP-068 is optional IDE paint in parallel with BP-066 (not install Finish). BP-067 is vendor-docs, not product runtime.

### Security & transparency

Open source is **neither automatically more nor less secure**. This `backlog/` tree is the intended **public product risk list** (scrub rules in [BP-026](./BP-026-oss-security-public-backlog.md)); vulnerability reports stay private until fixed. Do not paste advisory detail, PoCs, or customer data into public BP bodies. IDE trust-boundary outcomes live in [control-ide-security-audit.md](../docs/architecture/control-ide-security-audit.md).

**Tree audit (2026-08-25):** Item headers were checked against routes, migrations, and tests. Statuses that still said Open/Pending after the work landed (BP-009 Phase 7, BP-033/065 Phase 1) were aligned to the tree. Frozen chrome files stay omitted from the table; IDs remain in the retired-trackers list above.

Confirmed SI-campaign **defects** are GitHub issues (`[campaign G-…]`), not new BP files. Registry: [customer-rollout-gap-log.md](../docs/customer-rollout-gap-log.md). Fix PRs cite `Fixes #<issue>`.
