# BP-014 `invoke_skill` — remainder tech design + agentic build plan

**Work-order slot:** 3 of 12 (recommended Finish order from backlog/README.md)
**Backlog:** [BP-014](../../../backlog/BP-014-agent-outbound-integrations.md)
**Track:** Keep (remainders only — `invoke_skill` itself is Keep; Phases 1–4 of this remainder landed)
**Status of remainder:** Keep — do not re-implement catalog, executor, Deploy `allowedSkills` name-check, or happy-path tests
**Domain agents:** `api-families` then `worker-jobs` then `deploy-ops` (docs/DX last; no `control-ide`)
**Playbooks:** [agent-api-families.md](../agent-api-families.md) · [agent-worker.md](../agent-worker.md) · [agent-deploy.md](../agent-deploy.md) · [agent-authz.md](../agent-authz.md) · [agent-runtime-build-plan.md](../agent-runtime-build-plan.md)
**Existing plans (do not duplicate):** [outbound-otel-build-plan.md](../outbound-otel-build-plan.md) · [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) · [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) § Skills / § MCP · [integrations-build-plan.md](../integrations-build-plan.md) (OAuth / Client invoke — BP-047)

---

## Verdict (shipped vs remainder)

**`invoke_skill` on MCP + hosted loop is already shipped.** The Finish slot listed in [backlog/README.md](../../../backlog/README.md) is **Keep** — do not re-implement the catalog, executor, deny matrix, Deploy `allowedSkills` existence check, or skill-job-class scaffold.

BP-014 as a whole is **not** Mitigated until an owner accepts Deferred items (OAuth/ExecutionRun on [BP-047](../../../backlog/BP-047-integrations-callable-oauth.md) / [BP-033](../../../backlog/BP-033-customer-runtime-isolation.md); sync outbound is never). Outbound HTTPS (secrets, connectors, egress, `ctx.http` / `ctx.connector`) and AgentSpec **grants** (`allowedSkills`) shipped earlier. Do not reopen the hosted multi-tool loop ([BP-006](../../../backlog/BP-006-agent-guardrails.md) mitigated). Control IDE Govern catalog chrome stays **frozen**.

---

## 1. Remainder inventory

| Surface | Shipped (cite packages/tests) | Still open | Evidence |
|---|---|---|---|
| MCP `invoke_skill` catalog + `CallTool` | Yes. `internal/mcp/gateway.go` lists the tool; `CallTool` → `invokeSkill` in `internal/mcp/builder.go` (client scope, agent `playbookApiName` required, `allowedSkills` ∩ PS `canRun`, enqueue `automation.run`) | — | `TestListToolsIncludesDescribeAndCreate`; `TestInvokeSkillDeniedWhenNotInAllowlist` (`internal/mcp/gateway_test.go`); `TestMCPInvokeSkillDeniedWithoutAllowlistOrCanRun` (`internal/httpapi/mcp_builder_test.go`) |
| Hosted loop admission (`skills.invoke` → `invoke_skill`) | Yes. Write class in `HostedLoopV1Catalog`; expand map in `internal/agentharness/hosted.go`; loop injects `playbookApiName` then `mcp.CallTool` (`internal/agentloop/loop.go`) | — | `TestExpandToHostedMCP` (`internal/agentharness/hosted_test.go`); `TestHostedLoopDeniesInvokeSkillNotInAllowlist` (`internal/agentloop/loop_integration_test.go`) |
| Hosted loop write parking for `invoke_skill` | Yes (class=write). Same `requireApproval` park as `create_record` / `invoke_action` | No extra invoke-specific park test | `internal/agentharness/hosted.go`; loop classify-write in `internal/agentloop/loop.go` |
| AgentSpec `allowedSkills` Metadata write | Yes. Create/PATCH call `validatePlaybookSkills` (non-empty names must exist in `metadata_automations`); unknown names 400 | Keep / regression | `internal/httpapi/metadata_routes.go`; `TestMetadataPlaybookUnknownAllowedSkillRejected` |
| AgentSpec `allowedSkills` worker pre-check | Yes. `validateAgentSkills` before `agentloop.Execute` | Reloads live **tools** from the playbook, not **skills** (skills come from the job payload; invoke AuthZ uses **live** playbook via MCP) | `internal/worker/process.go` |
| AgentSpec `allowedSkills` Deploy validate | Yes. `unknownAllowedSkillIssues` fails closed when a playbook names a skill missing from the bundle/install automations | Keep / regression | `internal/deploy/validate.go`; `internal/deploy/allowed_skills_validate_test.go` |
| `invoke_skill` happy path (enqueue) | Yes. Granted skill + `canRun` inserts `automation.run` | Keep / regression | `TestMCPInvokeSkillEnqueuesAutomationRun`; `TestHostedLoopInvokeSkillEnqueuesAutomationRun` |
| Skill job-class builder DX | Job class + harness floor `skills.invoke` shipped; `one project init` writes `skills/skill/SKILL.md` | Keep | `internal/agentharness/jobclass.go`; `internal/customerrepo/scaffold/skills/skill/SKILL.md`; `deploy/customer-repo-template/skills/skill/SKILL.md` |
| Secrets / connectors / egress / `ctx.http` | Yes (outbound-otel Phases 2–6; ADR-014 Phase 7) | Not this remainder | [outbound-otel-build-plan.md](../outbound-otel-build-plan.md); `internal/automation/outbound.go`; `internal/egress` |
| Connector OAuth / Client automation HTTP invoke | Owned by **BP-047** (Phases 0–4 done; ExecutionRun projection stays BP-033) | Do not fold into BP-014 | [integrations-build-plan.md](../integrations-build-plan.md) |
| Control IDE Govern Integrations catalog | Shipped; **frozen** | No new chrome | BP-014 shipped #5; BP-027 frozen |
| Hosted multi-tool loop | BP-006 **mitigated** | Do not reopen | [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) |
| Sync outbound / Deno `fetch` / BYO LLM on AgentSpec | Explicit non-goals | Stay deferred | ADR-014; BP-052 |

