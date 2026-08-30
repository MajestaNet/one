export type WorkspaceMode = "operate" | "build" | "govern";

/** Shell section: three mode workspaces plus Settings (launcher tile). */
export type AppSection = WorkspaceMode | "settings";

export type TileId =
  | "agents"
  | "client"
  | "crm"
  | "query"
  | "monitor"
  | "explorer"
  | "runGraph"
  | "runTool"
  | "objectHome"
  | "objects"
  | "packages"
  | "agentSpecs"
  | "tools"
  | "automations"
  | "metadata"
  | "repo"
  | "deploy"
  | "connect"
  | "env"
  | "users"
  | "integrations"
  | "experiences"
  | "installAuth"
  | "permissions"
  | "govern"
  | "account"
  | "hosting"
  | "inference";

export type ChangeStatus = "draft" | "running" | "needs_review" | "ready" | "applied" | "promoted";

export type StreamRole = "human" | "agent" | "system" | "tool" | "approval";

export type StreamStep = {
  id: string;
  label: string;
  state: "pending" | "running" | "done" | "failed";
};

export type StreamMessage = {
  id: string;
  role: StreamRole;
  body: string;
  tileAction?: TileId;
  tileActionLabel?: string;
  /** Agent run id when this bubble came from /agents/runs. */
  runId?: string;
  /** Structured board handoff for Operate CRM (BP-024). */
  boardHandoff?: import("../operate/types").BoardHandoff;
  /** Structured agent → Run Tool handoff (ADR-021 Phase 4). */
  toolHandoff?: import("../run/toolHandoff").ToolHandoff;
  /** ISO timestamp for transcript chrome. */
  createdAt?: string;
  /** Last known run status for lifecycle chrome. */
  runStatus?: string;
  /** Planned / executed tool names from run output. */
  toolsPlanned?: string[];
  /** Collapsible plan / tool steps. */
  steps?: StreamStep[];
  /** Display name for agent role bubbles. */
  agentLabel?: string;
  /** Selection / table rows attached as chat context (Agentic Run uplift). */
  contextExcerpts?: import("./contextExcerpt").ContextExcerpt[];
  /**
   * When true, Approve applies IDE tool/graph effects for an already-completed
   * LLM run (does not POST /agents/runs/{id}/approve).
   */
  pendingToolApply?: boolean;
};

export type AgentChat = {
  id: string;
  title: string;
  /**
   * Bound playbook apiName. Empty string when Operate primary is unbound
   * ("Choose an agent").
   */
  agentName: string;
  summary: string;
  /** Modes where this agent appears in the stream catalog (includes Account/settings). */
  modes: AppSection[];
  messages: StreamMessage[];
  /** Operate primary working thread — not listed as a dock card. */
  primary?: boolean;
  /** Active BoardHandoff applied into this chat session. */
  sessionHandoff?: import("../operate/types").BoardHandoff | null;
  /** Offline demo stub (not a live customer AgentSpec). */
  demoStub?: boolean;
  /** Server-backed conversation id when persisted via Client API. */
  conversationId?: string;
};

/** Stable id for the Operate session chat (handoff surface; not seeded on entry). */
export const PRIMARY_OPERATE_CHAT_ID = "operate-primary";

export const PRIMARY_OPERATE_CHAT: AgentChat = {
  id: PRIMARY_OPERATE_CHAT_ID,
  title: "Build session",
  agentName: "",
  summary: "Ask questions about the active install — choose one agent from the dock.",
  modes: ["build"],
  primary: true,
  sessionHandoff: null,
  messages: [
    {
      id: "op1",
      role: "system",
      body: "Choose an agent from the dock to start. One chat talks to one agent — ask questions about the active environment; matching records land inline.",
      createdAt: new Date(0).toISOString(),
    },
  ],
};

export type CheckItem = {
  id: string;
  label: string;
  state: "running" | "passed" | "pending" | "failed";
  duration?: string;
};

export type WorkspaceTileKind = "chat" | "crm" | "panel";

/** A tile placed in the middle Workspace canvas. */
export type WorkspaceTile = {
  id: string;
  kind: WorkspaceTileKind;
  /** Present when kind === "chat" */
  chatId?: string;
  /** Present when kind === "panel" */
  panelId?: TileId;
  colSpan: 1 | 2;
  rowSpan: 1 | 2;
};

export const AGENT_CHAT_MIME = "application/x-one-agent-chat";

/** Re-export for drag payloads — see contextExcerpt.ts */
export { CONTEXT_EXCERPT_MIME } from "./contextExcerpt";

