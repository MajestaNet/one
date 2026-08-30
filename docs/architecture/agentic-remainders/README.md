# Agentic remainder plans — Finish work order

Remainder-only tech designs for the recommended Finish sequence in [backlog/README.md](../../../backlog/README.md). Each file inventories what already shipped, specifies the remaining design, phases an agentic build, and ends with **paste-ready implementation prompts** (§5).

Use these **after this docs PR is merged**. Do not re-plan shipped phases. Do not unfreeze Control IDE chrome unless the slot is [BP-065](../../../backlog/BP-065-ide-backend-coupling.md) lockstep.

Existing original plans stay canonical for locked decisions; these docs are remainder-only. **Index rows retargeted 2026-08-25** after a tree audit (do not paste a first prompt that re-implements a Keep surface).

| Slot | Remainder plan | Backlog | Remainder status | First prompt |
|---|---|---|---|---|
| 1 | [01-bp-065-ide-backend-coupling.md](./01-bp-065-ide-backend-coupling.md) | [BP-065](../../../backlog/BP-065-ide-backend-coupling.md) | Partial — Phase 1 AuthN neutrality landed; Phases 2–4 remain | Phase 2 coaching + IDE Apply lockstep (then chrome routes) |
| 2 | [02-bp-048-one-cli.md](./02-bp-048-one-cli.md) | [BP-048](../../../backlog/BP-048-one-cli.md) | Partial — Phases 1–4 + R1 assets shipped; datapack pin landed | R2 `client_credentials` + env alias hints + org httptest |
| 3 | [03-bp-014-skill-invoke.md](./03-bp-014-skill-invoke.md) | [BP-014](../../../backlog/BP-014-agent-outbound-integrations.md) | Keep — `invoke_skill` + Deploy `allowedSkills` + happy-path tests in tree | Owner closeout only (do not re-implement invoke); OAuth/ExecutionRun are BP-047/033 |
| 4 | [04-bp-052-customer-inference.md](./04-bp-052-customer-inference.md) | [BP-052](../../../backlog/BP-052-customer-inference.md) | Partial — Settings UI frozen; model IDs landed | SSE reconnect/cancel (`Last-Event-ID`, cancel route) |
| 5 | [05-bp-040-client-experience.md](./05-bp-040-client-experience.md) | [BP-040](../../../backlog/BP-040-client-experience-oss-kits.md) | Partial — R1 kit wire + tests landed | R2 `@one/auth` refresh/revoke/exchange |
| 6 | [06-bp-013-037-jwt-claim-sso.md](./06-bp-013-037-jwt-claim-sso.md) | [BP-013](../../../backlog/BP-013-jwt-unified-principals.md) · [BP-037](../../../backlog/BP-037-install-claim-customer-sso.md) | Partial — OIDC encrypt + Slack OpenID Keep | Claim UX (`format=redirect`, `one auth claim`); then exchange hardening |
| 7 | [07-bp-017-identity-directory.md](./07-bp-017-identity-directory.md) | [BP-017](../../../backlog/BP-017-identity-directory-productionization.md) | Open post-GA — R1 Groups-as-tags shipped | Richer SCIM/Client filters (R2); then bulk (R3) |
| 8 | [08-bp-033-runtime-isolation.md](./08-bp-033-runtime-isolation.md) | [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md) | Partial — Phase 1 admission + async Deploy in tree | Phase 2 job classes + quotas (then ExecutionRun) |
| 9 | [09-bp-029-030-011-002-install-distro.md](./09-bp-029-030-011-002-install-distro.md) | [BP-029](../../../backlog/BP-029-app-platform-install.md) · [BP-030](../../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md) · [BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md) | Partial — live DO smoke is operator | Path B Compose/Helm upgrade playbooks; Ops App Platform roller |
| 10 | [10-bp-025-api-revision.md](./10-bp-025-api-revision.md) | [BP-025](../../../backlog/BP-025-ide-api-version-compatibility.md) | Keep — pin gaps closed; Phase 4 deferred | Adapters only on a declared Client wire break; persist pin on hosted runs |
| 11 | [11-bp-041-046-061-headless-client.md](./11-bp-041-046-061-headless-client.md) | [BP-041](../../../backlog/BP-041-record-external-id-upsert-bulk.md) · [BP-042](../../../backlog/BP-042-change-feed-cdc-consumer.md) · [BP-045](../../../backlog/BP-045-files-content-storage.md) · [BP-046](../../../backlog/BP-046-record-merge-dedupe.md) · [BP-061](../../../backlog/BP-061-platform-actions.md) | BP-041 Mitigated; 042/045/046 Open; 061 Partial | CDC (042) → files (045) → `record.merge` |
| 12 | [12-bp-008-026-009-047-ops-automations.md](./12-bp-008-026-009-047-ops-automations.md) | [BP-008](../../../backlog/BP-008-production-packaging.md) · [BP-026](../../../backlog/BP-026-oss-security-public-backlog.md) · [BP-009](../../../backlog/BP-009-no-in-kernel-language.md) · [BP-047](../../../backlog/BP-047-integrations-callable-oauth.md) | Partial — OTEL logs + BP-009 Phase 7 Keep; GHSA/queue-depth remain | GHSA publish-after-fix policy; OTEL queue-depth only after BP-033 P2 |

**How to execute:** open the slot file → read §1 inventory → paste the first §5 prompt into a new implementation agent. Do not skip to a later phase until that slot’s exit criteria pass.

Frozen IDE chrome (BP-015, 016, 018, 019, 021, 024, 027, 034, 051, 057, 062, IDE remainders of 022/023/059) is **not** in this work order.