BP-014 **Track** is Keep (`invoke_skill` regression). Do not mark the whole item Mitigated until an owner accepts Deferred OAuth/ExecutionRun as BP-047/033 follow-ons.

---

## 2. Detailed design (remainder only)

Cite [ADR-010](../../adr/010-customer-agentic-platform.md) (MCP adapter, AgentSpec), [ADR-014](../../adr/014-customer-code-automations.md) (guest deny-net; async outbound only; PS `canRun`; run-as = caller), [ADR-030](../../adr/030-install-agent-runtime.md) (install is the runtime). Do not invent a parallel skill type or a second tool namespace.

### 2.1 Locked invoke contract (already shipped — do not change)

```text
MCP / hosted loop
  invoke_skill { apiName, input?, playbookApiName? }
        │
        ├─ scope:client
        ├─ principal_type=agent → playbookApiName required
        ├─ if playbookApiName set → apiName ∈ live agent_playbooks.allowed_skills (active playbook)
        ├─ AutomationAz.AssertCanRunAutomation (PS canRun / allAutomations / admin)
        └─ INSERT jobs (job_type=automation.run)  — same payload shape as POST /client/v1/automations/{apiName}/runs
```

Hosted `/agents/runs` additionally:

- Admit `invoke_skill` only when `skills.invoke` expands into the hosted v1 catalog (`ExpandToHostedMCP`).
- Inject `playbookApiName` from the run’s playbook when the model omitted it.
- Classify as **write**: park at `awaiting_tool_approval` when applied `requireApproval` is true; approve resume executes the parked call (BP-006 — do not reopen).

**Direct MCP `invoke_skill` is Client HTTP invoke**, not a hosted-loop write. Do **not** add AgentSpec `requireApproval` parking on `POST /mcp` `tools/call` for `invoke_skill`. Builders already hit family HTTP; the loop’s park is for **model-chosen** writes on `/agents/runs`.

**Live playbook is SoR for skill allowlist** (`playbookAllowsSkill`). `agentloop.Config.AllowedSkills` is audit / worker existence pre-check / SSE `skillsGranted` — not a second AuthZ gate. Do not pin invoke to the run snapshot unless a later ADR says so.

**Inactive automations:** Metadata/worker existence checks do not require `active=true`. Runtime `invokeSkill` already 404s inactive defs. Granting a not-yet-active name is allowed (customer may activate later). Do not add `active=true` to grant validation.

**Empty `allowedSkills` on `jobClass=skill`:** fail-closed at invoke (deny). Do not require a non-empty list at Metadata create — an empty grant is a valid “no skills yet” spec.

