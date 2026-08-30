# Hosted agent tool loop — build plan

**Shipped.** Worker/SSE call `internal/agentloop` (native `tool_calls` + fence fallback, MCP names, write parking). BP-006 is mitigated.

**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-worker.md](./agent-worker.md) · [agent-authz.md](./agent-authz.md) · [agent-runtime-build-plan.md](./agent-runtime-build-plan.md)  
**Domain agents:** `api-families` then `worker-jobs` (then `authz-security` if actor reconstruction changes). **Do not** spawn `control-ide`.  
**Depends on:** [BP-064](../../backlog/BP-064-install-agent-runtime.md) MCP catalog + job-class harness (landed) · [BP-052](../../backlog/BP-052-customer-inference.md) generation + SSE (text-only today)  
**Related:** [ADR-010](../adr/010-customer-agentic-platform.md) · [ADR-030](../adr/030-install-agent-runtime.md) · [customization-authz.md](./customization-authz.md) · [inference-build-plan.md](./inference-build-plan.md)

This document **locks the loop contract**. AuthZ principal parity and the Go executor already shipped; do not reopen [customization-authz.md](./customization-authz.md).

---

## Goal (what a customer can execute)

The hosted loop is the install **doing the work** an AgentSpec described, not only writing a chat reply.

Today `POST /client/v1/agents/runs` streams an LLM answer and records `toolsPlanned`. It does not call Client/Metadata/Deploy. After this plan, the same run executes allowed tools **as that agent principal** — query and read records, then (when `requireApproval`) create/update a record or invoke a named skill / platform action — still limited by Roles, permission sets, harness floor, and the playbook allowlist.

A customer can then run an in-product agent that looks up Accounts and, after a human approve, updates a field or starts an automation, without an external MCP host or Control IDE in the middle. External MCP (`POST /mcp`) is a **different path**: builder hosts already hit HTTP. This loop is for **hosted** runs on the install.

```text
POST /client/v1/agents/runs  (+ SSE or worker job)
        │
        ▼
  inference (BP-052)  ── native tool_calls (MCP names)
        │
        ▼
  internal/agentloop  ── mcp.CallTool as reconstructed run Actor
        │
        ├─ read tools → execute, append tool result, continue
        └─ write tools → park awaiting_tool_approval → POST .../approve → execute → continue
```

---

## Locked product decisions

| Decision | Choice |
|---|---|
| Where the loop runs | Go install only (`internal/agentloop`). Worker JSON path **and** in-process SSE share that package. Not Electron, not Deno, not MCP-host-side. |
| Tool namespace | **MCP tool names** at execution time. AgentSpec `allowedTools` stays harness tokens (`sobjects.read`, …) and is **expanded** at admission. No customer YAML rewrite. |
| How the model calls tools | Native OpenAI-compatible `tools` / `tool_calls` on `ChatRequest`. Fallback: parse ```oneEffects``` `toolCalls` **only if** each entry uses an MCP name. |
| IDE fence | `graphCalls` / `proposal` / `boardHandoff` stay Control IDE Apply. The hosted executor **ignores** them (persist on run output for optional clients; do not Apply). |
| Executor | `mcp.CallTool(ctx, deps, runActor, name, args)` — same AuthZ as HTTP. Invent no MCP-only verbs. |
| Run actor | Reconstruct `authz.Actor` from `agent_runs.actor_id` (scopes, PSs, admin). If missing or unresolvable, **fail the run**. Never fall back to `DEFAULT_OWNER_ID`. |
| Reads vs writes | Table below. Reads execute immediately. Writes park when the applied harness/playbook `requireApproval` is true. |
| Approve | Reuse `POST /client/v1/agents/runs/{id}/approve`. JSON → worker job with resume payload (do not start a second generation). SSE approve continues in-process (do not also enqueue). Same split as BP-052 generation. |
| Parked status | New `awaiting_tool_approval` (distinct from pre-generation `awaiting_approval`). |
| Flag | Same `FEATURE_FLAGS` including `agents`. No second flag in v1. Production/Marketplace may omit `agents` until the loop is green. |
| Dry-run | Advertise tools to the model; **do not execute** any tool (plan only). Matches today’s “planned Platform API tool calls only.” |
| Inference surface | Do **not** add `/inference/chat/completions`. |

---

## Harness token → MCP name map

Stored `allowedTools` / harness `ToolFloor` keep today’s tokens. Expand at admission (`agentharness` + loop). Unknown MCP names are dropped.

| Harness token | MCP tools (v1 loop) | Class |
|---|---|---|
| `sobjects.read` | `describe_global`, `describe_object`, `get_record` | read |
| `query` | `query` | read |
| `search` | `search` | read |
| `sobjects.write` | `create_record`, `update_record` | write |
| `skills.invoke` | `invoke_skill` | write |
| `actions.invoke` | `invoke_action` | write |