/** Launcher tile order: Operate · Build / Govern · Settings (2×2). */
export const MODES: {
  id: WorkspaceMode;
  label: string;
  tagline: string;
  description: string;
}[] = [
  {
    id: "operate",
    label: "Operate",
    tagline: "Graph · Tools · daily work",
    description:
      "Personal graph home, declarative Tools, and List View for daily work. Pin, wire, and apply on the active install — AuthZ stays on the server.",
  },
  {
    id: "build",
    label: "Build",
    tagline: "Objects · deploy · inspect",
    description:
      "Shape metadata, validate and deploy from the customer repo, and inspect the install with Query / Monitor / Explorer. Agents help across the change loop.",
  },
  {
    id: "govern",
    label: "Govern",
    tagline: "Users · integrations · permissions",
    description:
      "Manage principals, integrations, and permission sets under explicit admin control.",
  },
];

/** Fourth launcher tile — Settings section (Account + Hosting + Inference + Environments). */
export const ACCOUNT_LAUNCHER = {
  id: "settings" as const,
  label: "Settings",
  tagline: "Account · hosting · environments",
  description:
    "Session preferences, cloud hosting admin, inference providers, and install Environments for the active workspace.",
};

export type LauncherTileId = WorkspaceMode | "settings";

export const TILE_META: Record<
  TileId,
  { label: string; modes: WorkspaceMode[]; /** Short catalog blurb under the title. */ summary: string }
> = {
  agents: { label: "Agents", modes: ["build"], summary: "Browse and bind agents for this mode." },
  client: {
    label: "Records / Query",
    modes: ["build"],
    summary: "Query and inspect records on the active install.",
  },
  crm: { label: "CRM", modes: ["build"], summary: "Account, Contact, and Opportunity board." },
  query: {
    label: "Query",
    modes: ["build"],
    summary: "Run record queries against the active install.",
  },
  monitor: {
    label: "Monitor",
    modes: ["build"],
    summary: "Tail execution runs and runtime signals.",
  },
  explorer: {
    label: "Explorer",
    modes: ["build"],
    summary: "Browse objects and fields on the connected org.",
  },
  runGraph: {
    label: "My graph",
    modes: ["operate"],
    summary: "Your connected view of objects, records, notes, and Tools.",
  },
  runTool: { label: "Tool", modes: ["operate"], summary: "Open a declarative ToolSpec surface." },
  objectHome: {
    label: "List View",
    modes: ["operate"],
    summary: "Browse records for any object with columns, filters, and multi-select.",
  },
  objects: {
    label: "Objects",
    modes: ["build"],
    summary: "Shape object and field metadata on the active env.",
  },
  packages: {
    label: "Packages",
    modes: ["build"],
    summary: "Enable optional managed modules for this install.",
  },
  agentSpecs: {
    label: "Agents",
    modes: ["build"],
    summary: "Author AgentSpecs and playbooks via Metadata.",
  },
  tools: {
    label: "Tools",
    modes: ["build"],
    summary: "Declare ToolSpecs for Run-mode business surfaces.",
  },
  /** Metadata automations CRUD (Build rail — BP-066 WS0). */
  automations: {
    label: "Automations",
    modes: ["build"],
    summary: "Create code automations and edit entry TypeScript on the active install.",
  },
  /** Deep-link / advanced YAML editor — not a Build chip. */
  metadata: {
    label: "Metadata YAML",
    modes: ["build"],
    summary: "Raw Metadata YAML editor for deep links.",
  },
  repo: {
    label: "Repo",
    modes: ["build"],
    summary: "Choose folder, init remote, or open the customer repo.",
  },
  deploy: {
    label: "Deploy Pipeline",
    modes: ["build"],
    summary: "Validate, test, and deploy from the customer Git SHA.",
  },
  /** @deprecated Connect lives inside Settings → Environments; kept for deep-link routing. */
  connect: {
    label: "Connect",
    modes: [],
    summary: "Add or refresh credentials for an install.",
  },
  env: {
    label: "Environments",
    modes: [],
    summary: "Install topology, peers, and Connect for the active workspace.",
  },
  users: {
    label: "Users",
    modes: ["govern"],
    summary: "Manage principals on the active install.",
  },
  integrations: {
    label: "Integrations",
    modes: ["govern"],
    summary: "Connected Apps and outbound integrations.",
  },
  experiences: {
    label: "Experiences",
    modes: ["govern"],
    summary: "Client experience surfaces and kits.",
  },
  installAuth: {
    label: "Install auth",
    modes: ["govern"],
    summary: "Install login and identity provider settings.",
  },
  permissions: {
    label: "Permissions",
    modes: ["govern"],
    summary: "Roles, permission sets, and IDE capabilities.",
  },
  /** @deprecated Split into users / integrations / permissions. */
  govern: {
    label: "Users & integrations",
    modes: ["govern"],
    summary: "Legacy combined Govern panel.",
  },
  /** Settings section tools — opened from the Settings launcher tile. */
  account: {
    label: "Account",
    modes: [],
    summary: "Majesta One Control account preferences.",
  },
  hosting: {
    label: "Hosting",
    modes: [],
    summary: "Day-2 cloud hosting admin for the active install.",
  },
  inference: {
    label: "Inference",
    modes: [],
    summary: "BYO model providers and Native DigitalOcean Inference.",
  },
};