### 2.2 Deploy `allowedSkills` existence (real leftover)

BP-014 shipped #3 claimed Metadata/**Deploy**/worker validation. Metadata and worker check names against `metadata_automations`. Deploy `ParseBundleArtifact` only defaults `AllowedSkills` to `[]`. `Validate` loops AgentSpecs for ownership + job class / section bind — **not** skill names.

**Contract (remainder):**

On Deploy validate (same pass that emits `ValidationIssue`s):

- Skip empty entries after trim; empty string → error (`UNKNOWN_SKILL` or reuse a `VALIDATION_ERROR`-style code already used for playbooks).
- Each non-empty `allowedSkills` entry must be one of:
  - an `apiName` in **this bundle’s** `automations` list, or
  - an existing `metadata_automations.api_name` on the **target install** (when validate has a DB — org validate / apply path). Bundle-only validate (no install) accepts names present in the artifact automations; unknown names that are neither in the bundle nor resolvable on the install are errors.
- Do not require the automation to be `active` (same as Metadata).
- Do not invent customer DDL. No new columns.

Prefer a small helper next to existing playbook validation in `internal/deploy/validate.go` rather than duplicating Metadata’s SQL in HTTP handlers. Metadata keep its current `validatePlaybookSkills` (install DB only) — that path already runs on create/PATCH.

Failure mode: promote of an AgentSpec that names a typo skill fails **closed** at validate, matching Metadata 400.

### 2.3 Happy-path tests (regression guard)

Deny tests already lock the Finish slot. Remainder adds **positive** enqueue:

| Test | Assert |
|---|---|
| MCP `tools/call` `invoke_skill` with admin (or PS `canRun`) + playbook whose `allowedSkills` contains a real active automation | HTTP 200 MCP result; `jobs` row `job_type=automation.run` with that `apiName` / `actorId` |
| Hosted `agentloop.Execute` with `skills.invoke` admitted, `requireApproval=false`, playbook grant, actor `canRun` | `StatusCompleted` (or non-failed); same `automation.run` job inserted; `playbookApiName` injected if omitted from the model call |
| Metadata POST/PATCH playbook `allowedSkills: ["NoSuch__c"]` | 400 `VALIDATION_ERROR` |
| Deploy validate artifact playbook `allowedSkills` naming neither bundle nor install automation | issue with error severity |

Reuse `internal/testutil` (`RequireDatabase`, `BootstrapCore`, `NewTestServer`). Hosted loop tests follow `internal/agentloop/loop_integration_test.go` harness (`setupLoopHarness`, `llmScript`). Do not add Deno guest tests here.

### 2.4 Skill job-class builder DX

Product-owned fragments customers copy via `one project init` ([agent-runtime-build-plan.md](../agent-runtime-build-plan.md) § Builder skills):

- Add `skills/skill/SKILL.md` (job class `skill`: invoke only named automations in `allowedSkills`; `invoke_skill` + PS `canRun`; no Metadata/Deploy from this class).
- Extend customer `AGENTS.md` job-class table with `skill` → that file. Optional same-PR row for `operate` is allowed **only** if it is a one-line table entry pointing at existing docs — do not author a new Operate chrome skill.
- Keep `internal/customerrepo/scaffold/` and `deploy/customer-repo-template/` in lockstep (existing pattern).
- Update `docs/builder-connect.md` init sentence that lists `skills/{connect,query,customize,ship,govern}`.
- No Control IDE panels. No Electron. No vendor `.cursor/` in the product image.

### 2.5 AuthZ / API family (no new routes)

No new HTTP paths. Family ownership stays:

| Verb | Family |
|---|---|
| `invoke_skill` | Adapter over **Client** `POST /client/v1/automations/{apiName}/runs` |
| AgentSpec `allowedSkills` CRUD | **Metadata** |
| Promote defs | **Deploy** (refs only for secrets; skill **names** are metadata) |

Capabilities unchanged. MCP invents no verbs (ADR-010).

### 2.6 Docs / BP status (same change set as Go, or docs-only first)