Admission for a call:

```text
name ∈ hostedLoopV1Catalog
  ∩ expand(harness.toolFloor ∪ spec.allowedTools)
  ∩ KnownAgentTools-expansion
  ∩ mcp.CallTool AuthZ (scope + object/FLS + caps + skill allowlist/canRun)
```

`invoke_skill` still requires AgentSpec `allowedSkills` (when a playbook is on the run) **and** PS `canRun`, as MCP already enforces.

---

## Hosted loop v1 catalog

**In (must implement):**

| MCP tool | Class | Notes |
|---|---|---|
| `describe_global` | read | |
| `describe_object` | read | |
| `get_record` | read | |
| `query` | read | |
| `search` | read | |
| `create_record` | write | |
| `update_record` | write | |
| `invoke_action` | write | Only if `actions.invoke` expanded into the allowlist |
| `invoke_skill` | write | Only if `skills.invoke` expanded; enqueue same as Client automation runs |

**Out of hosted loop v1** (builders use MCP / family HTTP; do not execute from `/agents/runs`):

- Metadata: `get_object_metadata`, `list_objects_metadata`, `upsert_object`, `upsert_field`, `list_agent_specs`
- Deploy: `org_validate`, `org_deploy`, `pack`, `org_retrieve`
- `install_version`
- Recursion: `create_agent_run`, `get_agent_run`
- IDE bridge: `graph.*`

Widening the hosted catalog later is adding a row to the “In” table **and** a harness token (or an explicit exception). Do not silently execute Deploy from a query agent.

---

## Model protocol

Extend `internal/inference` (same OpenAI-compatible client):

- `ChatRequest.Tools` — JSON Schema from `mcp.ListTools()` filtered to the admitted set.
- `Message` supports assistant `tool_calls` and `role=tool` results (`tool_call_id`, `content`).
- Stream path: if the provider emits tool-call deltas, assemble them; do not treat them as token text.
- If the model returns **only** prose, complete the run (today’s behavior). Do not invent tool calls.
- If the model returns ```oneEffects``` with `toolCalls: [{ "tool": "<mcpName>", "input": {…} }]`, treat as one round of calls (fallback). Ignore `graphCalls` / `proposal` / `boardHandoff` in the executor.

Native tool calling is the primary path. Fence fallback exists because some DO OSS models may ignore `tools`.

---

## Shared executor (`internal/agentloop`)

One package, two callers: `internal/worker` `agent.run` and `internal/httpapi` SSE (`streamAgentRunLLM`).

Responsibilities:

1. Load playbook + `agentharness.Apply` (already done by callers; loop takes `Applied` + admitted MCP names).
2. Reconstruct run `Actor` from `actor_id`.
3. Round: `Complete`/`Stream` with tools → zero or more `tool_calls` → for each, classify read/write → execute or park → append `role=tool` → next round.
4. Stop when: no tool calls, `MaxToolRounds` (8), context cancel, or unrecoverable error.
5. Persist SSE-adjacent events via `inference.AppendRunEvent` so `GET .../runs/{id}/stream` can replay.

Do not duplicate `mcp.CallTool`. Wire `mcp.Deps` the same way `mcp_routes.go` does.

### Actor reconstruction

Use the existing principal resolver (user row + Roles + permission sets). The run actor **is** the AgentSpec’s caller (`agent_runs.actor_id`), not a synthetic “loop user.” Audit `actor_id` on tool outcomes stays that principal.

### Parking writes

When the next call is **write** and applied `RequireApproval` is true:

1. Set run `status=awaiting_tool_approval`.
2. Persist `output.pendingToolCall`: `{ "id", "name", "arguments", "round" }` (JSON on existing `agent_runs.output`; no new column in v1).
3. Emit SSE `approval_required`.
4. Return. Do not execute.

`POST .../approve`:

- Requires `govern.agents` / `agents.approve` as today for human approvers (unchanged capability).
- Executes **only** the parked call (re-check admission + AuthZ), then continues the loop.
- JSON: enqueue `agent.run` with `resume: true` + parked call (worker must not start a blank generation).
- SSE: continue in-process; **do not** insert a job.

Generation-not-an-approval-event remains: streaming create still starts the LLM with `approved: false`. Approval is for **writes**, not tokens.

### SSE events (additive)

Existing: `run` · `harness` · `token` · `done` · `error`.

Add:

| Event | When |
|---|---|
| `tool_call` | Model requested a tool (name + args; redact secrets) |
| `tool_result` | Executor finished (name + truncated result / error) |
| `approval_required` | Parked write; includes `pendingToolCall` |

Replay uses `agent_run_events` as today.

---

## Budgets (v1 constants, retune without schema)

