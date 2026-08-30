# Agent section harness + Build create UX (build plan)

**Shipped (Phases 0–5)** for Control IDE section floors ([BP-053](../../backlog/BP-053-agent-section-harness.md)). **Follow-on:** job-class harnesses (not IDE tiles) are [agent-runtime-build-plan.md](./agent-runtime-build-plan.md) / [ADR-030](../adr/030-install-agent-runtime.md) / [BP-064](../../backlog/BP-064-install-agent-runtime.md). Do not add create-wizard chrome.

**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-authz.md](./agent-authz.md) · [agent-data-architecture.md](./agent-data-architecture.md)  
**Domain agents:** `api-families` + `db-backend-perf` for AgentSpec/harness; **not** `control-ide` for new work  
**Backlog:** [BP-053](../../backlog/BP-053-agent-section-harness.md) (keep) · [BP-064](../../backlog/BP-064-install-agent-runtime.md) (finish) · [BP-006](../../backlog/BP-006-agent-guardrails.md)

---

## Amendment — job class is the SoR (2026-08)

The proprietary wedge is the **managed floor + run-time Apply**, not launcher tiles. Keep `primarySection` as a compatibility alias. New catalog ids and `jobClass` land in BP-064 without breaking existing YAML. Control IDE wizard/docks are frozen.

---

## Thesis

> Customers still **own** AgentSpecs (instructions, goals, vertical flavor). Majesta One owns the **harness**: a product-versioned control plane that binds each agent to a **job class** (compat: exactly one IDE section alias), injects a floor (tools, context pack, approval defaults), and enforces that floor at run time. Creating an agent starts with “which job?”, not a blank form. That is the wedge—BYO agents without BYO chaos. Control IDE section cards were the v1 create UX; builders now use Metadata/MCP/CLI ([ADR-030](../adr/030-install-agent-runtime.md)).

```text
Build → Agents → New
        │
        ▼
  Pick primary section (required)
  operate | run | build | ship | govern | settings
        │
        ▼
  Apply Majesta One section harness (managed floor)
  + customer fields (label, instructions, extras)
        │
        ▼
  AgentSpec persisted (Metadata) ──► dock filters by primarySection
        │
        ▼
  POST /agents/runs ──► Go merges harness overlay + customer instructions
                         + inference (BP-052) + allowlist floor
```

---

## Problem today

| Gap | Reality |
|---|---|
| Mode binding | Hardcoded in IDE by `apiName` (`MetadataBuilder`→build, …); custom agents always `operate+build` |
| No harness entity | Docs describe “adaptive harness”; nothing provisions per-section floors or run-time overlay |
| Create UX | Thin form (label / apiName / goal / instructions / tools CSV) — no section, templates, skills, scopes |
| Differentiation | Customers get a playbook row; platform does not add proprietary lifecycle/context/tool packing |
| Account | Sixth launcher section exists but is agent-free; no settings-home assistants |

---

## Locked product decisions

| Decision | Choice | Rationale |
|---|---|---|
| Primary section | **Required** on create; exactly **one** of `operate` \| `run` \| `build` \| `ship` \| `govern` \| `settings` | Matches the six Account-launcher tiles; forces intentional home |
| Secondary sections | Optional later (`secondarySections[]`); **out of v1** | Keep dock mental model 1:1 with primary |
| Account / settings | First-class harness + dock when `section === "settings"` | User ask for six; install/settings assistants belong here |
| Harness ownership | **Product-managed** catalog in Go (`internal/agentharness` or `internal/packages` harness defs); versioned with product | Proprietary IP; not customer-editable YAML |
| Customer vs harness | Customer may **widen** tools/skills within AuthZ; may **not** drop below harness floor or change `primarySection` without explicit migrate | Security + consistent chrome |
| Persistence | New AgentSpec fields: `primarySection`, `harnessId`, `harnessVersion` (+ optional `secondarySections` later) | First-class Metadata/Deploy/YAML |
| Run-time merge | Go composes system overlay = harness preamble + customer `instructions`; allowlists = union(floor, customer) then AuthZ | IDE stays API-thin (ADR-012) |
| Create UX | Multi-step wizard: Section → Harness preview → Identity → Allowlists → Review | Replaces single blank form |
| Starters | Existing clones keep mapped sections; wizard offers “blank + harness” (default) and “start from starter shape” | No forced clone overwrite |
| Hosted tool loop | Still BP-006; floors keep harness tokens (`sobjects.read`, …); loop expands them to MCP names | Do not block harness on full loop — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md) |
| Electron LLM | Forbidden | Runs stay on install |

