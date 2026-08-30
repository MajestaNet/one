# Install as agent runtime — build plan

**Active plan** for [ADR-030](../adr/030-install-agent-runtime.md): the Go install is the governed agent runtime; Control IDE is an optional JWT client (refactor it when that cleans the install — [BP-065](../../backlog/BP-065-ide-backend-coupling.md)); builders use MCP + `one`; end users use Client Experiences.

**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md) · [agent-deploy.md](./agent-deploy.md) · [agent-data-architecture.md](./agent-data-architecture.md) · **loop:** [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) · **coupling:** [ide-backend-coupling-review.md](./ide-backend-coupling-review.md)  
**Domain agents:** `api-families` + `worker-jobs` + `authz-security` + `db-backend-perf` + `deploy-ops`. Spawn `control-ide` in the **same** change set when a backend cleanup needs a client lockstep (BP-065). Do not spawn it to add Electron-only product chrome.  
**Backlog:** [BP-064](../../backlog/BP-064-install-agent-runtime.md) (this plan) · [BP-065](../../backlog/BP-065-ide-backend-coupling.md) · [BP-006](../../backlog/BP-006-agent-guardrails.md) ([loop plan](./hosted-agent-tool-loop-build-plan.md)) · [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) · [BP-048](../../backlog/BP-048-one-cli.md) · [BP-052](../../backlog/BP-052-customer-inference.md) · [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) · compat [BP-053](../../backlog/BP-053-agent-section-harness.md)

---

## Thesis

> Majesta One wins when **every agent is allowed to do only what this org’s AuthZ, harness floor, and skills permit** — not when we ship another IDE. Coding agents and bots are first-class builders. The install owns the loop.

```text
Coding agents, bots, and CI
        │  MCP  ·  family HTTP  ·  one CLI
        ▼
┌──────────────────────────────────────────────────────────┐
│  Majesta One install (product)                               │
│  AuthZ · metadata · Deploy/Ship · AgentSpec              │
│  job-class harness · skills · hosted tool loop · MCP     │
│  inference router (BP-052)                               │
└──────────────────────────────────────────────────────────┘
        │  /auth/v1 + /client/v1
        ▼
Tenant-hosted Client Experience apps (ADR-019)
```

Control IDE, if present, is a thin JWT client of the same APIs. It is **not** where agents live and **not** the Ship GUI of record.

---

## Freeze vs finish

Tracks are binding for agents. **Finish** = implement. **Keep** = already mitigated; no expansion. **Frozen** = no new product work unless a later task explicitly unfreezes.

### Finish (install / builder / end-user)

| Item | Why | Design in this plan |
|---|---|---|
| [BP-006](../../backlog/BP-006-agent-guardrails.md) | Hosted tool loop — **mitigated** | [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) (summary in § Hosted loop) |
| [BP-064](../../backlog/BP-064-install-agent-runtime.md) | Job-class harness, builder MCP, builder skills — **mitigated** | § Harness, § MCP, § builder skills |
| [BP-048](../../backlog/BP-048-one-cli.md) | Ship without Electron (keychain shipped; scratch orgs deferred) | § CLI |
| [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) | Skill-invoke **Keep** (`invoke_skill` + Deploy `allowedSkills` + happy-path tests in tree). Deferred OAuth/ExecutionRun on BP-047/033 | [03-bp-014-skill-invoke.md](./agentic-remainders/03-bp-014-skill-invoke.md) · § Skills |
| [BP-052](../../backlog/BP-052-customer-inference.md) | Install inference SoR; SSE reconnect/cancel remain; Settings UI not required | § Inference |
| [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) | End-user apps, not admin IDE (R1 kit wire landed; R2/R3 remain) | ADR-019 (remainders only) |
| [BP-065](../../backlog/BP-065-ide-backend-coupling.md) | Phase 1 AuthN neutrality landed; coaching / chrome routes / `ide.*` remain | [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) |
| Identity / install | [BP-013](../../backlog/BP-013-jwt-unified-principals.md), [BP-017](../../backlog/BP-017-identity-directory-productionization.md), [BP-037](../../backlog/BP-037-install-claim-customer-sso.md), [BP-063](../../backlog/BP-063-refresh-token-sessions.md) | Unchanged; any client needs them |
| Distribution | [BP-029](../../backlog/BP-029-app-platform-install.md), [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md), [BP-011](../../backlog/BP-011-container-marketplace-fargate.md), [BP-002](../../backlog/BP-002-dedicated-install-fleet-ops.md) | Unchanged |
| Isolation / platform | [BP-033](../../backlog/BP-033-customer-runtime-isolation.md), [BP-008](../../backlog/BP-008-production-packaging.md), [BP-026](../../backlog/BP-026-oss-security-public-backlog.md) | Unchanged |
| Headless Client | [BP-041](../../backlog/BP-041-record-external-id-upsert-bulk.md)–[BP-046](../../backlog/BP-046-record-merge-dedupe.md), [BP-061](../../backlog/BP-061-platform-actions.md) | Unchanged (API, not IDE) |

