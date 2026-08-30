# ADR-021: Run mode and ToolSpec (declarative in-IDE business Tools)

## Status

Accepted (plan locked; implementation phased)

## Context

Control IDE today has four launcher modes — Operate, Build, Ship, Govern ([customer-ide-ux.md](../customer-ide-ux.md)). Operate is chat-first with Query / Monitor / Explorer inspect tools. Build / Ship / Govern author and promote metadata. Customers still lack a **first-class in-IDE surface to run the business**: interactive CRM boards, queues, and action surfaces that agents and builders can customize **without** shipping React into Electron.

[ADR-018](./018-crm-canvas-document.md) locked a declarative CanvasDocument model (allowlisted node kinds, first-party renderers, CanvasSpec metadata). The IDE canvas surface under Operate was deferred ([BP-039](./018-crm-canvas-document.md)). Putting that surface back under Operate conflates **ask/inspect** with **run daily work**.

[ADR-019](./019-client-experience-oss-kits.md) covers customer-hosted browser/mobile Client Experiences (OSS kits + Connected Apps). Those apps must not be embedded as iframes or remote React in Control IDE ([ADR-012](./012-customer-repo-and-control-ide.md)).

Customers already author **server-side** TypeScript automations in Deno ([ADR-014](./014-customer-code-automations.md)). The complementary frontend contract is **configure-only**: declare layout and bindings; Majesta One ships how nodes render.

## Decision

### 1. Fifth mode: Run

Add Control IDE launcher mode **`run`** (“Run” — run your business).

| Mode | Job |
|---|---|
| **Operate** | Ask / query / inspect the active install (chat-primary) |
| **Run** | Open and use declarative **Tools** (interactive business surfaces) |
| **Build** | Author metadata (including ToolSpecs) |
| **Ship** | Validate + deploy |
| **Govern** | Identity, integrations, Experience SPA config, permissions |

`WorkspaceMode` gains `"run"`. Family scope prerequisite: Role **`client`** (same class as Operate). Chrome capability: **`ide.run`**.

### 2. Tools = declarative ToolSpec, not customer UI code

A **Tool** is a customer (or managed-package) metadata artifact that the Run left rail lists and opens.

- **Product name:** Tool / ToolSpec  
- **Document body:** evolves CanvasDocument (`one.canvas/v1` node allowlist and validate path in `internal/canvas`)  
- **Chrome fields:** `label`, `description`, `icon`, `sortOrder` (rail presentation)  
- **Customer path:** `metadata/tools/<apiName>.yaml` in `one/v1`  
- **API (target):** `/metadata/v1/tools` (migrate/alias from `/metadata/v1/canvases`)

Customers promote ToolSpecs via Deploy. They do **not** ship UI component packages, webpack bundles, iframes, or remote renderer code into Control IDE.

### 3. Run information architecture

```text
┌─ Top bar: brand · [centered Mode title = Run] · theme · session ─┐
├─44px─┬─ Workspace (2 vertical slices) ─────────────┬─44px───────┤
│ Tool │  Open Tool = first-party document renderer  │ Agent     │
│ rail │  (allowlisted nodes only; swap on select)   │ dock      │
│dynamic│ Empty Run: prompt to open a Tool or ask    │ hover     │
│meta  │  an agent to compose one                    │ (modes⊇run)│
└──────┴─────────────────────────────────────────────┴───────────┘
```

- **Left rail (Run-only):** metadata-driven list of ToolSpecs (not a static `TileId[]` alone). Filter by package gate + `ide.run` (+ tool-level grants when shipped).  
- **Center:** opening a Tool places a `runTool` workspace tile; shared 2-slice board (max 1 tool + 1 agent; select swaps).  
- **Right dock:** existing `AgentStreamDock`; catalog entries with `modes` including `run`.

### 4. Authoring split

| Where | What |
|---|---|
| **Build** | Durable ToolSpec YAML / Metadata CRUD panel (`ide.build.tools`); pack into customer repo |
| **Run** | Open/use Tools; agents may `tool.create` / `tool.update` / `tool.rerun` then save as ToolSpec |
| **Ship** | Promote ToolSpec changes like any other customer metadata |

### 5. First-party renderers + OSS base kit

Control IDE renders Tools with vendor React under `tools/control-ide/src/renderer/run/`. Unknown node `kind` values are rejected.

**Allowlisted node kinds** (carried from ADR-018; extend only via ADR amendment or build-plan phase):

`stat` · `recordTable` · `recordCard` · `relatedList` · `queryResult` · `mutationProposal` · `messageThread` · `pipelineLane` · `actionChipGroup` · `markdownNote` · `sectionHeader`

**Forbidden without a new ADR:** `rawHtml`, `iframe`, `remoteReact`, `customScript`, or any node that evaluates agent- or customer-supplied code.