- BP-014: Status notes must record **invoke_skill verified shipped**; drop “hosted multi-tool loop remains” from Status/Track/Deferred (that loop is BP-006 mitigated). Keep Status **Partially mitigated** until Deploy validation + happy-path tests land; then an owner may flip Mitigated / Keep.
- [customer-agents.md](../../customer-agents.md) § Skills: invoke is shipped; outbound HTTP from the **automation** remains this BP’s connector/egress surface (already shipped).
- [agent-runtime-build-plan.md](../agent-runtime-build-plan.md) Finish table row for BP-014: retarget to this remainder (not “implement invoke”).
- [outbound-otel-build-plan.md](../outbound-otel-build-plan.md) thesis still points invoke at BP-006 — amend to “invoke shipped; remainders in this file”.
- Do **not** edit `backlog/README.md` in the remainder implementation unless a later docs owner updates the Finish order (this design’s extra-edit fence).

---

## 3. Concrete agentic build plan

### Phase 1 — Status + docs alignment (no product behavior)

- **Owner:** `api-families` (docs-only)
- **Allowed:** `backlog/BP-014-agent-outbound-integrations.md`; `docs/customer-agents.md`; `docs/architecture/agent-runtime-build-plan.md` (Finish table row only); `docs/architecture/outbound-otel-build-plan.md` (thesis / agent surface sentence); this file already merged
- **Forbidden:** `tools/control-ide/**`; `backlog/README.md`; reopening [hosted-agent-tool-loop-build-plan.md](../hosted-agent-tool-loop-build-plan.md) phases
- **Files:** BP-014; customer-agents; agent-runtime-build-plan Finish table; outbound-otel thesis
- **Tests:** none
- **Exit:** BP-014 no longer claims hosted loop or `invoke_skill` as unshipped; Status remains Partially mitigated until Phase 2–3; Track names this remainder
- **Depends:** none (BP-006 already mitigated)

### Phase 2 — Happy-path + Metadata unknown-skill tests

- **Owner:** `api-families` then `worker-jobs`
- **Allowed:** `internal/httpapi/mcp_builder_test.go` (or sibling); `internal/agentloop/loop_integration_test.go`; Metadata playbook tests under `internal/httpapi/` if a playbook-create test file already exists, else extend `mcp_builder_test.go` / metadata integration tests; `internal/testutil` only as consumer
- **Forbidden:** `tools/control-ide/**`; changing `invokeSkill` AuthZ; widening hosted v1 catalog
- **Files likely:** test files above; fixtures via BootstrapCore + insert automation / playbook (same pattern as deny tests)
- **Tests:** `go test ./internal/httpapi/ ./internal/agentloop/ ./internal/mcp/` (DB-gated tests skip without `DATABASE_URL`)
- **Exit:** one MCP success enqueue; one hosted-loop success enqueue (or park+approve if the harness defaults `requireApproval` — then assert job after approve); Metadata unknown skill 400
- **Depends:** Phase 1 optional (docs can land after)

### Phase 3 — Deploy `allowedSkills` name check

- **Owner:** `deploy-ops` (+ `db-backend-perf` only if the helper needs a pool query already used by org validate)
- **Allowed:** `internal/deploy/validate.go`, `internal/deploy/validate_test.go` (or existing deploy validate tests), `internal/deploy/apply.go` only if validate is the gate (prefer validate-only)
- **Forbidden:** `tools/control-ide/**`; new migrations; promoting secrets/tokens
- **Files likely:** `internal/deploy/validate.go`; a unit/integration test that feeds a `BundleArtifact` with a bogus skill
- **Tests:** `go test ./internal/deploy/`
- **Exit:** unknown skill → error-severity `ValidationIssue`; skill named in the same artifact’s automations passes without DB; org-validate against install accepts names already in `metadata_automations`
- **Depends:** none (can parallel Phase 2)

### Phase 4 — Skill job-class scaffold DX

- **Owner:** `deploy-ops` (customer repo scaffold) — not `control-ide`
- **Allowed:** `internal/customerrepo/scaffold/**`, `deploy/customer-repo-template/**`, `docs/builder-connect.md`, `internal/customerrepo/dx_test.go` (path list)
- **Forbidden:** `tools/control-ide/**`; new AgentSpec starters in `agents_starter` unless already required (no new managed playbook)
- **Files likely:** `skills/skill/SKILL.md` (both trees); `AGENTS.md` job-class table; `dx_test.go` expected paths; `docs/builder-connect.md`
- **Tests:** `go test ./internal/customerrepo/` (scaffold path assertions)
- **Exit:** `project init` materializes `skills/skill/SKILL.md`; customer AGENTS.md lists `skill`
- **Depends:** none; can parallel 2–3

