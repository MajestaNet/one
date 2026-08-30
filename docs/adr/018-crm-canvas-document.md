# ADR-018: CRM CanvasDocument and agent-constructed canvases

## Status

Accepted (document contract locked; product IDE surface relocated to Run — see [ADR-021](./021-run-mode-toolspec.md))

## Context

Control IDE Operate is chat-first ([customer-ide-ux.md](../customer-ide-ux.md)). BoardHandoff lands query results as inline chat strips ([BP-024](./030-install-agent-runtime.md)). Users and agents still lack a **durable, interactive working set** for CRM artifacts — an interactive canvas model (agent composes first-party UI components into a persistent artifact beside chat) is the right product pattern, but Majesta One must keep the Electron trust boundary ([ADR-012](./012-customer-repo-and-control-ide.md)): no customer React, iframes, or remote renderer code.

An earlier exploration of an OSS browser React SDK + same-origin `/x` Experience host was rejected for this track: it risks a second IDE and does not inherit Control IDE’s hardened shell.

## Decision

### 1. Canvas is a declarative document, not executable UI

Agents (and humans) author a **`CanvasDocument`** with `apiVersion: one.canvas/v1`. The Control IDE validates the document and renders it with a **first-party node library** under `tools/control-ide`. Unknown `kind` values are rejected — never best-effort HTML/JS.

### 2. Allowlisted node kinds only

v1 allowlist (extend only via ADR amendment or explicit plan phase):

`stat` · `recordTable` · `recordCard` · `relatedList` · `queryResult` · `mutationProposal` · `messageThread` · `pipelineLane` · `actionChipGroup` · `markdownNote` · `sectionHeader`

**Forbidden without a new ADR:** `rawHtml`, `iframe`, `remoteReact`, `customScript`, or any node that evaluates agent-supplied code.

### 3. Agent tools construct and revise canvases

Operate agents gain tools (IDE bridge first; Client/MCP when [BP-006](../../backlog/BP-006-agent-guardrails.md) hosted loop is ready):

| Tool | Role |
|---|---|
| `canvas.create` | Validate → persist → open canvas pane → return `canvasId` |
| `canvas.update` | Patch layout/nodes/bindings |
| `canvas.rerun` | Re-execute Client `dataBindings` under current JWT |
| `canvas.get` / `canvas.list` | Source + workspace list for iteration |

Prompt → tool → CanvasDocument is the supported construction path (interactive canvas analogue).

### 4. Configure via CanvasSpec + AgentSpec skills

Reusable layouts live as customer **CanvasSpec** metadata (`metadata/canvases/<apiName>.yaml` in `one/v1`) and optional AgentSpec canvas-skill references. Customers “install” canvases by promoting metadata packages — **not** by shipping UI component packages into the IDE.

### 5. Chat remains primary; canvas is a sibling artifact

1 chat = 1 agent unchanged. Canvas opens beside chat with a reference card (`canvasId` on BoardHandoff / run events). Default Operate IA stays chat-first for customers who never use canvas.

### 6. AuthZ stays on the install

All `dataBindings` and mutations go through Client APIs with the session JWT (or agent principal). The canvas never invents AuthZ; FLS/sharing omit rows/fields honestly.

### 7. Explicit non-goals (this ADR)

- OSS `@one/client` / `@one/react` or Go `/x` Experience host **as the CRM canvas / Operate surface** (customer-hosted Client Experiences are [ADR-019](./019-client-experience-oss-kits.md))
- Customer Electron/React/iframe plugins (ADR-012 stands)
- Replacing Build / Ship / Govern with canvas
- Evaluating agent-emitted JavaScript in the renderer

## Amendment — Client Experience is a separate track (2026-07)

CRM Canvas was framed as an **Operate tool** inside Control IDE. Scalable end-user browser apps are **Client Experiences** under [ADR-019](./019-client-experience-oss-kits.md): OSS kits in `sdk/client/`, Connected Apps with Client-only defaults, customer-hosted SPAs. This ADR's rejection of an OSS Experience host applies to **in-IDE canvas / second IDE**, not to customer Client Experience apps.

## Amendment — Product surface moves to Run mode (2026-08)

The declarative document model (allowlisted nodes, first-party renderers, no customer React) remains. The **product IDE surface** is relocated from Operate to the fifth mode **Run** with **ToolSpec** metadata ([ADR-021](./021-run-mode-toolspec.md), [BP-050](../../backlog/BP-050-run-mode-toolspec.md)). Operate stays chat-first + inspect tools; do not revive an Operate canvas tile. CanvasSpec evolves into ToolSpec (`metadata/tools/`).

## Consequences

- Historical plan: [ADR-018](./018-crm-canvas-document.md) · [BP-039](./018-crm-canvas-document.md) (superseded for IDE UI)
- Active delivery: [ADR-021](./021-run-mode-toolspec.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md)
- ADR-012 stands: declarative configure; no customer renderer plugins
- BoardHandoff may later reference Tool ids under Run ([BP-024](./030-install-agent-runtime.md))
- Deno automations ([ADR-014](./014-customer-code-automations.md)) remain server-side only; they do not import Tool UI

## Related

- [ADR-010](./010-customer-agentic-platform.md) · [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-014](./014-customer-code-automations.md) · [ADR-019](./019-client-experience-oss-kits.md) · [ADR-021](./021-run-mode-toolspec.md)
- [customer-ide-ux.md](../customer-ide-ux.md)
- [BP-018](./030-install-agent-runtime.md) · [BP-019](./030-install-agent-runtime.md) · [BP-024](./030-install-agent-runtime.md) · [BP-006](../../backlog/BP-006-agent-guardrails.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md)
