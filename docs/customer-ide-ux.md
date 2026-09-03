# Customer IDE — Agent-First UX Spec

**Frozen chrome ([ADR-030](./adr/030-install-agent-runtime.md)).** This spec describes the optional Control IDE client. Do not expand graphs, docks, or license UX. Builder DX: [builder-connect.md](./builder-connect.md). End-user DX: [ADR-019](./adr/019-client-experience-oss-kits.md).

Human-facing name for Majesta One Control (`tools/control-ide`): the vendor-plane Electron client for a Majesta One install ([ADR-012](./adr/012-customer-repo-and-control-ide.md)). Visual tokens and updates: [control-ide-design.md](./control-ide-design.md). Agent playbook: [architecture/agent-control-ide.md](./architecture/agent-control-ide.md).

## Product job

Make **business process change feel like a feature change**:

```text
intent → agent plan → reviewable artifacts → act / promote → thread of record
```

The IDE is API-thin. AuthZ, Deploy, and agent runtime stay on the Go install. The UI composes Client / Metadata / Deploy / Auth surfaces and binds them to a persistent **Agent Stream** plus typed **tiles**.

**Operate thesis (amended):** **Operate** is the daily-driver **personal graph** home — glanceable nodes, collection/list **work sheets**, drop-to-mount ToolSpecs, Pin / Wire / Apply, and a **graph command bar** for find ([ADR-023](./adr/023-run-personal-graph.md) · [ADR-027](./adr/027-run-graph-collection-nodes.md) · [ADR-028](./adr/028-operate-graph-surface.md) · [BP-055](../backlog/BP-055-run-personal-graph.md) · [ADR-024](./adr/024-run-graph-interactions.md) · [BP-056](../backlog/BP-056-run-graph-crm-interactions.md) · [BP-059](./adr/027-run-graph-collection-nodes.md) · [BP-060](../backlog/BP-060-operate-graph-surface.md)). **Build** is the unified builder/release/inspect tile: metadata panels, Deploy pipeline, and **Query / Monitor / Explorer** chat agents (one chat = one agent). Declarative in-IDE business **Tools** mount on the Operate graph ([ADR-021](./adr/021-run-mode-toolspec.md) · [BP-050](../backlog/BP-050-run-mode-toolspec.md)). Frozen chrome: [ADR-030](./adr/030-install-agent-runtime.md) · [ADR-028](./adr/028-operate-graph-surface.md).

**Launcher IA (shipped):** four tiles in a **2×2** grid — **Operate** (graph), **Build** (metadata + ship + inspect), **Govern**, **Settings** (launcher rename from Account; **Environments** live here only). Legacy `primarySection` and `ide.*` caps alias for compat (`run→operate`, `ship→build`).

## Mental model: Change for everyone

| Engineer metaphor | Business user (Operate) | Admin / builder (Build) |
|---|---|---|
| Branch / PR | Working session on personal graph | Metadata change + deploy |
| Diff | Staged proposals / graph pins | YAML / package diff |
| CI checks | Agent tool calls + policy gates | Validate / tests |
| Review | Apply proposal on graph | Approve promote |
| Merge | Client API actions | Deploy promote |
| Thread | Graph + ToolSpec sessions | Change / Promotion thread |

Status language (shared muscle memory): **Draft → Running checks → Needs review → Ready → Promoted / Applied**.

## Personas as modes (not separate apps)

| Mode | Who | Primary tiles | Typical agents |
|---|---|---|---|
| **Operate** | Business users | **Personal graph home** + ToolSpecs / List View on left rail | Run-capable AgentSpecs (`primarySection=operate` or legacy `run`) |
| **Build** | Metadata developers / release operators | Objects, Packages, Agents, Tools, Repo, Deploy, Query / Monitor / Explorer | MetadataBuilder, Deploy, query agents (`primarySection=build` or legacy `ship` / `operate`) |
| **Govern** | Identity / integration admins | Users, Integrations, Permissions | AdminSetup-style |
| **Settings** | Install operators | Account, Hosting, Inference, **Environments** (Connect); org **License** activation is frozen ([ADR-030](./adr/030-install-agent-runtime.md)) | AccountGuide-style |