/** Soft palette of workspace tools per mode (left hover tool rail). */
export const MODE_WORKSPACE_TOOLS: Record<WorkspaceMode, TileId[]> = {
  /** Graph home + ToolSpecs (former Run rail). */
  operate: ["runGraph", "objectHome"],
  /** Metadata + deploy + inspect (former Build + Ship + Operate inspect). */
  build: ["objects", "packages", "agentSpecs", "tools", "automations", "repo", "deploy", "query", "monitor", "explorer"],
  govern: ["users", "integrations", "experiences", "installAuth", "permissions"],
};

/** Settings section tools (launcher Settings tile). */
export const SETTINGS_WORKSPACE_TOOLS: TileId[] = ["account", "hosting", "inference", "env"];

/** Max vertical slices in the shared workspace board (all modes): 1 tool + 1 agent. */
export const MAX_WORKSPACE_TILES = 2;

/** At most one tool panel (or CRM) — selecting another tool swaps it. */
export const MAX_WORKSPACE_TOOLS = 1;

/** At most one agent chat — selecting another agent swaps it. */
export const MAX_WORKSPACE_AGENTS = 1;

/** Govern config panels (open as workspace slices; metadata is deep-link only elsewhere). */
export const GOVERN_CONFIG_PANELS: TileId[] = ["users", "integrations", "experiences", "installAuth", "permissions"];

/** Build config panels (open as workspace slices; metadata YAML remains deep-link only). */
export const BUILD_CONFIG_PANELS: TileId[] = [
  "objects",
  "packages",
  "agentSpecs",
  "tools",
  "repo",
  "metadata",
  "automations",
];

/**
 * @deprecated Exclusive config split retired — all modes use the 2-slice board.
 * Kept returning false so legacy callers stop branching on exclusive layout.
 */
export function isExclusiveConfigMode(_mode: WorkspaceMode): boolean {
  return false;
}

export function isToolTile(tile: WorkspaceTile): boolean {
  return tile.kind === "panel" || tile.kind === "crm";
}

export function isChatTile(tile: WorkspaceTile): boolean {
  return tile.kind === "chat";
}

/** True when the board already has a tool panel/CRM and cannot add another without swapping. */
export function hasWorkspaceTool(tiles: WorkspaceTile[]): boolean {
  return tiles.some(isToolTile);
}

/** True when the board already has an agent chat. */
export function hasWorkspaceAgent(tiles: WorkspaceTile[]): boolean {
  return tiles.some(isChatTile);
}

/**
 * Canonical board order: tool (left) then agent chat (right).
 * Both select helpers normalize to this so opening a tool while a chat is open
 * never pushes the chat left of the tool.
 */
export function normalizeWorkspaceOrder(tiles: WorkspaceTile[]): WorkspaceTile[] {
  const tools = tiles.filter(isToolTile).slice(0, MAX_WORKSPACE_TOOLS);
  const chats = tiles.filter(isChatTile).slice(0, MAX_WORKSPACE_AGENTS);
  return [...tools, ...chats].slice(0, MAX_WORKSPACE_TILES);
}

/**
 * Select a tool panel: focus if already open; otherwise swap the existing tool
 * (max 1) or add when under the board cap. Always places the tool on the left
 * and any open agent chat on the right.
 */
