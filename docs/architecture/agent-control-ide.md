# Agent playbook: Control IDE

**Optional client ([ADR-030](../adr/030-install-agent-runtime.md)).** New Electron-only product chrome (license, update CDN, in-IDE agent host, Operate as end-user CRM) stays out. **Refactor this tree when that cleans the Go install** — [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) / [BP-065](../../backlog/BP-065-ide-backend-coupling.md) is explicit cross-plane work. New agent/Ship/Build capability belongs on the Go install (MCP, harness, CLI) — [agent-runtime-build-plan.md](./agent-runtime-build-plan.md).

For agents changing the Majesta One Control IDE (`tools/control-ide`) — Electron shell, React panels, Vite build, Vitest. Follow this before writing code.

## Plane fence (mandatory)

| May edit | Must not edit |
|---|---|
| `tools/control-ide/**` | `cmd/`, `internal/`, `migrations/`, `deploy/` (product plane) |
| This playbook, [`control-ide-build.md`](../control-ide-build.md), [`control-ide-design.md`](../control-ide-design.md), [`customer-ide-ux.md`](../customer-ide-ux.md), [`local-development-mac.md`](../local-development-mac.md), ADR-012, ADR-021–024, ADR-027, ADR-028, ADR-030 | Product Go packages unless the task is **explicitly** cross-plane and a backend playbook is attached |

Control IDE is **vendor plane** (ADR-012). It never ships in product images. If a panel needs a new API capability, hand off to `api-families` / `authz-security` / `deploy-ops` — do not embed AuthZ or Deploy logic in the IDE.

## Where to look

| Concern | Path |
|---|---|
| ADR / product decision | [`docs/adr/012-customer-repo-and-control-ide.md`](../adr/012-customer-repo-and-control-ide.md) · [ADR-030](../adr/030-install-agent-runtime.md) |
| Customer IDE UX (modes, tiles, stream) | [`docs/customer-ide-ux.md`](../customer-ide-ux.md) |
| Run mode + ToolSpec | [ADR-021](../adr/021-run-mode-toolspec.md) / [BP-050](../../backlog/BP-050-run-mode-toolspec.md) — no chrome expansion |
| Run personal graph | [ADR-023](../adr/023-run-personal-graph.md) / [BP-055](../../backlog/BP-055-run-personal-graph.md) — refs-only |
| Run graph CRM interactions | [ADR-024](../adr/024-run-graph-interactions.md) / [BP-056](../../backlog/BP-056-run-graph-crm-interactions.md) |
| Collection nodes | [ADR-027](../adr/027-run-graph-collection-nodes.md) — remainders frozen |
| Operate graph surface | [ADR-028](../adr/028-operate-graph-surface.md) / [BP-060](../../backlog/BP-060-operate-graph-surface.md) |
| Inference (install SoR; Settings UI frozen) | [`inference-build-plan.md`](./inference-build-plan.md) — [BP-052](../../backlog/BP-052-customer-inference.md) |
| Agent section harness | [`agent-section-harness-build-plan.md`](./agent-section-harness-build-plan.md) — shipped floors (BP-053). Job-class follow-on is Go-only: [`agent-runtime-build-plan.md`](./agent-runtime-build-plan.md) |
| Operate search API | [`cross-object-search-build-plan.md`](./cross-object-search-build-plan.md) — Client `/search` ([BP-043](../../backlog/BP-043-cross-object-search-api.md)) |
| Security audit + trust boundary | [`control-ide-security-audit.md`](./control-ide-security-audit.md) — findings register, IPC/window/CSP rules to preserve |
| Desktop build / CI | [`docs/control-ide-build.md`](../control-ide-build.md) |
| Visual + dual theme | [`docs/control-ide-design.md`](../control-ide-design.md) |
| Mac local runbook | [`docs/local-development-mac.md`](../local-development-mac.md) |
| Install → IDE connect | [`install-ide-connect-build-plan.md`](./install-ide-connect-build-plan.md) — single-Prod URL + first admin |
| IDE ↔ API version compat | [`ide-api-version-compatibility-build-plan.md`](./ide-api-version-compatibility-build-plan.md) — API revision pin + soft product tested-against ([ADR-025](../adr/025-api-revision-versioning.md) / [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md)) |
| Demo-client fidelity (stubs vs shipped APIs) | [`ide-demo-client-uplift-build-plan.md`](./ide-demo-client-uplift-build-plan.md) — **active** honest JWT demo ([BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md)); does not unfreeze license / Operate CRM / update CDN |
| Brand / dual-theme restyle | [`ide-brand-visual-build-plan.md`](./ide-brand-visual-build-plan.md) — **active** navy/gold tokens + sourced SVG marks ([BP-068](../../backlog/BP-068-ide-brand-visual.md)); restyle existing chrome only |
| Install coupling cleanup | [`ide-backend-coupling-review.md`](./ide-backend-coupling-review.md) / [BP-065](../../backlog/BP-065-ide-backend-coupling.md) |
| App README | [`tools/control-ide/README.md`](../../tools/control-ide/README.md) |
| Electron main + IPC | `tools/control-ide/src/main/` |
| Preload bridge | `tools/control-ide/src/preload/preload.ts` |
| React shell + theme | `tools/control-ide/src/renderer/` (`App.tsx`, `theme.ts`, `styles.css`, `ui/`, `icons/`) |
| Workspace chrome | `tools/control-ide/src/renderer/workspace/` |
| Panels (tiles) | `tools/control-ide/src/renderer/panels/` |
| Unit / component tests | `tools/control-ide/src/**/*.test.{ts,tsx}` |
| Live-API contracts | `tools/control-ide/src/**/*.integration.test.ts` |
| Module map row | [`module-map.md`](./module-map.md) |