Nav and default tiles adapt by JWT family scopes (`client`, `metadata`, `deploy`, plus identity admin capabilities) and `ide.*` capability grants. A sales user should not see Deploy chrome. Operate graph tools require `ide.run.tools` (legacy `ide.run` opens the tile; tool caps gate the rail).

## Information architecture

**Initial load — auth then mode launcher:** when not connected, a full-shell **Sign in** screen presents every Connect auth path (claim, browser SSO/password, client credentials, JWT). After a JWT session is active, four large tiles appear in a **2×2** grid (**Operate / Build**, **Govern / Settings**) with tagline + description — no scroll required to pick a tile. Selecting one enters the workspace. The encrypted session is restored when the app is reopened (OS keyring, or a local AES-GCM file when the keyring is missing). Clearing the session returns to Sign in; authenticated users do not see the login screen again until they sign out or the JWT is rejected.

**After selection:**

```text
┌─ Top bar: brand · [centered Mode title] · theme · account ───────┐
├─44px─┬─ Workspace (2 vertical slices) ─────────────┬─44px───────┤
│ Tool │  1 tile = full · 2 = resize (swap on select)│ Agent     │
│ rail │  Operate: graph home seeded (runGraph)       │ dock      │
│hover │  Build: panels + inspect chats as slices      │ hover     │
│      │  Govern / Settings: config panels               │           │
└──────┴─────────────────────────────────────────────┴───────────┘
│ Bottom: active Change · checks · Mark ready                      │
```

Hover + click the centered **Mode title** to reopen the launcher tiles (animated overlay). Left **tool rail** and right **agent dock** stay collapsed at 44px and reveal detail on hover (flyout overlay). No left mode rail or submenu — business users can stay in Operate all day.

### Top bar

- Brand: **Majesta One Control** (product name in prose / window title). **Today** the top-left is typed text. **Target ([BP-068](../backlog/BP-068-ide-brand-visual.md)):** globe symbol top-left plus optional “Control”; full `MAJESTA.NET` SVG lockup only on Sign in and the mode launcher (lockup stays ≥ 180px). Do not type the wordmark.
- **Centered mode title** — hover animates; click brings launcher tiles back. Geometry is **identical in every mode** (no Operate-only search field in this bar)
- Env switcher, theme toggle (dark / light, `Ctrl/Cmd+Shift+L`), account chip (initials + username) → Environments
- Change / update status lives in the footer status bar (not the top-right)
- **Operate find** lives on the graph as a command bar (`Ctrl/Cmd+K`), not in this header ([ADR-028](./adr/028-operate-graph-surface.md))

### Workspace (middle)

A shared **2-slice vertical board** for all modes. One tile fills the board; two tiles share a drag-resize handle that only moves while the primary button is held (max 2: one tool + one agent). An agent opening beside a tool defaults to **1/3** of the board (tool 2/3). Selecting another tool or agent **swaps** the existing interaction of that kind. Tile chrome × closes ordinary slices; immersive Operate **My graph** omits generic tile chrome and can be removed from the Tool rail. The board can still be fully empty.