After Phases 2–3 (and 4 if in the same PR), an owner **may** set BP-014 Status to **Mitigated** (Keep / regression guard) **if** they accept OAuth ExecutionRun as BP-047/033 and sync outbound as never. This design does **not** auto-flip Mitigated.

---

## 4. Explicit non-goals

- Reopening hosted multi-tool loop, catalog widening, native `tool_calls` protocol, or `graph.*` Apply ([BP-006](../../../backlog/BP-006-agent-guardrails.md) mitigated)
- Control IDE Govern Integrations / connector wizard chrome (frozen)
- MCP `requireApproval` parking for direct `invoke_skill`
- Snapshot-vs-live dual allowlist; pinning `Config.AllowedSkills` as AuthZ
- Requiring non-empty `allowedSkills` on `jobClass=skill` create
- Requiring `active=true` on granted automations
- Sync automation outbound; Deno `fetch` / npm; guest OAuth
- BYO LLM / AgentSpec provider keys ([BP-052](../../../backlog/BP-052-customer-inference.md))
- Connector OAuth token lifecycle / Client `POST /automations/.../runs` HTTP (already BP-047)
- BP-033 `ExecutionRun` projection
- Editing `backlog/README.md` Finish order in the implementation PR unless a docs owner asks
- New MCP tools, new API families, Ops mutate, product image COPY changes

---

## 5. Agentic implementation prompt(s)

```text
Implement the BP-014 remainder in docs/architecture/agentic-remainders/03-bp-014-skill-invoke.md (Phases 1–4).

Read first:
- that remainder plan (locked invoke contract, Deploy skill check, non-goals)
- backlog/BP-014-agent-outbound-integrations.md
- docs/architecture/outbound-otel-build-plan.md
- docs/architecture/hosted-agent-tool-loop-build-plan.md (do not reopen)
- docs/architecture/agent-runtime-build-plan.md § Skills / MCP
- docs/architecture/agent-api-families.md
- docs/architecture/agent-worker.md
- docs/architecture/agent-deploy.md
- ADR-010, ADR-014, ADR-030
- internal/mcp/builder.go (invokeSkill, playbookAllowsSkill)
- internal/agentloop/loop.go (playbookApiName inject)
- internal/deploy/validate.go (AgentSpec loop)
- existing deny tests: TestHostedLoopDeniesInvokeSkillNotInAllowlist,
  TestInvokeSkillDeniedWhenNotInAllowlist, TestMCPInvokeSkillDeniedWithoutAllowlistOrCanRun

Honest baseline: invoke_skill on MCP + hosted loop is ALREADY SHIPPED. Do not re-implement CallTool, the hosted catalog, or the deny matrix.

Scope:
- Phase 1: retarget BP-014 Status/Track/Deferred (invoke_skill verified shipped; hosted loop is BP-006 mitigated; keep Partially mitigated until 2–3). Touch customer-agents.md, agent-runtime-build-plan Finish row, outbound-otel thesis as listed in the remainder. Do not edit backlog/README.md.
- Phase 2: add MCP + hosted-loop happy-path tests (granted skill + canRun enqueues automation.run). Add Metadata 400 for unknown allowedSkills. Prefer internal/testutil.
- Phase 3: Deploy validate — each allowedSkills name must exist in the bundle automations or install metadata_automations. Fail closed. No new tables.
- Phase 4: add skills/skill/SKILL.md to both scaffold trees; AGENTS.md table row; builder-connect.md init list; dx_test.go paths.

AuthZ: live playbook allowed_skills + PS canRun. Do not add MCP requireApproval parking. Do not use Config.AllowedSkills as a second gate.

Out of scope: tools/control-ide/**; hosted loop protocol; catalog widening; sync outbound; OAuth (BP-047); BYO LLM (BP-052); ExecutionRun (BP-033); marking BP-014 Mitigated unless 2–3 are green and the remainder’s closeout rule is met.

Tests: go test ./internal/httpapi/ ./internal/agentloop/ ./internal/mcp/ ./internal/deploy/ ./internal/customerrepo/
```