export function selectToolTiles(
  prev: WorkspaceTile[],
  nextTool: WorkspaceTile,
  maxTiles = MAX_WORKSPACE_TILES,
): WorkspaceTile[] {
  const nextId =
    nextTool.kind === "crm" ? "crm" : nextTool.kind === "panel" ? nextTool.panelId : undefined;
  if (
    nextTool.kind === "crm" &&
    prev.some((t) => t.kind === "crm")
  ) {
    return prev;
  }
  if (
    nextTool.kind === "panel" &&
    nextId &&
    prev.some((t) => t.kind === "panel" && t.panelId === nextId)
  ) {
    return prev;
  }

  const chats = prev.filter(isChatTile).slice(0, MAX_WORKSPACE_AGENTS);
  return normalizeWorkspaceOrder([nextTool, ...chats]).slice(0, maxTiles);
}

/**
 * Select an agent chat: focus if already open; otherwise swap the existing chat
 * (max 1) or add when under the board cap. Always places tools left, chat right.
 */
export function selectAgentTiles(
  prev: WorkspaceTile[],
  nextChat: WorkspaceTile,
  maxTiles = MAX_WORKSPACE_TILES,
): WorkspaceTile[] {
  const chatId = nextChat.chatId;
  if (chatId && prev.some((t) => t.kind === "chat" && t.chatId === chatId)) {
    return prev;
  }
  const tools = prev.filter(isToolTile).slice(0, MAX_WORKSPACE_TOOLS);
  return normalizeWorkspaceOrder([...tools, nextChat]).slice(0, maxTiles);
}

export function exclusiveConfigPanelsForMode(mode: WorkspaceMode): TileId[] {
  if (mode === "govern") return GOVERN_CONFIG_PANELS;
  if (mode === "build") return BUILD_CONFIG_PANELS;
  return [];
}

/** @deprecated — left submenu removed; prefer MODE_WORKSPACE_TOOLS */
export const MODE_SUBMENU = MODE_WORKSPACE_TOOLS;

/** @deprecated alias */
export const MODE_DEFAULT_TILES = MODE_WORKSPACE_TOOLS;

export const SEED_AGENT_CHATS: AgentChat[] = [
  {
    id: "agent-query",
    title: "Query assistant",
    agentName: "QueryAssistant",
    summary: "Ask questions about records on the active install.",
    modes: ["build"],
    demoStub: true,
    messages: [
      {
        id: "q1",
        role: "system",
        body: "Connect to load live customer AgentSpecs. Vertical agents are BYO via Metadata + MCP.",
      },
      {
        id: "q2",
        role: "agent",
        body: "I can help you query and inspect data on this install. Use me to open a one-agent chat.",
      },
    ],
  },
  {
    id: "agent-records",
    title: "Records assistant",
    agentName: "RecordsAssistant",
    summary: "Inspect and summarize records on the active install.",
    modes: ["build"],
    demoStub: true,
    messages: [
      {
        id: "r1",
        role: "system",
        body: "Connect to load live customer AgentSpecs.",
      },
      {
        id: "r2",
        role: "agent",
        body: "I help with record lookup and summaries. Choosing me swaps the Build session to this agent.",
      },
    ],
  },
  {
    id: "agent-run",
    title: "Run coach",
    agentName: "RunCoach",
    summary: "Curate live attention, stage work, and publish proven ToolSpecs.",
    modes: ["operate"],
    demoStub: true,
    messages: [
      {
        id: "run1",
        role: "system",
        body: "Connect to load live customer AgentSpecs with modes including operate.",
      },
      {
        id: "run2",
        role: "agent",
        body: "I switch between Curator (My day, signals, topology) and Doer (staged proposals for human Apply). When a private workflow proves reusable, I suggest graph.publishSubgraph as an org ToolSpec.",
      },
    ],
  },
  {
    id: "agent-meta",
    title: "Metadata builder",
    agentName: "MetadataBuilder",
    summary: "Propose field and automation edits (pairs with Build mode).",
    modes: ["build"],
    demoStub: true,
    messages: [
      { id: "b1", role: "agent", body: "I draft metadata diffs. Use me from Build to open a dedicated chat." },
    ],
  },
  {
    id: "agent-deploy",
    title: "Deploy guide",
    agentName: "DeployBot",
    summary: "Walk pack → validate → tests → deploy to the connected org.",
    modes: ["build"],
    demoStub: true,
    messages: [
      {
        id: "d1",
        role: "agent",
        body: "I narrate validate → deploy on the connected org. Use me when you want a dedicated deploy chat.",
      },
    ],
  },
  {
    id: "agent-admin",
    title: "Admin setup",
    agentName: "AdminSetup",
    summary: "Guide principals, integrations, and module enablement.",
    modes: ["govern"],
    demoStub: true,
    messages: [
      { id: "g1", role: "agent", body: "I help with Govern tasks. Connect first, then ask about principals." },
    ],
  },
  {
    id: "agent-account",
    title: "Account guide",
    agentName: "AccountGuide",
    summary: "Orient Account, hosting, and inference settings for this install.",
    modes: ["settings"],
    demoStub: true,
    messages: [
      {
        id: "acc1",
        role: "agent",
        body: "I help with Account settings — hosting, inference, and your install profile.",
      },
    ],
  },
];

