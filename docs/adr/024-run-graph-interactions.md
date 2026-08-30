# ADR-024: Run graph interaction contract (Pin / Wire / Apply)

## Status

Accepted (BP-056 Phases 0–6 complete; hosted inbound curator remains deferred to BP-006 + worker jobs)

## Context

[ADR-023](./023-run-personal-graph.md) / [BP-055](../../backlog/BP-055-run-personal-graph.md) shipped the **foundation**: principal-scoped reference-only Run home graph, hydrate-on-read, agent `graph.*` bridge, Tool mount/publish, compaction.

That makes Run a durable attention surface — but not yet a day-to-day CRM replacement. Users still lack:

1. Inspect → act on a node (focus panel with real record work).
2. Human wiring (edges / queue semantics), not only agent rewiring.
3. My day as an actionable work queue (not only a lens filter).
4. Proposal apply on-canvas (mutations staged, never baked into graph JSON).
5. Operate → Run pin handoff so ask/research lands as attention.
6. Selection-scoped coaching that prefers topology writes after every turn.

Without a locked interaction contract, implementers will either rebuild classic CRM tabs inside the graph or violate ADR-023 by storing field payloads / write-through mutations in `principal_run_graphs`.

## Decision

### 1. Three verbs: Pin / Wire / Apply

| Verb | Meaning | Who |
|---|---|---|
| **Pin** | Bring a thing into personal attention (record, person, tool, signal, insight/question) | Human (Object Home, Operate handoff, search) + agent (`graph.pin` / mount / annotate) |
| **Wire** | Give pins CRM meaning via typed edges and clusters | Human (connect / mark next / watch) + agent (`graph.link` / `unlink` / `cluster`) |
| **Apply** | Commit business change via Client APIs | Human approve (or gated agent approve) on a **proposal**; never write field values into the graph document |

Operate remains **ask / research**. Run remains **where work lives**. Classic object tabs stay lenses / factories over the graph — not the IA root.

### 2. Edge semantics (CRM replacement of related lists + tasks + follows)

Allowlisted edge kinds from `one.runGraph/v1` keep these product meanings:

| Kind | Day-to-day meaning |
|---|---|
| `next` | Work queue / follow-up chain |
| `watches` | Monitoring / VIP attention |
| `blocks` | Exceptions / escalations |
| `explains` | Agent or human rationale attached to topology |
| `opens` | This tool/procedure applies to that node |
| `relates` / `owns` / `derivedFrom` | Relationship / provenance |

My day is a **generated queue** over `next` / `watches` / `blocks` (+ optional live signals), not merely `cluster=my-day` filter chrome.

### 3. Proposal persistence (AuthZ-honest)

Durable graph node: `{ kind: "proposal", proposalId }` (+ layout) only.

Proposal **payload** (`ProposedMutation[]`, Client-shaped) lives in:

1. The originating agent run / tool result (evidence), and
2. IDE session staging keyed by `proposalId` for Apply UI.

Apply calls Client mutate under the user’s JWT. Reject / success removes or compacts the proposal node. Graph sanitizer continues to strip `operations`, `data` maps, `rows`, and other baked payloads — proposals must not become a shadow mutation store.

### 4. Operate → Run handoff

[BoardHandoff](../customer-ide-ux.md) gains a first-class **Pin to my graph** path: map `recordIds` (and optional object) → `graph.pin`, open Run home graph, optional `next`/`watches` edges from suggestions. Operate chat remains the ask surface; Run absorbs durable attention.

### 5. Selection-scoped agent coaching

When the user selects one or more graph nodes, Run agents receive those refs via `graph.get` (structure only) plus separate hydrate for display — never full resolve dumps into durable graph state. Good turns **end in visible topology changes** (pin/link/annotate/propose), not transcript-only advice.

### 6. Agent roles

| Role | Job | Graph verbs |
|---|---|---|
| **Curator** | Rebuild My day, raise signals, ask questions, compact stale pins | Pin / Wire / annotate |
| **Doer** | Stage CRM mutations, draft follow-ups | Propose → human Apply |
| **Publisher** | Turn successful personal subgraph into shared ToolSpec | `graph.publishSubgraph` (BP-055) |

Agents may rewire attention freely. Agents may propose record changes. Agents must not silently mutate CRM through the graph store.

### 7. Explicit non-goals

- Rebuilding classic list/report IA inside the graph viewport (list UX lives in **collection focus**, [ADR-027](./027-run-graph-collection-nodes.md))
- Team/shared personal graphs in v1 (shared process = published ToolSpecs)
- GraphQL; durable CRM field snapshots; Operate canvas revival
- Bypassing Client AuthZ / FLS on hydrate or Apply
- Duplicating BP-055 foundation work (CRUD, resolve, publish, compact)

## Consequences

- New [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) + [ADR-024](./024-run-graph-interactions.md).
- Reuse Operate record UX ([BP-018](./030-install-agent-runtime.md)) and BoardHandoff ([BP-024](./030-install-agent-runtime.md)) rather than a second form system.
- Amend customer IDE UX: Run daily driver = graph interactions under this ADR; Object Home remains pin factory.
- Hosted server-side `graph.*` and inbound event→graph curators remain gated on [BP-006](../../backlog/BP-006-agent-guardrails.md) / worker jobs; IDE bridge is sufficient for Phases 1–4.

## Related

- [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-021](./021-run-mode-toolspec.md) · [ADR-022](./022-agent-conversations.md) · [ADR-023](./023-run-personal-graph.md) · [ADR-027](./027-run-graph-collection-nodes.md) · [ADR-028](./028-operate-graph-surface.md)
- [BP-018](./030-install-agent-runtime.md) · [BP-024](./030-install-agent-runtime.md) · [BP-055](../../backlog/BP-055-run-personal-graph.md) · [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) · [BP-060](../../backlog/BP-060-operate-graph-surface.md)
- [ADR-023](./023-run-personal-graph.md) · [customer-ide-ux.md](../customer-ide-ux.md) · [ADR-028](./028-operate-graph-surface.md)