### Keep (mitigated — do not reopen for IDE expansion)

[BP-053](../../backlog/BP-053-agent-section-harness.md) section floors (compat), [BP-050](../../backlog/BP-050-run-mode-toolspec.md) / [BP-055](../../backlog/BP-055-run-personal-graph.md) / [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) / [BP-060](../../backlog/BP-060-operate-graph-surface.md) shipped IDE surfaces, [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md) API revision pin (still required for **all** clients). Chrome Client routes (run-graphs, conversations, preferences, principal canvases) are **remove-with-IDE-lockstep** — [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) / [BP-065](../../backlog/BP-065-ide-backend-coupling.md).

### Frozen (Control IDE chrome / commercial IDE)

No new work on Control IDE chrome: private update CDN, premium chrome, Operate record/domain/reporting UX, IDE device/mTLS remainders, IDE Object Manager remainders, BoardHandoff, IDE DO Govern UI, Query/Monitor/Explorer tools, CRM Canvas, Hosting chrome, conversation persist expansion, four-tile IA, collection-node remainders, or IDE license onboarding. See this freeze table and [ADR-030](../adr/030-install-agent-runtime.md). Historical tracker IDs (files removed): BP-015, 016, 018, 019, 021, 023, 024, 027, 034, 039, 051, 057, 059, 062.

Refactor existing `tools/control-ide` when that lets the install drop IDE-shaped AuthN, chrome routes, `ide.*` caps, or Electron Apply coaching (BP-065). Do **not** add a Majesta One coding agent inside Electron. Do **not** add a second admin SPA “because we dropped Electron” unless claim/SSO truly cannot be CLI + hosted login pages already on `/auth/v1`.

**Exception — demo honesty, not chrome:** [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md) / [ide-demo-client-uplift-build-plan.md](./ide-demo-client-uplift-build-plan.md) may change existing panels so they call shipped family routes honestly (kill stubs, consume the hosted loop, fill thin Metadata/Govern/Client tools). That is not a license to reopen the Frozen list above.

---

## Locked product decisions

| Decision | Choice |
|---|---|
| Product | Go install (API + worker + MCP + harness + skills + loop) |
| Builder IDE | **Bring your own** (coding agents and bots) |
| Ship GUI of record | `one` + Deploy HTTP; IDE Ship is a twin, not the path |
| End-user UI | Client Experience (ADR-019), not Control IDE Operate |
| Harness SoR | Job class in Go catalog; `primarySection` alias |
| Customer vs harness | Customer may widen tools/skills within AuthZ; may **not** drop below floor |
| MCP | Adapter over existing HTTP; expand catalog only when the family path exists |
| Hosted loop | BP-006; execute **MCP names**; v1 catalog is Client read + gated write + invoke (not Metadata/Deploy) — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) |
| Skills | Named automations + platform actions; invoke via Client and MCP |
| Electron LLM | Forbidden (unchanged) |
| Ops mutate via MCP | Out of v1 |
| Repo layout | One public product monorepo ([monorepo.md](../monorepo.md)); customer Git stays outside |

---

