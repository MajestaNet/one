# ADR-023: Run personal graph (reference-only, principal-scoped)

## Status

Accepted (Phases 0–6 complete)

## Context

Control IDE **Run** ([ADR-021](./021-run-mode-toolspec.md) / [BP-050](../../backlog/BP-050-run-mode-toolspec.md)) opens declarative ToolSpecs as workspace documents. Spatial layout exists (`layout.mode=spatial`, React Flow) but Tools remain **catalog documents**, not a personal work surface. Business users still mentally navigate classic CRM (object tabs / list → detail).

We want Run home to become a **personal graph**: tools, record pins, insights, and work edges live as topology; agents interpret and rewire that topology. Traditional CRM becomes optional projections of the same graph.

Hard constraints:

1. Graphs are **personal** → stored against the principal on the install.
2. Graphs store **references/definitions only** — never AuthZ-controlled record payloads as SoR.
3. Efficiency via **batch hydrate + IDE cache**, not GraphQL (explicit non-goal in [tech-stack.md](../tech-stack.md) / [api-families.md](../api-families.md)).
4. Server must **strip/reject baked payloads** on write (extend `internal/canvas.SanitizeNodesJSON` pattern).
5. Mutations never write-through the graph store — only Client APIs (+ `mutationProposal`).
6. No customer React / plugins ([ADR-012](./012-customer-repo-and-control-ide.md)). No parallel agent-only AuthZ ([BP-006](../../backlog/BP-006-agent-guardrails.md)).

## Decision

### 1. New Client artifact: RunGraph

Principal-scoped document on Client (`scope: client`), one **home** graph per principal per install (optional named lenses later).

- Not Metadata; not Deploy-promoted.
- Publish path: explicit subgraph → ToolSpec (`metadata.build` + tool AuthZ), separate from personal graph.

### 2. API version `one.runGraph/v1`

Closed schema. Unknown top-level and node keys rejected or stripped (fail closed). Node kinds and edge kinds are product allowlists.

### 3. Reference-only persistence

Durable fields: topology, refs (`objectApiName` + `recordId`), ToolSpec/working-tool mounts, query **definitions** (bindings), annotations (markdown text), layout, lens filters.

Never durable as truth: `rows`, field maps, query results, hydrated cards, message bodies, mutation operation payloads.

### 4. Hydrate-on-read

Display of records/signals uses Client GET/query/composite (or dedicated `POST .../resolve`) under the caller JWT. UI ignores baked props even if present.

### 5. AuthZ

- Graph row: owner = JWT principal only.
- Mounted Tools: `tool_permissions` (`can_open`, `can_interact`, `can_modify`, `can_publish`).
- Record visibility: unchanged Client object/FLS/sharing on hydrate.
- Agent graph tools use the same sanitize + owner path (run-as user).

### 6. Explicit non-goals

GraphQL; storing record snapshots as SoR; team shared graphs in v1; Operate canvas revival; replacing [ADR-019](./019-client-experience-oss-kits.md) Experiences; Electron-side LLM.

## Consequences

- New [BP-055](../../backlog/BP-055-run-personal-graph.md) + [ADR-023](./023-run-personal-graph.md); amend [ADR-021](./021-run-mode-toolspec.md) (Run home = graph); keep ToolSpec as reusable subgraph templates.
- Extend sanitizer for graph docs; `principal_canvas_documents` may evolve or be replaced by `principal_run_graphs`.
- Hosted agent loop ([BP-006](../../backlog/BP-006-agent-guardrails.md)) remains required for server-side agent graph ops; IDE bridge unblocks earlier.
- Graph GET/PUT/PATCH carry revision ETags; IDE mutations use `If-Match` so concurrent viewport, human, and agent writes conflict instead of overwriting.
- Day-to-day CRM interaction layer (focus panel, human wire, My day queue, proposals, Operate→Run handoff) is [ADR-024](./024-run-graph-interactions.md) / [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) — does not reopen this ADR’s storage rules.
- Object/list-view replacement via `collection` nodes (set definitions; list in focus) is [ADR-027](./027-run-graph-collection-nodes.md) / [BP-059](./027-run-graph-collection-nodes.md) — extends the node allowlist only.
- Graph **surface** (glance cards, work sheets, drop-to-mount Tools, hygiene, command bar) is [ADR-028](./028-operate-graph-surface.md) / [BP-060](../../backlog/BP-060-operate-graph-surface.md) — does not reopen storage rules.

## Related

- [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-016](./016-record-sharing.md) · [ADR-018](./018-crm-canvas-document.md) · [ADR-021](./021-run-mode-toolspec.md) · [ADR-022](./022-agent-conversations.md) · [ADR-024](./024-run-graph-interactions.md)
- [BP-006](../../backlog/BP-006-agent-guardrails.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md) · [BP-053](../../backlog/BP-053-agent-section-harness.md) · [BP-055](../../backlog/BP-055-run-personal-graph.md) · [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md)
- `internal/canvas` sanitize