### Section → harness intent (v1 floors)

| Section | Harness id | Job | Floor tools (illustrative) | Context pack | Chrome |
|---|---|---|---|---|---|
| `operate` | `harness.operate.query` | Query / ask on business data | `sobjects.read`, `query` (+ write only if customer opts in) | Active env, selection, BoardHandoff | Inline results, approve |
| `run` | `harness.run.tools` | Guide ToolSpec / Run tools | `sobjects.read`, `query` + `allowedToolSpecs` empty→customer fills | Active ToolSpec / session tool | Tool handoff cards |
| `build` | `harness.build.metadata` | Shape customer metadata | `sobjects.read`, `query` (describe via Metadata when BP-006 expands) | Open object / package context | Build dual-write awareness |
| `ship` | `harness.ship.release` | Validate / ship guidance | read-heavy; deploy verbs **not** in agent tools v1 | Change set / env peers | Link to Ship panels |
| `govern` | `harness.govern.admin` | Identity / PS / install policy | read + careful write with **requireApproval=true** default | Principals / caps summary | Strong approve default |
| `settings` | `harness.settings.install` | Account / hosting / inference orientation | read-only floor | `ide.settings*` surfaces, cloudHost | Dock in Account; no mute Hosting secrets |

Exact tool strings stay in one Go constants file (like DO inference model map)—retune without schema churn.

---

## Data model

```sql
-- migration 0051_agent_section_harness.sql (number may shift)
ALTER TABLE agent_playbooks
  ADD COLUMN IF NOT EXISTS primary_section TEXT
    CHECK (primary_section IS NULL OR primary_section IN
      ('operate','run','build','ship','govern','settings')),
  ADD COLUMN IF NOT EXISTS harness_id TEXT,
  ADD COLUMN IF NOT EXISTS harness_version TEXT NOT NULL DEFAULT '';

-- Backfill existing starters + customs (see Phase 1)
```

**API (Metadata playbook JSON):**

```json
{
  "apiName": "RevenueCoach",
  "label": "Revenue coach",
  "primarySection": "operate",
  "harnessId": "harness.operate.query",
  "harnessVersion": "1",
  "instructions": "…customer vertical…",
  "allowedTools": ["sobjects.read", "query", "sobjects.write"],
  "requireApproval": true
}
```

**YAML mirror:** `primarySection`, `harnessId`, `harnessVersion` under `metadata/agents/playbooks/<apiName>.yaml`.

**Create validation:** reject missing/invalid `primarySection`; server sets `harnessId`/`harnessVersion` from catalog (client may hint; server SoR).

---

## Runtime composition

```text
customer instructions
        ▲
        │ prepend (not replace)
harness.systemPreamble(section, install context)
        │
        ▼
messages → inference (BP-052)

allowedTools_effective = union(harness.toolFloor, customer.allowedTools)
                         ∩ knownAgentTools ∩ AuthZ
```

Worker / stream path both call `agentharness.Apply(spec, installCtx)` before LLM/tools. Harness version mismatch → warn in run output; never silent drop of floor.

---

## IDE — Build → Agents create wizard

### Target flow