| Tile | Default span | Notes |
|---|---|---|
| Agent chat | Vertical slice | Always one agent per tile; max 1 agent — clicking another agent swaps it. Beside a tool, defaults to 1/3 width |
| Object Manager (Build) | Vertical slice | Open from left tool rail — searchable list (label / plural / API name) + detail; Metadata API + optional YAML dual-write; max 1 tool (swap on select) |
| Packages (Build) | Vertical slice | List managed packages and enable/disable optionals (Messages, Sales, …) via Metadata packages API |
| Agents (Build) | Vertical slice | Declarative AgentSpecs / playbooks — section harness wizard on create; dock home = `primarySection` ([BP-053](../backlog/BP-053-agent-section-harness.md)); Metadata API + optional YAML dual-write |
| Repo (Build) | Vertical slice | Choose local folder → Initialize remote → Pull from org → Open in an editor (edit/commit elsewhere) |
| Deploy pipeline (Build) | Vertical slice | Open from Build left rail |
| Govern config (Users / Integrations / Permissions) | Vertical slice | Open from left tool rail |
| Settings (Account / Hosting / Inference / Environments) | Vertical slice | Open from Settings left rail; Environments owns topology + Connect ([ADR-030](./adr/030-install-agent-runtime.md)) |
| Query / Monitor / Explorer (Build) | Vertical slice | Inspect tools from Build left rail — metadata query console, user TraceFlag log tail, object graph ([ADR-030](./adr/030-install-agent-runtime.md)); selecting another tool swaps |
| Operate graph + ToolSpec | Immersive vertical slice | Operate seeds **runGraph** on entry without generic tile chrome; the rail can still remove it. Every accessible object appears as a connected collection. ToolSpecs **drop onto the graph** as `tool` nodes (click-while-graph-open mounts; “Open as board” may still swap) ([ADR-028](./adr/028-operate-graph-surface.md)) |
| Classic CRM board | — | **Removed from default Operate IA** (optional Phase 6 sheet only) |

Mode tools live on the **left hover rail** (not top chips). Catalog cards show **one title** plus summary (no id/name kicker that repeats the label). Long catalogs **scroll quietly** (wheel/trackpad; scrollbar only on hover). Workspace tools except Operate graph use the shared `ToolSurface` frame ([agent-control-ide.md](./architecture/agent-control-ide.md)): one header, actions left / search right, body scroll. Tile chrome is close-only so the panel title is not repeated. **Operate** seeds **runGraph** on entry; rail chips are graph home, List View (factory until collections are habitual), and ToolSpecs as a **droppable palette**. **Build** rail lists metadata panels, Deploy, and Query / Monitor / Explorer inspect tools. **My graph** command bar finds records (`POST /client/v1/search`) and lands on a **collection** sheet; Pin is explicit. Collection cards glance in-place; click opens List View in a docked work sheet ([ADR-027](./adr/027-run-graph-collection-nodes.md) · [ADR-028](./adr/028-operate-graph-surface.md)). Record pins still enter through that list **Pin to graph**. Dropping a ToolSpec onto the canvas `graph.mountTool`s it; the graph tile stays.

### Agent stream (right dock)

- Collapsed **44px** rail by default; hover/focus reveals catalog flyout (same width as left tool rail)
- Search by agent name
- Catalog filtered to the **active mode**
- Click agent → open, bind, or **swap** the single **1:1** chat for that agent; never multi-attach
- Catalog cards show one title (drop agentName when it duplicates the title)
- Agent composer is pinned to the bottom of the chat tile in **every** mode (Build included) via `AgentChatPane`
- Open workspace tools/agents show a × in the hovered rail to remove them from the board
- Rich transcript chrome lives primarily in the workspace chat (assistant-ui + Majesta One parts); dock is catalog-first over time

### Adaptive harness (summary)

Competitive pattern: adaptive harness (context + permissions + tools + handoffs), not prettier bubbles alone. Majesta One wedge: BoardHandoff → **inline chat working set**.

### Change status bar (bottom)

Single compact horizontal strip (fixed ~2.35rem): Draft/status badge, check counts, and inline check chips. Does **not** vertically stack Running/Pending/Passed groups — height stays constant across Operate / Build / Govern / Settings.

## Stream + tiles (why)

Majesta One work is multi-artifact (chat + records + YAML + deploy). A single panel forces context switching; a pure chat app hides structure. Stream-driven tiles let the agent propose panes (“open Priority Board”) while the human keeps spatial control.

**Layout presets (documented; Operate seeds primary chat):**

| Preset | Mode | Tiles |
|---|---|---|
| Graph home | Operate | runGraph full; dock catalog |
| Inspect + chat | Build | Query / Monitor / Explorer + agent chat |
| Edit + Diff | Build | Metadata (+ Repo) |
| Deploy | Build | Deploy pipeline |

## Signature flows