export function statusLabel(status: ChangeStatus): string {
  switch (status) {
    case "draft":
      return "Draft";
    case "running":
      return "Running checks";
    case "needs_review":
      return "Needs review";
    case "ready":
      return "Ready";
    case "applied":
      return "Applied";
    case "promoted":
      return "Promoted";
  }
}

function agentMatchesQuery(chat: AgentChat, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    chat.agentName.toLowerCase().includes(q) ||
    chat.title.toLowerCase().includes(q) ||
    chat.summary.toLowerCase().includes(q) ||
    chat.id.toLowerCase().includes(q)
  );
}

export function agentsForMode(catalog: AgentChat[], mode: AppSection, query = ""): AgentChat[] {
  return catalog.filter((c) => {
    if (c.primary) return false;
    if (!agentMatchesDockMode(c.modes, mode)) return false;
    return agentMatchesQuery(c, query);
  });
}

/**
 * Match agent dock homes including legacy primarySection values.
 * Canonical docks are operate | build | govern | settings. "run" still aliases Operate
 * and "ship" still aliases Build. Bare "operate" is a first-class dock (RunCoach / graph
 * agents) — ingest maps legacy chat operate→Build in dockSectionForPrimarySection, so
 * this helper must not re-home canonical operate agents onto Build.
 */
export function agentMatchesDockMode(agentModes: AppSection[], dock: AppSection): boolean {
  for (const m of agentModes) {
    if (m === dock) return true;
    const legacy = m as string;
    if (dock === "operate" && legacy === "run") return true;
    if (dock === "build" && legacy === "ship") return true;
  }
  return false;
}

/** Per-section workspace tiles (Operate / Build / Govern / Settings). */
export type SectionTileMap = Record<AppSection, WorkspaceTile[]>;

export const EMPTY_SECTION_TILES: SectionTileMap = {
  operate: [],
  build: [],
  govern: [],
  settings: [],
};

export function isAgentBound(chat: AgentChat): boolean {
  return Boolean(chat.agentName.trim());
}

export function defaultChatLayout(): Pick<WorkspaceTile, "colSpan" | "rowSpan"> {
  // Tall vertical chat — default on drop (non-Operate modes).
  return { colSpan: 1, rowSpan: 2 };
}

/** Full-span primary Operate chat. */
export function primaryChatLayout(): Pick<WorkspaceTile, "colSpan" | "rowSpan"> {
  return { colSpan: 2, rowSpan: 2 };
}

export function defaultCrmLayout(): Pick<WorkspaceTile, "colSpan" | "rowSpan"> {
  // Wide top band.
  return { colSpan: 2, rowSpan: 1 };
}

export function defaultPanelLayout(): Pick<WorkspaceTile, "colSpan" | "rowSpan"> {
  return { colSpan: 2, rowSpan: 2 };
}

export function cycleTileLayout(tile: WorkspaceTile): WorkspaceTile {
  // Cycle: tall → wide → quarter → tall
  if (tile.colSpan === 1 && tile.rowSpan === 2) return { ...tile, colSpan: 2, rowSpan: 1 };
  if (tile.colSpan === 2 && tile.rowSpan === 1) return { ...tile, colSpan: 1, rowSpan: 1 };
  if (tile.colSpan === 1 && tile.rowSpan === 1) return { ...tile, colSpan: 2, rowSpan: 2 };
  return { ...tile, colSpan: 1, rowSpan: 2 };
}

export function layoutLabel(tile: Pick<WorkspaceTile, "colSpan" | "rowSpan">): string {
  if (tile.colSpan === 1 && tile.rowSpan === 2) return "Tall";
  if (tile.colSpan === 2 && tile.rowSpan === 1) return "Wide";
  if (tile.colSpan === 2 && tile.rowSpan === 2) return "Full";
  return "Half";
}

/** @deprecated Prefer WorkspaceCanvas layout; kept for older tests. */
export function agentGridClass(count: number): string {
  if (count <= 1) return "agent-grid count-1";
  if (count === 2) return "agent-grid count-2";
  if (count === 3) return "agent-grid count-3";
  return "agent-grid count-4";
}