## Job-class harness (BP-064 + BP-053 compat)

### Job classes

| Job class | Harness id (v1) | Job | Floor (illustrative) | Approval default |
|---|---|---|---|---|
| `query` | `harness.query.read` | Ask / query business data | `sobjects.read`, `query`, `search` | writes off |
| `customize` | `harness.customize.metadata` | Shape customer metadata | describe + Metadata reads; writes only if customer widens | `requireApproval=true` on writes |
| `ship` | `harness.ship.release` | Validate / pack / deploy vs org | read-heavy; deploy verbs only when principal has `deploy` | always approve deploy |
| `govern` | `harness.govern.admin` | Identity / PS / install policy | read + careful write | `requireApproval=true` |
| `operate` | `harness.operate.mutate` | Record mutate + platform actions | read + `sobjects.write` / `actions.invoke` if customer opts in | `requireApproval=true` for mutate |
| `skill` | `harness.skill.invoke` | Invoke named automations only | `skills.invoke` ∩ `allowedSkills` | per skill / PS `canRun` |

Exact tool strings live in `internal/agentharness` (same pattern as today). Retune without schema churn.

### Compat map (`primarySection` → job class)

Existing AgentSpecs and YAML keep `primarySection`. Bind/Apply derive `jobClass` if unset:

| `primarySection` | Job class |
|---|---|
| `operate` | `query` |
| `run` | `operate` (ToolSpec/Run coaching → business mutate/tools) |
| `build` | `customize` |
| `ship` | `ship` |
| `govern` | `govern` |
| `settings` | `govern` (install orientation; read-only floor stays `harness.settings.install` until catalog merge) |

Do **not** require a breaking Metadata migration in phase 1. Add optional `jobClass` on AgentSpec; server sets it on create/move from the map. Run-time `Apply` uses job class when present, else the section catalog (today’s `CatalogVersion`).

### Runtime composition (unchanged shape)

```text
tenant instructions
        ▲
        │ prepend
harness.systemPreamble(jobClass, install context)
        │
        ▼
messages → inference (BP-052)

allowedTools_effective = union(harness.toolFloor, customer.allowedTools)
                         ∩ knownAgentTools ∩ AuthZ
```

Worker **and** stream **and** MCP tool-call admission (when the hosted loop or MCP enforces playbook allowlists) call `agentharness.Apply`. Customers cannot PATCH away the floor.

### Persistence

- Keep `primary_section`, `harness_id`, `harness_version`.
- Add nullable `job_class` (`query|customize|ship|govern|operate|skill`) when implementing BP-064 phase 1.
- YAML: `jobClass` optional; pack/apply round-trip; Deploy validate uses Bind.

---

## Builder MCP catalog (BP-064)

Today `internal/mcp` projects Client describe/query/search/CRUD, Metadata `get_object_metadata`, and agent run create/get ([customer-connect.md](../customer-connect.md)). **Deploy/Ops are out of MCP v1** until this plan.

### Add only as projections of existing HTTP

| MCP tool | Maps to | Scope | Notes |
|---|---|---|---|
| `invoke_action` | `POST /client/v1/actions/{apiName}` | `client` | Platform actions (ADR-029) |
| `invoke_skill` | `POST /client/v1/automations/{apiName}/runs` | `client` + PS `automationAccess` | AgentSpec `allowedSkills` ∩ grants |
| `list_objects_metadata` / `upsert_object` / `upsert_field` | Metadata family paths already used by Build | `metadata` + caps | Names match existing routes; do not invent Metadata |
| `org_validate` | `POST /deploy/v1/packages/validate-local` (or current validate route) | `deploy` | Same as `one org validate` |
| `org_deploy` | Deploy apply vs connected install | `deploy` + promote caps | Same SoR as CLI; customer tests gate unchanged |
| `org_retrieve` / `pack` | existing pack/export/retrieve | `deploy` | |
| `install_version` | `GET /version` | authenticated | Read-only; not Ops mutate |

