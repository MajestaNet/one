# ADR-028: Operate graph surface (glance, drop, hygiene, graph search)

## Status

Accepted (plan locked; implementation phased)

## Context

Operate’s daily driver is the personal graph ([ADR-023](./023-run-personal-graph.md) · [ADR-024](./024-run-graph-interactions.md) · [ADR-027](./027-run-graph-collection-nodes.md)). Foundation work shipped: refs-only home graph, Pin / Wire / Apply, collection nodes with list-in-focus, ToolSpec mounts, and Client `/search` consumed by an Operate **top-bar** find field.

User feedback after that surface shipped:

1. **Clicking a node feels clunky.** Click toggles multi-select (a second click deselects). The work itself lives in a heavy side inspector (`RunGraphFocusPanel` / embedded Object Home). Node cards are kind + title only, so every glance requires a click. Users asked for either a simple **in-node** experience or a **layout that treats lists as first-class**.
2. **Tools feel leftover.** The left rail still opens ToolSpecs as workspace slices, which **swap the graph away** (`MAX_WORKSPACE_TOOLS = 1`). Mounting a Tool is a header `<select>` + button, not a graph gesture. The graph is the product; Tools are not on it.
3. **The graph accumulates repeats.** Pin / mount / pinCollection are already idempotent on identity, but search hits open a parallel List View tile, rail clicks create a second surface, and compact only removes forbidden/missing records. Users still get duplicate attention.
4. **Global search in the top bar was not designed for chrome stability.** `OperateSearch` sits beside the centered mode title and switches `.top-bar-center` to `justify-content: flex-start`, so Env / theme / session and the launcher title **move** only in Operate.
5. **Search belongs to the graph.** Users expect find at the top of the canvas, with a small focus animation, landing on graph objects — not a second chrome system in the app header.

Storage rules in ADR-023 are not in question. Collection-as-set (ADR-027) is not in question. This ADR locks the **surface contract**: how click, drop, hygiene, and find behave on the graph tile.

## Decision

### 1. Two-speed nodes: glance on the card, work in a sheet

| Speed | Gesture | What the user gets |
|---|---|---|
| **Glance** | Nothing / hover | The node card is enough to recognize the thing (hydrated title, object/tool badge, one subtitle or list affordance). No click required to know *what* it is. |
| **Focus** | Single click | Radio-select that node. Open a **work sheet** docked inside the graph tile (canvas stays visible). Clicking the same node again does **not** deselect. |
| **Clear** | Esc, pane click, sheet × | Selection and sheet close. |
| **Multi** | Shift+click or Cmd/Ctrl+click | Additive selection for Wire / Publish only. |

Collection focus still opens the list (ADR-027). Record focus opens a **compact** record sheet (identity + key fields + related peek + Pin/Wire), not the full Object Home dump. Tool focus opens the Tool renderer **in the sheet**. “Open full record” / “Open Tool as board” remain explicit secondary actions.

The React Flow viewport stays topology. Lists and Tool documents do **not** explode into child nodes.

### 2. Kind-aware layout; collections are the shelf

When nodes have no `layout`, or the user chooses **Tidy layout**, the canvas uses a stable banded layout:

1. **Object shelf** (top) — `collection` nodes, one per object/slice.
2. **Tool shelf** — `tool` mounts.
3. **Working cluster** — records, people, proposals, signals, notes.

User-dragged positions still persist. Tidy never runs implicitly on every load if layout already exists. My day remains a generated queue (ADR-024), not a second canvas.

### 3. Tools are graph objects: drop to mount

Operate’s left-rail ToolSpecs are a **catalog of droppable graph objects**, not a parallel board.

| Gesture | Result |
|---|---|
| Drag a rail Tool onto empty canvas | `graph.mountTool` at drop coordinates (idempotent on Tool identity) |
| Drag a rail Tool onto an existing node | Ensure the tool node + `opens` edge to the target |
| Click a rail Tool while the graph tile is open | Ensure mount at viewport center (or existing node), focus it, open the tool sheet. **Do not swap the graph tile away.** |
| “Open as board” in the tool sheet | Current workspace-slice path (swap), for authoring / full-bleed work |