### A. Business — prioritize accounts

1. Operate → primary chat already open; **Use** a customer query AgentSpec from the dock (binds primary 1:1).
2. Composer: “Which accounts should I review today?”
3. Stream/chat shows plan + tool steps (query Accounts / Opportunities).
4. Agent lands **inline matching records** via BoardHandoff — optional “what to do” chips.
5. Approvals for outbound / mutations inline; evidence on the thread; Change bar tracks Draft → Needs review → Applied.

### B. Admin — metadata then deploy (environment-aware)

1. Connect with admin + deploy. Top-bar **env switcher** sets the active install (the org you validate/deploy against).
2. Build → **Repo**: **Initialize remote** once (typically from prod) → **Sync local** (clone or pull `origin/main`). Sample init stays under Advanced for offline local-dev.
3. Switch to **test** for day-to-day work. Optional `change/<slug>` branch.
4. Build → **Objects** / **Automations** (Metadata API + YAML/TS dual-write into `metadata/` / `src/` — never `.one/baseline`).
5. Build → **Repo**: commit allowlisted paths → optional Push to customer Git (GitHub/GitLab/Bitbucket/…).
6. Ship → **Validate vs org** (always first; diff + Deploy gates; no org mutation). Run suites until green.
7. Ship → **Deploy to org** (only after green validate for this HEAD) — applies **from the repo pack** to the **connected** install.
8. Other environments: **switch env** (or CI with that base URL), checkout the **same Git SHA**, validate → deploy again. Do **not** peer-promote bundles install→install.

Zip upload / import export stay under Advanced (air-gap). Peer push is removed — multi-env is switch org + validate/deploy ([customer-developer-workflow.md](./customer-developer-workflow.md)).

### C. Admin — users & integrations

1. Settings → Environments (connect / refresh JWT). Govern → Users · Integrations · Permissions.
2. Optionally drop **one** AdminSetup-style agent beside the config column.
3. Live Client / Metadata admin APIs for principals, integrations, roles, and permission sets.

### D. Admin — hosting (Path A day-2)

1. Launcher → **Account** → **Hosting** (capability `ide.settings` + `ide.settings.hosting`) — plan: [ADR-030](./adr/030-install-agent-runtime.md) · [BP-051](./adr/030-install-agent-runtime.md).
2. Bind / scale App Platform app, resize Managed Postgres, provision peer envs via Deploy `/deploy/v1/cloud/*` (JWT only).
3. Settings **Environments** remains topology + Connect — not cloud scale admin (relocation from EnvPanel).

## Theme

Dual theme via `data-theme="dark" | "light"` on the document element. Owned CSS tokens only. Accent teal retained. Preference persisted in `localStorage` (`one.control.theme`); first launch follows `prefers-color-scheme`. Monaco follows theme (`vs-dark` / `vs`). Details: [control-ide-design.md](./control-ide-design.md).

## Plane fence & non-goals

- No AuthZ or Deploy business logic in the IDE
- Not a classic admin UI clone; not browser SaaS hosting
- Slack bridge / outbound email deferred as **product transport** — Message ingest + connector adapters are the path; do not ship in-kernel CTI/SMTP
- Prefer existing Client agent-run APIs (`/client/v1/agents/runs`) for live stream wiring
- **No customer Electron / React plugins** — customers do not ship IDE UI code. Customization stays in `one/v1` metadata (objects, fields, rules, automations, AgentSpecs, ToolSpecs, tests). The vendor IDE renders those declaratively (allowlisted Tool/Canvas node kinds — [ADR-018](./adr/018-crm-canvas-document.md) · [ADR-021](./adr/021-run-mode-toolspec.md)). Local dedicated installs do **not** make arbitrary renderer code loading safe (JWT session, FS IPC, OS privileges). Future “plugins” (if any) follow [BP-009](../backlog/BP-009-no-in-kernel-language.md) as sandboxed **server-side** executables on the install — not desktop UI bundles, iframes, or remote code in the Electron renderer. Distribution/signing stays vendor-controlled ([BP-015](./adr/030-install-agent-runtime.md)).
- **No OSS browser Experience host as Run/Operate surface** — interactive Tools live in Control IDE **Run** mode ([ADR-021](./adr/021-run-mode-toolspec.md)); do not treat a same-origin `/x` SPA as a second IDE. **Customer end-user browser apps** are a separate **Client Experience** track ([ADR-019](./adr/019-client-experience-oss-kits.md) · [client-experience-build-plan.md](./architecture/client-experience-build-plan.md) · [BP-040](../backlog/BP-040-client-experience-oss-kits.md)).

