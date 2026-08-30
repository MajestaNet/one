# ADR-027: Run graph collection nodes (list-view replacement)

## Status

Accepted (Phase 1 shipping: collection kind + list-in-focus)

## Context

[ADR-023](./023-run-personal-graph.md) stores a personal **attention graph** (refs and definitions only, capped at 200 nodes). [ADR-024](./024-run-graph-interactions.md) makes that graph a day-to-day CRM surface via Pin / Wire / Apply. Classic object-tab CRM is supposed to become **lenses and factories over the graph**, not the IA root.

Today the factory is still a **parallel List View rail** (`RunObjectHomePanel`). Users browse objects there, then pin records onto the canvas. That cannot replace list views:

1. The graph has no node that *is* “Accounts” (or “my open Opportunities”).
2. Clicking a `record` pin inspects one row — it cannot stand in for the object.
3. Materializing every row as a graph node blows the 200-node cap and violates hydrate-on-read.
4. Embedding Operate **global search** as a graph node duplicates top-bar chrome ([cross-object-search-build-plan.md](../architecture/cross-object-search-build-plan.md)). Search is ephemeral find (`q` required; empty query is 400). It cannot browse an object.

Two product ideas were on the table:

- A **base / search node per object** that represents the object in its entirety; first click renders the list experience.
- A **search node** that embeds global search on the canvas.

## Decision

### 1. Collection nodes, not a search widget and not exploded records

Add allowlisted node kind `collection`. One node is a **set definition** for an object (the object in its entirety, or a named slice). Live rows hydrate only when the node is focused.

```ts
{
  id: string;
  kind: "collection";
  ref: { objectApiName: string };   // no recordId
  bindingId?: string;               // optional dataBinding (saved query slice)
  searchQ?: string;                 // optional saved find definition (not results)
  label?: string;
  layout?: Layout;
}
```

Caps, sanitizer, and Client AuthZ rules from ADR-023 are unchanged. `searchQ` is a definition string (max 200 bytes). Bindings remain query definitions without `rows`.

### 2. Click → list in focus, not in the viewport

Selecting a collection node opens the **existing List View experience in the graph focus panel** (reuse `RunObjectHomePanel`: query/filter/sort/saved views/bulk/pin). The React Flow viewport stays topology. This does **not** rebuild classic tab IA inside the canvas ([ADR-024](./024-run-graph-interactions.md) non-goal stands).

`record` clicks still open record focus. Collection clicks open the list first.

Pin from that list creates/ensures a `record` node and a `derivedFrom` edge to the collection (provenance). Rows never land in graph JSON.

### 3. Search is a mode of a collection, not a node kind

| Surface | Job | Persistence |
|---|---|---|
| Operate graph command bar | Cross-object type-and-go chrome on the graph tile ([ADR-028](./028-operate-graph-surface.md)) | None (hits land on a collection; pin is explicit) |
| `collection` + `objectApiName` | Browse the object (Client `/query`) | Durable attention |
| `collection` + `bindingId` | Named list / saved query slice | Durable definition |
| `collection` + `searchQ` | Saved find scoped to that object | Durable `q`; results hydrate on focus |
| `signal` | Live KPI tile from a binding | Already shipped |

Do **not** embed the global search combobox as a graph node. A “search node” is just a collection with `searchQ` (or a binding). Global search stays always-on **graph chrome** (command bar on `RunGraphHome`), not app top-bar chrome — [ADR-028](./028-operate-graph-surface.md) relocates the field so Operate’s header matches other modes.

### 4. Scale model

Graph node count stays **O(objects + pins + tools + notes)**, not O(records). Lists paginate through Client query/search under the caller JWT. Agents pin collections with `graph.pinCollection` without dumping rows.

Default empty-graph action: pin collection nodes for enabled objects the principal can describe (Account / Contact / Opportunity when present). List View rail remains a factory until collections are the habitual home.

### 5. Explicit non-goals

- Expanding a collection into persisted child record nodes (optional later: ephemeral preview, still not SoR)
- Team/shared collections (still published ToolSpecs)
- Baking query results, search hits, or field maps into `principal_run_graphs`
- Replacing Client Experiences ([ADR-019](./019-client-experience-oss-kits.md))
- Removing the List View rail in this phase

## Consequences

- Closed `one.runGraph/v1` allowlist gains `collection` (Go sanitizer + IDE validator in lockstep).
- New [BP-059](./027-run-graph-collection-nodes.md) + [ADR-027](./027-run-graph-collection-nodes.md).
- [ADR-023](./023-run-personal-graph.md) storage rules are not reopened; this extends the node allowlist only.
- [ADR-024](./024-run-graph-interactions.md) Pin sources include collections; Object Home becomes the **focus of a collection**, not only a left-rail factory.

## Related

- [ADR-023](./023-run-personal-graph.md) · [ADR-024](./024-run-graph-interactions.md) · [ADR-003](./003-sql-query-engine.md)
- [BP-018](./030-install-agent-runtime.md) · [BP-020](../../backlog/BP-043-cross-object-search-api.md) · [BP-055](../../backlog/BP-055-run-personal-graph.md) · [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) · [BP-059](./027-run-graph-collection-nodes.md) · [BP-060](../../backlog/BP-060-operate-graph-surface.md) · [ADR-028](./028-operate-graph-surface.md)