1. **Section** — six large selectable cards (same labels/icons as launcher). Required.
2. **Harness** — show what Majesta One will apply (tools floor, approval default, context chips). One harness per section in v1 (no picker noise).
3. **Identity** — label, apiName (slug from label), short goal template with `{{focus}}` helper.
4. **Behavior** — instructions (textarea with section starter stub prefilled, editable), requireApproval (pre-checked per harness), optional widen tools / objectScopes / skills.
5. **Review** — summary + Create → POST Metadata → YAML mirror → open detail + toast “Appears in {Section} dock”.

### List / detail uplift

- Badge: section + harness id
- Filter chips by section
- Detail: section read-only (or “Move section…” confirm that re-applies harness floor)
- Remove apiName hardcoding in `App.tsx`; catalog uses `primarySection`

### Account dock

When `section === "settings"`, render `AgentStreamDock` filtered to `primarySection === "settings"` (same hover dock pattern). No agent dock remains “settings-exception.”

---

## Phases

| Phase | Owner | Deliverable |
|---|---|---|
| **0** | docs | This plan + BP-053 + index links |
| **1** | Go | Migration + backfill; Metadata CRUD fields; Deploy snapshot/YAML; harness catalog package; create validation — **done** |
| **2** | Go | Run-time `Apply` on stream + worker agent.run; tests — **done** |
| **3** | IDE | Create wizard + list badges/filters; catalog from `primarySection`; Account dock — **done** |
| **4** | seed/docs | Map AdminSetup→govern, MetadataBuilder→build, RunCoach→run; Ship/Settings starter shapes optional; customer-agents + customer-ide-ux — **done** |
| **5** | polish | Move-section flow; harness version bump playbook; Vitest + go test — **done** |

### Phase 0 exit

Agents can execute Phases 1–4 without re-deriving IA. ✅

### Acceptance (end state)

- [x] Creating an agent without `primarySection` fails (API + IDE)
- [x] Each of the six sections has a documented harness floor in Go
- [x] Custom agents appear only in their primary section dock (incl. Account)
- [x] Run-time messages include harness preamble; tool floor cannot be removed by customer PATCH
- [x] Starters backfilled; IDE no longer hardcodes modes by apiName
- [x] Build → Agents wizard is the default create path (blank form retired)

---

## Harness version bump playbook

When changing product harness floors/preambles in `internal/agentharness`:

1. Bump `CatalogVersion` (and each `Definition.Version`) in `catalog.go`.
2. Existing AgentSpecs keep their pinned `harnessVersion` until create/move/PATCH `primarySection` or a future migrate job rewrites them.
3. Run-time `Apply` **warns** on mismatch (`versionMismatch` + `harness` SSE/output) but still applies the **current catalog floor** — never silently drops tools.
4. Document the bump in release notes; optional SQL backfill of `harness_version` is product-ops, not required for safety.
5. IDE create/move always writes the current catalog version from the server Bind path.

---

## Explicit non-goals

- Multi-section primary / automatic cross-mode routing (v1)
- Customer-authored harness definitions
- Completing BP-006 tool loop (harness floors use current tool names)
- CopilotKit / Electron-side model loop
- Renaming Account launcher tile

---

## Risks

| Risk | Mitigation |
|---|---|
| Existing customs default wrongly | Backfill `operate` + `harness.operate.query`; document in release notes |
| Ship/settings tools too weak until BP-006 | Read-only floors + deep-links to panels; widen when tools exist |
| Harness version drift | Pin `harnessVersion` on spec; Apply warns on mismatch |
| Account dock surprises | Same dock chrome as modes; empty state explains settings assistants |

---

## Related

- [customer-ide-ux.md](../customer-ide-ux.md) — adaptive harness / chat chrome (frozen)
- [ADR-021](../adr/021-run-mode-toolspec.md) — Run ToolSpecs
- [ADR-030](../adr/030-install-agent-runtime.md) — Account section
- [inference-build-plan.md](./inference-build-plan.md) — model routing for composed runs
- [customer-agents.md](../customer-agents.md) · [modules/agents-starter.md](../modules/agents-starter.md)