## Implementation map

| Area | Path |
|---|---|
| Shell | `tools/control-ide/src/renderer/App.tsx` |
| Theme | `tools/control-ide/src/renderer/theme.ts` |
| Workspace chrome | `tools/control-ide/src/renderer/workspace/` |
| Run Tools | `tools/control-ide/src/renderer/run/` — [BP-050](../backlog/BP-050-run-mode-toolspec.md) · [ADR-021](./adr/021-run-mode-toolspec.md) |
| Run personal graph | Shipped `tools/control-ide/src/renderer/run/graph/` — [BP-055](../backlog/BP-055-run-personal-graph.md) · [ADR-023](./adr/023-run-personal-graph.md) |
| Run graph CRM interactions | Active — focus / wire / My day queue / proposals / Operate→Run handoff — [BP-056](../backlog/BP-056-run-graph-crm-interactions.md) · [ADR-024](./adr/024-run-graph-interactions.md) |
| CRM Canvas (historical) | Removed from IDE; superseded by Run — [BP-039](./adr/018-crm-canvas-document.md) · [BP-050](../backlog/BP-050-run-mode-toolspec.md) |
| Tokens | `tools/control-ide/src/renderer/styles.css` |
| Fonts | `tools/control-ide/assets/fonts/` |
| Icons / primitives | `tools/control-ide/src/renderer/icons/`, `tools/control-ide/src/renderer/ui/` |
| Panels (tiles) | `tools/control-ide/src/renderer/panels/` |
| Capability chrome | `tools/control-ide/src/renderer/scopes.ts` |
| Agent runs client | `tools/control-ide/src/renderer/agents/runs.ts` |

### Six-tile premium baseline (2026-08)

> **Shipped four-tile IA (frozen):** Operate / Build / Govern / Settings (2×2). Do not reopen six-tile chrome or extra docks ([ADR-030](./adr/030-install-agent-runtime.md)).

| Tile | Shipped baseline | Deliberate remainder |
|---|---|---|
| **Run** | Every enabled Client object generates an interactive Object Home (list, detail, create, edit); customer ToolSpecs remain the dynamic rail | Bulk edit and saved view depth follow the shared record/search tracks |
| **Operate** | Client-safe AgentSpec catalog, direct SSE token rendering, approval fallback, and rich Markdown transcript | Hosted multi-tool loop, reconnect/cancel, search, and channel send contracts remain BP-006/BP-020/BP-024 |
| **Build** | Curated **Open pipeline**, **Case queue + triage**, and **Quote follow-up** starter packs clone coherent ToolSpec + AgentSpec + automation metadata and mirror YAML when a repo is linked | A broader managed recipe marketplace remains product work, not customer hand assembly |
| **Ship** | Deploy is hard-gated on validation + customer tests and exposes structured release evidence (change counts, issues, baseline, checksum) | ExecutionRun-backed durable release history remains BP-033 |
| **Govern** | Connected Apps plus catalog-driven outbound connectors with guided secret, egress, and OAuth setup | Provider-specific catalogs and channel transports stay adapters |
| **Account** | Live session, workspace, access, and shortcut dashboard alongside Hosting/Inference | Provider account lifecycle and BP-027 OAuth remain |

### Polish phases