**Approved base kit** (headless / lightly styled engines only; paint with Majesta One CSS tokens — [control-ide-design.md](../control-ide-design.md)):

| Concern | Library |
|---|---|
| A11y primitives | Base UI (`@base-ui/react`) |
| Tables | TanStack Table + TanStack Virtual |
| Pipeline drag | `@dnd-kit/core` (+ sortable) |
| Spatial shell | React Flow (`@xyflow/react`) |
| Forms | React Hook Form + Zod |
| Stats | Recharts |
| Markdown notes | `react-markdown` (no raw HTML) |

Do **not** adopt a third-party component library as the Run design system.

### 6. AuthZ stays on the install

All `dataBindings`, mutations, and automation invokes go through Client APIs under the session JWT (or agent principal). Tools never invent AuthZ; FLS/sharing omit rows/fields honestly. `actionChipGroup` may invoke `POST /client/v1/automations/{apiName}/runs` when the caller is allowed — secrets stay in connectors ([ADR-014](./014-customer-code-automations.md), [BP-047](../../backlog/BP-047-integrations-callable-oauth.md)).

### 7. Agent tools (Run)

| Tool | Role |
|---|---|
| `tool.create` | Validate → persist working doc → open Run tile → return id |
| `tool.update` | Patch layout/nodes/bindings/chrome |
| `tool.rerun` | Re-execute Client `dataBindings` under current JWT |
| `tool.get` / `tool.list` | Source + list for iteration |
| `tool.saveAsSpec` | Persist working doc as ToolSpec metadata (requires `metadata.build` / Build path as designed) |

AgentSpec playbooks may declare `allowedToolSpecs` (evolves `allowedCanvasSpecs`).

### 8. Relationship to CanvasSpec and Experience

| Artifact | Role after this ADR |
|---|---|
| **ToolSpec** | Product configure type for Run Tools (evolves CanvasSpec) |
| **CanvasSpec / Operate canvas** | Historical ADR-018 contract; IDE product surface **relocated to Run** ([BP-050](../../backlog/BP-050-run-mode-toolspec.md) supersedes BP-039 for IDE UI) |
| **Experience** (ADR-019) | Customer-hosted SPA config only (`homeUrl`, Connected App, origins) — Govern list; **not** embedded in Run |

### 9. Explicit non-goals

- Customer Electron / React / iframe / remote code plugins (ADR-012 stands)  
- Product-hosted `/x` SPA or webpack customer UI into the Go image  
- Replacing Client Experience for portals, public sites, mobile shells, telephony softphones  
- Reviving CRM Canvas as an Operate left-rail tool  
- Full third-party design-system adoption  
- Evaluating agent-emitted JavaScript in the renderer

## Consequences

- Implementation: this ADR 
- Risk tracking: [BP-050](../../backlog/BP-050-run-mode-toolspec.md)  
- [ADR-018](./018-crm-canvas-document.md) amended: product Tool/canvas surface lives in **Run**  
- [BP-039](./018-crm-canvas-document.md) marked superseded for IDE surface (design history preserved)  
- Capabilities: `ide.run`, `ide.run.tools`, `ide.build.tools`; redirect `ide.operate.canvases` / `ide.build.canvasSpecs`  
- [tech-stack.md](../tech-stack.md) documents the Run base kit (deps land in implementation Phase 3)  
- Deno automations remain server-side only; Tools invoke them via Client — they do not import UI  
- **Run home evolution:** personal reference-only work graph ([ADR-023](./023-run-personal-graph.md) / [BP-055](../../backlog/BP-055-run-personal-graph.md)); CRM-replacement interactions Pin / Wire / Apply ([ADR-024](./024-run-graph-interactions.md) / [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md)) — ToolSpecs remain mountable templates; graph stores refs/definitions only and hydrates via Client AuthZ

## Amendment — Run chrome frozen (2026-08)

ToolSpec **metadata** remains valid. **New Control IDE Run/Operate graph chrome is frozen** ([ADR-030](./030-install-agent-runtime.md)). End-user Tools belong on Client Experience; builder invoke is MCP / hosted loop / platform actions. Do not expand Electron Run mode unless a BP is explicitly unfrozen.

## Related

- [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-014](./014-customer-code-automations.md) · [ADR-018](./018-crm-canvas-document.md) · [ADR-019](./019-client-experience-oss-kits.md) · [ADR-023](./023-run-personal-graph.md) · [ADR-024](./024-run-graph-interactions.md)  
- [customer-ide-ux.md](../customer-ide-ux.md) · [control-ide-design.md](../control-ide-design.md) · [agent-control-ide.md](../architecture/agent-control-ide.md)  
- [BP-039](./018-crm-canvas-document.md) · [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md) · [BP-055](../../backlog/BP-055-run-personal-graph.md) · [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md)