## Stack (do not invent a parallel frontend)

| Layer | Choice |
|---|---|
| Shell | Electron 43 |
| UI | React 19 + CSS |
| Chat UI | `@assistant-ui/react` + Majesta One custom runtime (`/agents/runs`) |
| Editor | Monaco (YAML) |
| Bundler | Vite 8 + `vite-plugin-electron` |
| Tests | Vitest 4 + Testing Library + jsdom |
| Packaging | electron-builder (dmg / NSIS / AppImage) |

Confirmed in [`tech-stack.md`](../tech-stack.md). Prefer existing patterns over new frameworks.

## What ships today

Agent-first **Customer IDE** chrome:

- **Modes** — **Operate** / **Build** / **Govern** / **Settings** via home launcher (**2×2** grid); centered mode title reopens launcher overlay (no left rail/submenu). Unauthenticated boot shows a full **Sign in** screen (all Connect auth options); tiles are unavailable until a JWT session is active. Legacy `primarySection` and `ide.*` caps alias (`run→operate`, `ship→build`).
- **Env switcher** — top-bar active install context for all API calls and agents (hidden until connected)
- **Workspace** — shared 2-slice board in all modes (1 fills / 2 drag-resize while the button is held; agent beside a tool defaults to 1/3; max 1 tool + 1 agent, select swaps; immersive graph omits generic tile chrome but can be removed from its rail); left hover tool rail; Connect lives inside Settings → Environments
- **Operate** — immersive personal **graph home** (refs-only + hydrate) ensures all accessible objects and describe-derived relationships on entry; declarative **Tools** (ToolSpec) on the left rail click or drop onto the graph; agents with `primarySection=operate` or legacy `run` ([ADR-023](../adr/023-run-personal-graph.md) · [ADR-024](../adr/024-run-graph-interactions.md) · [ADR-028](../adr/028-operate-graph-surface.md))
- **Build** — Objects, Packages, Agents, Tools, Repo, **Deploy pipeline**, and inspect tools; Explorer visualizes objects across packages and can include catalog objects not enabled yet; legacy `ship` agents dock here
- **Govern** — live Client/Metadata admin for principals, integrations, roles, permission sets. Day-2 cloud hosting admin is Deploy API ([BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md)), not new IDE chrome
- **Settings** — launcher tile with Account + Hosting + Inference + **Environments** (Connect) tool rail (`ide.settings*`); agent dock filtered to `primarySection === "settings"` ([inference-build-plan.md](./inference-build-plan.md))
- **Agent stream** — right **hover** dock (44px collapsed; flyout matches left rail width) with mode-filtered agents + name search; **click** opens a **1:1** chat (bind unbound Operate primary or focus existing); × removes open workspace items from either rail
- **Theme** — dark / light toggle with persistence
- **Top bar** — Env switcher (when connected), theme toggle, account chip (initials + name; opens Environments). Change status lives in the footer bar. Operate record find is **graph chrome** (command bar), not a top-bar field that shifts the mode title (API: [cross-object-search-build-plan.md](./cross-object-search-build-plan.md))
- **Change status bar** — Compact single-row Draft / check chips (fixed height across modes)