`graph.mountTool` may accept optional `{ x, y }` layout. Identity uniqueness already shipped; the UX must **use** it (pulse the existing node instead of appearing to create a second one).

### 4. Attention hygiene: ensure, don’t clone

Graph node count stays **O(objects + in-flight pins + tools + notes)**.

| Kind | Identity | Default search / rail gesture |
|---|---|---|
| `collection` | `(objectApiName, bindingId, searchQ)` | Ensure + focus the collection; open the list sheet |
| `record` | `(objectApiName, recordId)` | **Do not auto-pin.** Pin is an explicit hit action |
| `tool` | `toolSpecApiName` **or** `workingToolId` | Ensure + focus |

**Tidy attention** (`graph.compact` strategy `tidy-attention`) may remove:

- Record pins that hydrate as `NOT_FOUND` / `FORBIDDEN` (existing `demote-stale`).
- Record pins with **no** CRM edges (`next`, `watches`, `blocks`, `opens`) that already have `derivedFrom` a collection (the list still holds them).

It must not remove collections, tools, notes, questions, proposals, or My day members. Soft warn in graph chrome at **80** nodes; hard cap **200** unchanged (ADR-023).

### 5. Find is graph chrome, not app chrome

Relocate Operate global search from the **app top bar** into a **graph command bar** at the top of `RunGraphHome`.

- Search is still **not** a node kind (ADR-027 stands).
- Search is still Client `POST /client/v1/search` (BP-043 / BP-020 stay Mitigated; this is chrome placement + landing).
- App top bar Mode title stays **centered in every mode**. Env / theme / session do not move when entering Operate.
- `Ctrl/Cmd+K` focuses the graph command bar. If the graph tile is closed, reopen it first, then focus.
- Focus animation: bar expands slightly, accent ring, results drop from the bar over the canvas; matching collection nodes pulse; landing may pan the viewport. Honor `prefers-reduced-motion`.

Hit landing: ensure a collection for that object, focus it, select the row in the list sheet. Secondary: **Pin to graph**.

### 6. Explicit non-goals

- Baking search hits, list rows, or Tool documents into `principal_run_graphs`
- Embedding the search combobox as a graph node
- Reviving Operate canvas / classic tab IA inside the viewport
- Raising the 200-node cap
- Changing Client `/search` ranking, AuthZ, or bulk composite
- Customer React / iframes (ADR-012)
- Auto-running Tidy layout on every graph load when positions exist

## Consequences

- New [BP-060](../../backlog/BP-060-operate-graph-surface.md) + [ADR-028](./028-operate-graph-surface.md).
- [ADR-027](./027-run-graph-collection-nodes.md) search row: chrome lives on the **graph**, not the app top bar. Collection click still opens the list; the sheet is the focus surface.
- [cross-object-search-build-plan.md](../architecture/cross-object-search-build-plan.md) follow-on: move `OperateSearch` out of `App.tsx` top bar; keep the combobox contract.
- Operate rail click-to-swap for ToolSpecs becomes a secondary “Open as board” path so the graph remains the Operate home tile.

## Related

- [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-021](./021-run-mode-toolspec.md) · [ADR-023](./023-run-personal-graph.md) · [ADR-024](./024-run-graph-interactions.md) · [ADR-027](./027-run-graph-collection-nodes.md)
- [BP-018](./030-install-agent-runtime.md) · [BP-020](../../backlog/BP-043-cross-object-search-api.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md) · [BP-055](../../backlog/BP-055-run-personal-graph.md) · [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) · [BP-059](./027-run-graph-collection-nodes.md) · [BP-060](../../backlog/BP-060-operate-graph-surface.md)