**Rules:** MCP invents no capabilities (ADR-010). If the HTTP path does not exist, do not add the tool. `tools/call` must use the caller JWT and existing `requireCapability` / object AuthZ. High-risk tools (`org_deploy`, writes, `invoke_*`) should honor AgentSpec `requireApproval` when the caller is `principal_type=agent` and a playbook allowlist is in context; user/service principals follow their own Roles.

### Out of MCP v1

- Ops roll / confirm / rollback
- Secret plaintext, `install_secrets` values
- Bootstrap `API_KEYS` minting
- Electron/IDE bridge tools (`graph.*`)

---

## Skills (BP-014 remainder + BP-006)

`allowedSkills` already stores automation `api_name`s. Hosted loop and MCP must **invoke** them:

1. Resolve skill → automation (tenant-owned or managed as allowed today).
2. Enforce AgentSpec allowlist (agents) + PS `automationAccess` / `canRun`.
3. Enqueue the same worker path as `POST /client/v1/automations/{apiName}/runs` (run-as = caller).
4. Outbound HTTPS stays Go host RPC + egress allowlist (existing BP-014). Platform actions stay `invoke_action` / `ctx.invokeAction` (ADR-029), not a second skill type.

Do not put skill execution in Electron or in Deno `fetch`.

---

## Hosted agent tool loop (BP-006)

**Execution spec:** [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md). Do not re-litigate that contract here.

External MCP does **not** close BP-006. Implement in-process / worker execution of tools as the **run actor**:

1. `POST /client/v1/agents/runs` (SSE already streams tokens — BP-052) plans tool calls from the model (native `tools` / `tool_calls`; oneEffects fallback with MCP names).
2. Each call is an **MCP tool name** admitted from expand(harness floor ∪ AgentSpec `allowedTools`) ∩ hosted v1 catalog ∩ AuthZ. Stored allowlists stay harness tokens (`sobjects.read`, …).
3. Reads execute immediately. Writes: if `requireApproval`, park at `awaiting_tool_approval` and wait for `POST .../approve` (JSON → worker job with `resume`; SSE approve stays in-process — do not double-enqueue).
4. Generation is not an approval event (already locked on BP-052).
5. Audit actor = reconstructed run principal (`agent_runs.actor_id`); never `DEFAULT_OWNER_ID`. Executor is `mcp.CallTool`.
6. Keep `FEATURE_FLAGS` including `agents` for MCP; production may keep hosted loop behind the same flag until green.

Do not add `/inference/chat/completions` (BP-052 non-goal). Do not run the loop in Control IDE. Ignore `graphCalls` / `proposal` / `boardHandoff` in the executor. Hosted v1 does **not** execute Metadata upserts or Deploy `org_*` (builders use MCP / family HTTP).

---

## Inference (BP-052)

Install-local `active_source` + providers remain SoR. Control IDE Settings → Inference is an **optional client** of Metadata/Deploy inference routes, not the configuration path of record. Finish remainders that are API/router (model ID retune, reconnect/cancel) without new Settings chrome. Operators may configure via Metadata/Deploy HTTP or CLI.

---

## CLI as Ship (BP-048)

`one` is the builder Ship path:

```text
one auth login → project init → edit (in an editor) → org validate → org deploy
```

Remainders that still matter: OS keychain; document MCP + CLI as equivalent for validate/deploy. **IDE Ship parity** is frozen (do not add panels to match new CLI flags). Publish binaries on `v*` releases stays required.

Tenant Git format unchanged (`one/v1`). Pack/apply SoR remains install DB after org deploy ([ADR-012](../adr/012-customer-repo-and-control-ide.md)).

---

## Builder skills (BP-064)

Ship **product-owned** builder instructions that customers copy or that `one project init` writes into the customer repo (tenant-owned after copy — not vendor `.cursor/` in the product image):

| Artifact | Role |
|---|---|
| `docs/builder-connect.md` (this repo, vendor/docs plane) | How to point an MCP host at `POST /mcp`, pin `One-API-Revision`, use job-class harnesses, validate vs org |
| Customer template `AGENTS.md` + optional `SKILL.md` fragments via `project init` | Connect, query, customize, ship, govern — no Metadata/Deploy from browser Experiences |
| `tools/one-mcp` README | Stdio fallback only; prefer install `/mcp` |