Operate List View today is a **thin** Account / Contact / Opportunity surface (describe/query + Name-centric CRUD when connected). End-user CRM depth is Client Experience ([BP-040](../../backlog/BP-040-client-experience-oss-kits.md)), not frozen Operate chrome.

Default API base URL in UI: `http://localhost:8080`. Every Majesta One call sends `Authorization: Bearer <one_jwt>`. Effective AuthZ stays on the install (ADR-006). Session file is encrypted (`session.bin`, CIDE-10). Access JWTs expire (~1h); Control IDE silently refreshes with an opaque refresh token ([refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) / [BP-063](../../backlog/BP-063-refresh-token-sessions.md)). Do not lengthen the access JWT.

## What to do (change types)

### A. UI / panel / chrome change

1. Edit under `src/renderer/` (shell in `App.tsx` + `workspace/`; panels as tiles).
2. Keep handlers thin: HTTP via `apiFetch`; no business rules that belong on the Go API.
3. Preserve dual theme tokens in `styles.css` (`html[data-theme="dark|light"]`).
4. New or migrated **tools** (every left-rail panel except Operate graph) must use `ToolSurface` + `ToolToolbar` / `SearchField` in `tools/control-ide/src/renderer/workspace/`. Agent chats use `AgentChatPane` only (composer pinned at the tile bottom).
5. Add or update Vitest component tests next to the module.

### B. IPC / filesystem / git

1. Stay path-safe (`src/main/paths.ts`); never allow escape outside the chosen repo root.
2. Git credentials are separate from the Majesta One JWT.

### C. API contract gap

1. Document the needed family route / scope in the PR or handoff note.
2. Spawn (or ask the parent to spawn) the matching Go agent — do not patch `internal/httpapi` from this playbook.

## Verify

```bash
cd tools/control-ide
npm ci
npm run lint             # eslint (incl. eslint-plugin-security)
npm run audit:high       # high+ GHSA gate (npm audit, OSV fallback if the audit API is down)
npm test                 # unit + component
npm run test:coverage    # CI thresholds
npm run build            # tsc + vite + main/preload
npm run smoke:electron   # Electron trust-boundary harness (needs the build)
# optional, API must be running:
npm run test:integration
```

Changing `src/main/`, `src/preload/`, or anything that opens a window / touches the filesystem: re-run `npm run smoke:electron` and keep [`control-ide-security-audit.md`](./control-ide-security-audit.md) accurate.

Or from repo root: `make test-ide` / `make test-ide-integration`.

## CRM Canvas — superseded by Run mode

The Operate `canvas/` module, agent bridge tools (`canvas.create|update|rerun|get|list`), `ChatCanvasRefBlock`, `CanvasesPanel`, `canvasSpecs` Build panel, and `boardHandoff.canvasId` were **removed** from Control IDE.

Product delivery of declarative in-IDE business surfaces is **Run mode + ToolSpec** ([ADR-021](../adr/021-run-mode-toolspec.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md)). Historical contract: [ADR-018](../adr/018-crm-canvas-document.md). Do **not** re-introduce an Operate canvas tile.

## Explicit non-goals

- Hosting Control IDE as a browser SaaS
- Embedding the IDE in ECS / Marketplace images
- Replacing Electron with Tauri / an editor fork
- Putting customer fixtures or AuthZ enforcement inside the IDE
- Running Go `make ci` for IDE-only changes (path-filtered CI already skips product `go test`)
- **New product chrome** — frozen under ADR-030 (graphs, docks, license, update channel, Operate record UX)
- **Customer-built IDE components / plugin host** — do not load customer React/Electron code, remote scripts, or iframe “app exchange” panels. Customize via metadata + vendor panels; see ADR-012 amendment and [customer-ide-ux.md](../customer-ide-ux.md)
- **Operate canvas revival** — use Run Tools ([BP-050](../../backlog/BP-050-run-mode-toolspec.md)) if touching existing code. **Client Experience** (customer browser apps) remains [ADR-019](../adr/019-client-experience-oss-kits.md) · [client-experience-build-plan.md](./client-experience-build-plan.md). Builder DX is MCP + CLI, not a new admin SPA.