| Phase | Status | Notes |
|---|---|---|
| Phase 1 shell (modes, stream stubs, demo status) | Shipped | Agent-first IA |
| Phase 2 premium chrome + panel productization | Shipped | See [BP-016](./adr/030-install-agent-runtime.md); no default JSON primary views |
| Customer-repo loop (dirty → Metadata, pack HEAD, status bar) | Shipped | Repo deep-link + Deploy checks trail |
| Capability-gated Operate + live CRM describe/query | Shipped | JWT scope-gated modes/tools; thin Account/Contact/Opportunity board when connected — **not** full record UX |
| Live agent runs | Shipped | Chat consumes `/client/v1/agents/runs` SSE token events; streaming create skips pre-LLM Approve; Settings → Inference includes a Test chat probe; mutations stay gated (Apply / BP-006) |
| Env-aware Build/Ship loop (switcher, Object Manager, clone/editor, repo→org) | Shipped | [BP-023](../backlog/BP-048-one-cli.md) · [BP-048](../backlog/BP-048-one-cli.md) (env YAML stage order, selective Ship, New change) |
| Validate/deploy local repo vs org (validate-first; repo→org only) | Shipped | [customer-dx-build-plan.md](./architecture/customer-dx-build-plan.md) · [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md) · [customer-developer-workflow.md](./customer-developer-workflow.md) |
| Operate record UX (forms, filters, views, related lists, Activity Feed) | Frozen | [BP-018](./adr/030-install-agent-runtime.md) |
| Agent→board handoff MVP (“open board from run”, what-to-do chips) | Partial | [BP-024](./adr/030-install-agent-runtime.md) A — historical CRM landing |
| Operate domain surfaces (Sales pipeline/Quote, Service Case queue, Messages) | Partial | [BP-019](./adr/030-install-agent-runtime.md) |
| Chat-first Operate (no default CRM tile; primary chat; agents-into-chat; density; inline blocks) | Frozen | SSE/rich transcript and Client AgentSpec catalog shipped; BP-024 B retarget is frozen ([ADR-030](./adr/030-install-agent-runtime.md)) |
| Operate search + bulk actions | Mitigated (API + find/bulk shipped) | [BP-020](../backlog/BP-043-cross-object-search-api.md) · [BP-043](../backlog/BP-043-cross-object-search-api.md) — Client `/search` + Object Home bulk. Find lives in the **graph command bar** via [BP-060](../backlog/BP-060-operate-graph-surface.md) |
| Channel adapters (Message ingest, screen-pop, connectors — not in-kernel CTI/email) | Partial | Govern connector catalog shipped; Message ingest, screen-pop, and send contract remain [BP-024](./adr/030-install-agent-runtime.md) C |
| Operate reporting / team coaching views | Open | [BP-021](./adr/030-install-agent-runtime.md) |
| Settings Account + Hosting tools (move cloud admin out of Ship Env) | Partially mitigated | [BP-051](./adr/030-install-agent-runtime.md) · [ADR-030](./adr/030-install-agent-runtime.md) — live session/workspace/access dashboard + Hosting relocation shipped; provider lifecycle remains |
| Multi-tile canvas / Split chat | Deferred | Default remains primary Operate chat. CRM Canvas IDE surface removed pending readiness — see [BP-039](./adr/018-crm-canvas-document.md) · [ADR-018](./adr/018-crm-canvas-document.md) |
| Run personal graph foundation | Shipped | [BP-055](../backlog/BP-055-run-personal-graph.md) · [ADR-023](./adr/023-run-personal-graph.md) |
| Run graph CRM interactions (Pin / Wire / Apply) | Shipped (hosted curator remains BP-006) | [BP-056](../backlog/BP-056-run-graph-crm-interactions.md) · [ADR-024](./adr/024-run-graph-interactions.md) |
| Run graph collection nodes | Partial (Phase 1 shipped) | [BP-059](./adr/027-run-graph-collection-nodes.md) · [ADR-027](./adr/027-run-graph-collection-nodes.md) |
| Operate graph surface (accessible-object model, glance, connections, drop Tools, hygiene, graph search) | Mitigated | [BP-060](../backlog/BP-060-operate-graph-surface.md) · [ADR-028](./adr/028-operate-graph-surface.md) |