Do not tell customers to open Control IDE. Do not reference vendor paths (`internal/`, `BP-*`, `.cursor/agents`) in **managed starter AgentSpec instructions** (ADR-010). Product docs may.

---

## Client Experience (BP-040)

Unchanged ADR-019: end-user SPAs on customer infra, `client` scope only. Partner certification remainders stay on BP-040. Do not revive Operate as the end-user CRM (frozen BP-018–021).

---

## Phases

| Phase | Owner | Deliverable | Closes |
|---|---|---|---|
| **0** | docs | This plan + ADR-030 + BP-064 + freeze/finish backlog + index links | Docs PR |
| **1** | Go | Optional `jobClass` + Bind map; catalog ids for job classes; Apply uses job class when set; tests; starters keep `primarySection` | BP-064 harness — **Done** |
| **2** | Go | MCP: `invoke_action`, `invoke_skill`, Metadata upsert projections, `org_validate` / `org_deploy` / `pack` / `install_version`; tests | BP-064 MCP — **Done** |
| **3** | Go | Hosted tool loop as run actor per [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) | BP-006 — **Done** (#262) |
| **4** | Go | CLI keychain + `project init` builder templates; `docs/builder-connect.md` | BP-048 remainder / BP-064 skills — **Done** |
| **5** | docs/seed | Starter AgentSpecs document job class; `agents_starter` notes; customer-agents | BP-064 — **Done** |

Phase 0 does not require Go. Phases 1–2 must not wait on Control IDE. Phase 3 must not invent a second tool namespace.

### Acceptance

- [x] Creating an AgentSpec without `primarySection` still fails **or** accepts `jobClass` and fills the alias (Phase 1: accept `jobClass` XOR `primarySection`)
- [x] `Apply` cannot drop the job-class floor via customer PATCH
- [x] An MCP host (or curl MCP) can `org_validate` with a `deploy`-scoped principal and is 403 without it
- [x] `invoke_skill` fails when the skill is not in `allowedSkills` or PS denies `canRun`
- [x] Hosted run executes at least one read MCP tool and one gated write with `awaiting_tool_approval` (BP-006)
- [x] `one org validate|deploy` remains the documented Ship path
- [x] No new Control IDE chrome in the same change set

---

## Risks

| Risk | Mitigation |
|---|---|
| Breaking existing `primarySection` docks | Alias map; no required rewrite of customer YAML in phase 1 |
| MCP deploy as a foot-gun | Same caps as HTTP; AgentSpec approval; customer tests gate |
| Two tool namespaces | One catalog; MCP adapter only |
| Admins with no UI | Claim/SSO HTTP already works without IDE (BP-037); CLI + MCP cover Ship |
| Scope creep into Operate graph | Frozen list above; playbook banner |

---

## Related

- [ADR-030](../adr/030-install-agent-runtime.md) · [ADR-010](../adr/010-customer-agentic-platform.md) · [ADR-012](../adr/012-customer-repo-and-control-ide.md)
- [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) — hosted loop shipped (BP-006 mitigated; do not re-litigate here)
- [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) — remaining IDE-shaped AuthN/coaching/chrome APIs on the install ([BP-065](../../backlog/BP-065-ide-backend-coupling.md))
- [agent-section-harness-build-plan.md](./agent-section-harness-build-plan.md) (shipped section floors; compat)
- [one-cli-build-plan.md](./one-cli-build-plan.md)
- [inference-build-plan.md](./inference-build-plan.md)
- [client-experience-build-plan.md](./client-experience-build-plan.md)
- [outbound-otel-build-plan.md](./outbound-otel-build-plan.md)
- [customer-connect.md](../customer-connect.md) · [customer-agents.md](../customer-agents.md)

---

## Implementation agent prompt (copy into a new agent)

Use the block in [BP-064](../../backlog/BP-064-install-agent-runtime.md#implementation-agent-prompt) so it stays next to the backlog item.