| Cap | Value |
|---|---|
| `MaxToolRounds` | 8 model→tool rounds per run |
| SSE wall clock | Existing stream timeout (5 minutes) covers generation + tools |
| Worker | Existing job lease; do not hold a lease across `awaiting_tool_approval` (complete/release the job when parking; resume is a new job) |
| Tool result size | Truncate to 32 KiB in the model transcript; full result may live on the event payload |

BP-033 job-class quotas stay a follow-on: do not block this loop on ExecutionRun budgets. Document the hook: loop rounds count as agent-class work when BP-033 lands.

---

## What stays stub until this ships

Shipped: Worker/SSE call `internal/agentloop` (native `tool_calls` + fence fallback, MCP names, write parking).

---

## Phases (Go, after this docs PR)

| Phase | Owner | Deliverable |
|---|---|---|
| **0** | docs | This plan + index/playbook/BP-006 pointers (this change set) |
| **1** | Go | Expand harness tokens → MCP names; hosted v1 allowlist helper; unit tests |
| **2** | Go | Inference `Tools` / `tool_calls` / `role=tool`; stream assembly |
| **3** | Go | `internal/agentloop` + actor reconstruction; `mcp.CallTool`; read-tool path on worker **and** SSE |
| **4** | Go | Write parking `awaiting_tool_approval`; approve resume (JSON job / SSE in-process); SSE events |
| **5** | Go | `invoke_skill` / `invoke_action` when admitted; tests below |

Stacked PRs are fine. Do not invent a second tool namespace. Do not edit `tools/control-ide/**`.

### Acceptance

- [x] Hosted run executes at least one **read** MCP tool (`query` or `get_record` / `describe_object`) as the run actor
- [x] Hosted run parks a **write** (`create_record` or `update_record`) at `awaiting_tool_approval` when `requireApproval` is true; `POST .../approve` executes it and completes
- [x] SSE approve does not insert an `agent.run` job; JSON approve does (resume, not a blank run)
- [x] Tool not in expanded allowlist is not executed (403 / loop error; run fails or skips per executor — **fail the call, continue or stop documented**: **stop the run** on AuthZ deny)
- [x] `invoke_skill` denied when not in `allowedSkills` or PS `canRun` (same as MCP)
- [x] `graphCalls` in model output are not applied by the loop
- [x] Dry-run executes zero tools
- [x] Missing `actor_id` fails the run
- [x] No new Control IDE chrome; no `/inference/chat/completions`; no Ops mutate

---

## Explicit non-goals (not required to close BP-006)

- Richer high-risk approval matrices beyond playbook `requireApproval` + `govern.agents`
- Hosted inbound event→graph curator (Operate-adjacent; frozen chrome)
- Hosted Metadata upsert / Deploy `org_*` from `/agents/runs`
- Native tool calling on every DO OSS model (fence fallback is enough)
- Changing product image COPY allowlist
- Electron LLM or IDE-side tool loop
- A fourth API family or MCP-only verbs

---

## Risks

| Risk | Mitigation |
|---|---|
| Two tool namespaces | Expand tokens at admission; execute MCP names only |
| Dual worker/SSE logic | One `agentloop` package |
| Model ignores `tools` | oneEffects `toolCalls` fallback with MCP names |
| Approve double-enqueue | Same BP-052 rule: SSE in-process, JSON job with `resume` |
| Loop as bootstrap owner | Fail if actor cannot be reconstructed |
| Deploy from a query agent | Hosted v1 catalog excludes Deploy |

---

## Implementation agent prompt

Paste into a new agent after this docs PR is merged. Implement **Phases 1–4 first** (read + gated write). Add `invoke_skill` / `invoke_action` in the same PR only if 1–4 tests are green. Do **not** edit `tools/control-ide/**`.

```text
Implement Majesta One hosted agent tool loop (BP-006) per docs/architecture/hosted-agent-tool-loop-build-plan.md.

Read first:
- that plan (locked decisions, name map, v1 catalog, parking, SSE events)
- docs/architecture/agent-runtime-build-plan.md
- backlog/BP-006-agent-guardrails.md
- docs/architecture/agent-api-families.md
- docs/architecture/agent-worker.md
- docs/architecture/inference-build-plan.md (stream + approve enqueue rules)
- docs/architecture/module-map.md

Scope: internal/agentharness (expand map), internal/inference (tools/tool_calls),
internal/agentloop (new), internal/worker agent.run, internal/httpapi agent run SSE + approve.
Reuse mcp.CallTool. AuthZ = reconstructed run Actor. No new capabilities. No IDE.

Acceptance: one hosted read tool; one gated write with awaiting_tool_approval + approve;
SSE approve does not double-enqueue; dry-run executes nothing; graph.* not applied.
```
