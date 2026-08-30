import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { Session } from "./api";
import {
  apiFetch,
  activeConnection,
  isConnected,
  isUnauthorizedError,
  normalizeSession,
  switchActiveEnvironment,
  withActorIdentity,
  withRotatedTokens,
} from "./api";
import {
  approveAgentRunStream,
  createAgentRunStream,
  isTerminalRunStatus,
  listPlaybooks,
  pollAgentRun,
  STREAM_PARKED_HINT,
  type AgentRun,
} from "./agents/runs";
import { modesFromPrimarySection } from "./agents/sections";
import { ThemeContext } from "./ThemeContext";
import { AccountSettingsPanel } from "./panels/AccountSettingsPanel";
import { HostingPanel } from "./panels/HostingPanel";
import { InferencePanel } from "./panels/InferencePanel";
import { AgentsPanel } from "./panels/AgentsPanel";
import { ToolsPanel } from "./panels/ToolsPanel";
import { AutomationsPanel } from "./panels/AutomationsPanel";
import { ClientPanel } from "./panels/ClientPanel";
import { DeployPanel } from "./panels/DeployPanel";
import { EnvPanel } from "./panels/EnvPanel";
import { IntegrationsPanel } from "./panels/IntegrationsPanel";
import { ExperiencesPanel } from "./panels/ExperiencesPanel";
import { InstallAuthPanel } from "./panels/InstallAuthPanel";
import { MetadataPanel } from "./panels/MetadataPanel";
import { ObjectManagerPanel } from "./panels/ObjectManagerPanel";
import { PackagesPanel } from "./panels/PackagesPanel";
import { PermissionsPanel } from "./panels/PermissionsPanel";
import { RepoPanel } from "./panels/RepoPanel";
import { UsersPanel } from "./panels/UsersPanel";
import { ExplorerPanel } from "./operate/ExplorerPanel";
import { MonitorPanel } from "./operate/MonitorPanel";
import { QueryPanel } from "./operate/QueryPanel";
import { RunObjectHomePanel, parseObjectHomeRailId } from "./run/RunObjectHomePanel";
import { RunToolPanel } from "./run/RunToolPanel";
import { RunGraphHome } from "./run/graph/RunGraphHome";
import {
  ProposalStagingStore,
  stageProposalOnGraph,
} from "./run/graph/proposalStaging";
import { pinBoardHandoffToHomeGraph } from "./run/graph/pinRecord";
import { processRunToolEffects, pendingToolActionsFromRun } from "./run/runToolEffects";
import { buildActiveToolContext } from "./run/activeToolContext";
import {
  listSessionTools,
  listClientTools,
  parseSessionToolRailId,
  parseToolRailId,
  sessionToolRailId,
  toolRailId,
  type ToolSpecSummary,
} from "./run/tools";
import { canOpenSettings, modesForScopes, toolsForMode, toolsForSettings } from "./scopes";
import {
  applyTheme,
  persistTheme,
  resolveInitialTheme,
  toggleTheme,
  type Theme,
} from "./theme";
import type { BoardHandoff } from "./operate/types";
import { AuthScreen } from "./AuthScreen";
import { CONTROL_IDE_INTEGRATION } from "./oauthPkce";
import { accessTokenNearingExpiry, refreshAccessToken } from "./refreshSession";
import { mergeStreamedRunReplies, messagesFromRun, runningAgentPlaceholder } from "./workspace/messageModel";
import { AgentStreamDock } from "./workspace/AgentStreamDock";
import { ChangeStatusBar } from "./workspace/ChangeStatusBar";
import { EnvSwitcher } from "./workspace/EnvSwitcher";
import { SessionChip } from "./workspace/SessionChip";
import { ModeLauncher } from "./workspace/ModeLauncher";
import { ModeTitle } from "./workspace/ModeTitle";
import {
  makeChatTile,
  makePanelTile,
  makePrimaryChatTile,
  WorkspaceCanvas,
} from "./workspace/WorkspaceCanvas";
import { WorkspaceToolRail } from "./workspace/WorkspaceToolRail";
import { workspaceRailClassNames, workspaceRailDocking } from "./workspace/workspaceRailLayout";
import {
  EMPTY_SECTION_TILES,
  MAX_WORKSPACE_TILES,
  PRIMARY_OPERATE_CHAT,
  PRIMARY_OPERATE_CHAT_ID,
  SEED_AGENT_CHATS,
  TILE_META,
  hasWorkspaceTool,
  isAgentBound,
  normalizeWorkspaceOrder,
  selectAgentTiles,
  selectToolTiles,
  type AgentChat,
  type AppSection,
  type ChangeStatus,
  type CheckItem,
  type LauncherTileId,
  type SectionTileMap,
  type StreamMessage,
  type TileId,
  type WorkspaceMode,
  type WorkspaceTile,
} from "./workspace/types";
import { BootSkeleton } from "./ui";
import { IconMoon, IconSun } from "./icons/Icons";
import { AgentExcerptProvider } from "./workspace/AgentExcerptContext";
import type { ContextExcerpt } from "./workspace/contextExcerpt";
import {
  conversationTurnsFromMessages,
  getAgentConversation,
  listAgentConversations,
  persistStreamMessages,
  streamMessageFromConversationRow,
  syncConversationFromChat,
} from "./agents/conversations";

function sessionRefreshToken(session: Session): string | undefined {
  return session.refreshToken || activeConnection(session)?.refreshToken || undefined;
}

async function persistHydrated(session: Session): Promise<void> {
  if (window.one) {
    await window.one.setSession(session);
  }
}

async function signOutHydrated(
  session: Session,
  setConnectPrefillUrl: (url: string | null) => void,
): Promise<null> {
  setConnectPrefillUrl(session.baseUrl);
  if (window.one) {
    await window.one.setSession(null);
  }
  return null;
}

async function rotateHydrated(session: Session, refreshToken: string): Promise<Session> {
  const rotated = await refreshAccessToken(session.baseUrl, refreshToken, {
    allowInsecureHttp: session.allowInsecureHttp,
    clientId: CONTROL_IDE_INTEGRATION,
  });
  const next = withRotatedTokens(session, rotated);
  await persistHydrated(next);
  return next;
}

async function fetchMe(session: Session): Promise<Record<string, unknown>> {
  return (await apiFetch(session.baseUrl, session.token, "/client/v1/me", {}, {
    allowInsecureHttp: session.allowInsecureHttp,
    apiRevisionPin: activeConnection(session)?.apiRevisionPin,
    deviceId: session.deviceId,
    skipRefresh: true,
  })) as Record<string, unknown>;
}

/** Restore a stored session: silent refresh when the access JWT is stale, then /me. */
async function hydrateConnectedSession(
  loaded: Session,
  setConnectPrefillUrl: (url: string | null) => void,
): Promise<Session | null> {
  let next = loaded;
  try {
    const rt = sessionRefreshToken(next);
    const expiresAt = next.accessExpiresAt ?? activeConnection(next)?.accessExpiresAt;
    if (rt && accessTokenNearingExpiry(expiresAt)) {
      next = await rotateHydrated(next, rt);
    }
    try {
      const actor = await fetchMe(next);
      next = withActorIdentity(next, actor);
      await persistHydrated(next);
      return next;
    } catch (err) {
      if (!isUnauthorizedError(err)) {
        return next;
      }
      const retryRt = sessionRefreshToken(next);
      if (!retryRt) {
        return signOutHydrated(next, setConnectPrefillUrl);
      }
      next = await rotateHydrated(next, retryRt);
      const actor = await fetchMe(next);
      next = withActorIdentity(next, actor);
      await persistHydrated(next);
      return next;
    }
  } catch {
    return signOutHydrated(next, setConnectPrefillUrl);
  }
}

export function App() {
  const operateToolDismissedRef = useRef(false);
  const settingsToolDismissedRef = useRef(false);
  const [theme, setTheme] = useState<Theme>(() => resolveInitialTheme());
  const [entered, setEntered] = useState(false);
  const [launcherOpen, setLauncherOpen] = useState(false);
  const [mode, setMode] = useState<WorkspaceMode>("operate");
  const [section, setSection] = useState<AppSection>("operate");
  /** Per-section workspace slices — switching launcher tiles restores the last selection. */
  const [tilesBySection, setTilesBySection] = useState<SectionTileMap>(() => ({
    ...EMPTY_SECTION_TILES,
  }));
  const tiles = tilesBySection[section];
  const [changeStatus, setChangeStatus] = useState<ChangeStatus>("draft");
  const [pipelineChecks, setPipelineChecks] = useState<CheckItem[]>([]);
  const [session, setSession] = useState<Session | null>(null);
  const sessionRef = useRef<Session | null>(null);
  const [ready, setReady] = useState(false);
  const [metadataFocus, setMetadataFocus] = useState<string | null>(null);
  const [automationFocus, setAutomationFocus] = useState<string | null>(null);
  const [boardHandoff, setBoardHandoff] = useState<BoardHandoff | null>(null);
  const [envEpoch, setEnvEpoch] = useState(0);
  const [queryFocusEpoch, setQueryFocusEpoch] = useState(0);
  const [playbookEpoch, setPlaybookEpoch] = useState(0);
  const [runTools, setRunTools] = useState<ToolSpecSummary[]>([]);
  const [activeRunToolApiName, setActiveRunToolApiName] = useState<string | null>(null);
  const [activeRunSessionToolId, setActiveRunSessionToolId] = useState<string | null>(null);
  const [activeRunObjectApiName, setActiveRunObjectApiName] = useState<string | null>(null);
  const [operateFocusRecord, setOperateFocusRecord] = useState<{ id: string; epoch: number } | null>(null);
  const [toolStoreEpoch, setToolStoreEpoch] = useState(0);
  const [runGraphEpoch, setRunGraphEpoch] = useState(0);
  const [runGraphMountRequest, setRunGraphMountRequest] = useState<{ toolId: string; epoch: number } | null>(null);
  const [connectPrefillUrl, setConnectPrefillUrl] = useState<string | null>(null);
  const [focusConnect, setFocusConnect] = useState(false);
  const [toolsRailPinned, setToolsRailPinned] = useState(false);
  const [agentsRailPinned, setAgentsRailPinned] = useState(false);
  const [railDismissNonce, setRailDismissNonce] = useState(0);
  const [agentCatalog, setAgentCatalog] = useState<AgentChat[]>(() => [
    { ...PRIMARY_OPERATE_CHAT, messages: [...PRIMARY_OPERATE_CHAT.messages] },
    ...SEED_AGENT_CHATS.map((c) => ({ ...c, messages: [...c.messages] })),
  ]);
  const agentCatalogRef = useRef(agentCatalog);
  agentCatalogRef.current = agentCatalog;
  const pendingToolRunsRef = useRef<Record<string, AgentRun>>({});
  const [busyChatIds, setBusyChatIds] = useState<string[]>([]);
  const [approveBusy, setApproveBusy] = useState(false);
  const [pendingExcerptsByChat, setPendingExcerptsByChat] = useState<Record<string, ContextExcerpt[]>>({});
  const [runGraphSelectionExcerpt, setRunGraphSelectionExcerpt] = useState<ContextExcerpt | null>(null);
  const proposalStore = useMemo(() => new ProposalStagingStore(), []);

  useEffect(() => {
    applyTheme(theme);
    persistTheme(theme);
  }, [theme]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key.toLowerCase() === "l") {
        e.preventDefault();
        setTheme((t) => toggleTheme(t));
      }
      if (e.key === "Escape" && launcherOpen) {
        setLauncherOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [launcherOpen]);

  useEffect(() => {
    void (async () => {
      if (window.one) {
        const raw = await window.one.getSession();
        let next = normalizeSession(raw);
        if (next && isConnected(next)) {
          next = await hydrateConnectedSession(next, setConnectPrefillUrl);
        }
        sessionRef.current = next;
        setSession(next);
      }
      setReady(true);
    })();
  }, []);

  const connected = isConnected(session);
  const allowedModes = useMemo(
    () =>
      modesForScopes(session?.scopes, {
        systemPermissions: session?.systemPermissions,
        isAdmin: session?.isAdmin,
      }),
    [session?.scopes, session?.systemPermissions, session?.isAdmin],
  );
  const allowAccount = canOpenSettings(session?.scopes, {
    systemPermissions: session?.systemPermissions,
    isAdmin: session?.isAdmin,
  });

  // Drop workspace chrome when the JWT is cleared so the auth screen is exclusive.
  useEffect(() => {
    if (!connected) {
      setEntered(false);
      setLauncherOpen(false);
      setTilesBySection({ ...EMPTY_SECTION_TILES });
      operateToolDismissedRef.current = false;
      settingsToolDismissedRef.current = false;
    }
  }, [connected]);

  const persistSession = useCallback(async (s: Session | null) => {
    const next = s ? normalizeSession(s) : null;
    sessionRef.current = next;
    if (window.one) {
      const result = await window.one.setSession(next);
      // New IPC returns { ok, error }; older stubs/tests may still return a boolean.
      if (result && typeof result === "object" && "ok" in result && !result.ok) {
        throw new Error(result.error ?? "Failed to persist session");
      }
      if (result === false) {
        throw new Error("Failed to persist session");
      }
    }
    setSession(next);
  }, []);

  const clearEnvScopedState = useCallback(() => {
    setEnvEpoch((n) => n + 1);
    setAgentCatalog((prev) => {
      const primary = prev.find((c) => c.primary);
      const keepSeeds = !sessionRef.current?.token;
      const seeds = keepSeeds
        ? SEED_AGENT_CHATS.map((c) => ({ ...c, messages: [...c.messages] }))
        : [];
      const resetPrimary = primary
        ? {
            ...PRIMARY_OPERATE_CHAT,
            messages: [...PRIMARY_OPERATE_CHAT.messages],
            sessionHandoff: null,
          }
        : null;
      return resetPrimary ? [resetPrimary, ...seeds] : seeds;
    });
    setBusyChatIds([]);
    setBoardHandoff(null);
    proposalStore.clear();
    setRunGraphSelectionExcerpt(null);
    setChangeStatus("draft");
    setMetadataFocus(null);
    setActiveRunObjectApiName(null);
    setTilesBySection({ ...EMPTY_SECTION_TILES });
    operateToolDismissedRef.current = false;
    settingsToolDismissedRef.current = false;
  }, [proposalStore]);

  /** Patch one section's tiles. Always pass the destination section when also changing section. */
  const patchSectionTiles = useCallback(
    (targetSection: AppSection, updater: (prev: WorkspaceTile[]) => WorkspaceTile[]) => {
      setTilesBySection((prev) => {
        const current = prev[targetSection] ?? [];
        return { ...prev, [targetSection]: normalizeWorkspaceOrder(updater(current)) };
      });
    },
    [],
  );

  const setTiles = useCallback(
    (next: WorkspaceTile[] | ((prev: WorkspaceTile[]) => WorkspaceTile[])) => {
      setTilesBySection((prev) => {
        const current = prev[section] ?? [];
        const resolved = typeof next === "function" ? next(current) : next;
        const ordered = normalizeWorkspaceOrder(resolved);
        if (!hasWorkspaceTool(ordered)) {
          if (section === "operate") operateToolDismissedRef.current = true;
          if (section === "settings") settingsToolDismissedRef.current = true;
        }
        return { ...prev, [section]: ordered };
      });
    },
    [section],
  );

  const seedingDefaultTool =
    (section === "operate" && !operateToolDismissedRef.current && tiles.length === 0) ||
    (section === "settings" && !settingsToolDismissedRef.current && tiles.length === 0);
  const { empty: workspaceEmpty, toolsDocked: toolsRailDocked, agentsDocked: agentsRailDocked } =
    workspaceRailDocking({
      connected,
      entered,
      tileCount: tiles.length,
      seedingDefaultTool,
      toolsPinned: toolsRailPinned,
      agentsPinned: agentsRailPinned,
    });
  const workspaceRailClass = workspaceRailClassNames(
    { empty: workspaceEmpty, toolsDocked: toolsRailDocked, agentsDocked: agentsRailDocked },
    section === "settings" ? "settings-workspace" : "",
  );

  const workspaceEmptyRef = useRef(workspaceEmpty);
  useEffect(() => {
    const wasEmpty = workspaceEmptyRef.current;
    workspaceEmptyRef.current = workspaceEmpty;
    if (wasEmpty && !workspaceEmpty) {
      setToolsRailPinned(false);
      setAgentsRailPinned(false);
      setRailDismissNonce((n) => n + 1);
    }
  }, [workspaceEmpty]);

  const switchEnv = useCallback(
    async (installId: string) => {
      if (!session) return;
      const next = switchActiveEnvironment(session, installId);
      if (next.activeInstallId === session.activeInstallId) return;
      await persistSession(next);
      clearEnvScopedState();
    },
    [session, persistSession, clearEnvScopedState],
  );

  const persistRotatedTokens = useCallback(
    async (tokens: { accessToken: string; refreshToken?: string; expiresIn?: number }) => {
      const current = sessionRef.current;
      if (!current) return;
      await persistSession(withRotatedTokens(current, tokens));
    },
    [persistSession],
  );

  const bridge = useMemo(
    () => ({
      session,
      setSession: persistSession,
      fetch: async (path: string, init?: RequestInit) => {
        const current = sessionRef.current;
        if (!current?.baseUrl || !current.token) throw new Error("Not connected");
        const refreshToken = current.refreshToken || activeConnection(current)?.refreshToken;
        return apiFetch(current.baseUrl, current.token, path, init, {
          deviceId: current.deviceId,
          allowInsecureHttp: current.allowInsecureHttp,
          apiRevisionPin: activeConnection(current)?.apiRevisionPin,
          refreshToken,
          clientId: CONTROL_IDE_INTEGRATION,
          onRotated: persistRotatedTokens,
        });
      },
      onOAuthCallback: window.one?.onOAuthCallback,
    }),
    [session, persistSession, persistRotatedTokens],
  );

  /** Load the safe runtime AgentSpec catalog for least-privileged Client users. */
  useEffect(() => {
    if (!session?.token) return;
    if (!connected || !session?.scopes?.some((s) => s.includes("client") || s === "admin")) {
      setAgentCatalog((prev) => {
        const primary = prev.find((c) => c.primary);
        const rest = prev.filter((c) => !c.primary && !c.demoStub);
        return primary ? [primary, ...rest] : rest;
      });
      return;
    }
    void (async () => {
      try {
        const playbooks = await listPlaybooks(bridge.fetch);
        setAgentCatalog((prev) => {
          const extras: AgentChat[] = playbooks
            .filter((p) => p.apiName && p.active !== false)
            .map((p) => {
              const existing = prev.find((c) => c.agentName === p.apiName);
              const modes = modesFromPrimarySection(p.primarySection, p.harnessId);
              if (existing) {
                return {
                  ...existing,
                  modes,
                  title: p.label || existing.title,
                  summary: p.goalTemplate || existing.summary,
                  demoStub: false,
                };
              }
              return {
                id: `playbook-${p.apiName}`,
                title: p.label || p.apiName || "Playbook",
                agentName: p.apiName!,
                summary: p.goalTemplate || p.instructions || "Customer AgentSpec from install.",
                modes,
                messages: [
                  {
                    id: `pb-${p.apiName}`,
                    role: "agent" as const,
                    body: `Live AgentSpec ${p.apiName}. Use agent to open a one-to-one chat.`,
                  },
                ],
              };
            });
          const byName = new Set(extras.map((e) => e.agentName));
          const primary = prev.find((c) => c.primary);
          // Connected session: never keep seed stubs as a live dock.
          const rest = prev.filter(
            (c) => !c.primary && !byName.has(c.agentName) && !c.demoStub,
          );
          return primary ? [primary, ...extras, ...rest] : [...extras, ...rest];
        });
      } catch {
        setAgentCatalog((prev) => {
          const primary = prev.find((c) => c.primary);
          const rest = prev.filter((c) => !c.primary && !c.demoStub);
          return primary ? [primary, ...rest] : rest;
        });
      }
    })();
  }, [connected, session?.scopes, session?.activeInstallId, bridge.fetch, envEpoch, playbookEpoch]);

  /** Hydrate open agent chats from persisted Client conversations. */
  useEffect(() => {
    if (!connected || !session?.scopes?.some((s) => s.includes("client") || s === "admin")) return;
    void (async () => {
      try {
        const conversations = await listAgentConversations(bridge.fetch);
        if (!conversations.length) return;
        const byPlaybook = new Map(
          conversations.filter((c) => c.playbookApiName).map((c) => [c.playbookApiName!, c]),
        );
        const fullByPlaybook = new Map<string, Awaited<ReturnType<typeof getAgentConversation>>>();
        await Promise.all(
          [...byPlaybook.entries()].map(async ([apiName, conv]) => {
            try {
              fullByPlaybook.set(apiName, await getAgentConversation(bridge.fetch, conv.id));
            } catch {
              /* skip */
            }
          }),
        );
        setAgentCatalog((prev) =>
          prev.map((chat) => {
            if (chat.primary || chat.conversationId || !chat.agentName) return chat;
            const full = fullByPlaybook.get(chat.agentName);
            if (!full) return chat;
            const messages =
              full.messages?.map((row) => streamMessageFromConversationRow(row)) ?? chat.messages;
            return {
              ...chat,
              conversationId: full.id,
              messages: messages.length ? messages : chat.messages,
            };
          }),
        );
      } catch {
        /* keep in-memory catalog */
      }
    })();
  }, [connected, session?.scopes, session?.activeInstallId, bridge.fetch, envEpoch]);

  /** Load granted ToolSpecs into the Run rail via Client tool_permissions (fail-closed). */
  useEffect(() => {
    if (!connected || !session?.scopes?.some((s) => s.includes("client") || s === "admin")) {
      setRunTools([]);
      return;
    }
    void (async () => {
      try {
        const tools = await listClientTools(bridge.fetch);
        setRunTools(tools);
      } catch {
        setRunTools([]);
      }
    })();
  }, [connected, session?.scopes, session?.activeInstallId, bridge, envEpoch]);

  const pinnedChatIds = useMemo(
    () =>
      tiles
        .filter((t) => t.kind === "chat" && t.chatId && t.chatId !== PRIMARY_OPERATE_CHAT_ID)
        .map((t) => t.chatId!),
    [tiles],
  );

  /** Open or focus a 1:1 chat tile for the clicked agent (tool-like swap — never merge threads). */
  const openOrFocusAgentChat = useCallback(
    (agentId: string) => {
      const agent = agentCatalog.find((c) => c.id === agentId && !c.primary);
      if (!agent) return;

      const targetSection: AppSection = agent.modes[0] ?? "operate";
      if (targetSection !== "settings") {
        setMode(targetSection);
      }
      setSection(targetSection);
      setEntered(true);
      setLauncherOpen(false);
      setRailDismissNonce((n) => n + 1);

      patchSectionTiles(targetSection, (prev) => selectAgentTiles(prev, makeChatTile(agent.id)));
    },
    [agentCatalog, patchSectionTiles],
  );

  const openAccount = useCallback(() => {
    settingsToolDismissedRef.current = false;
    setSection("settings");
    setEntered(true);
    setLauncherOpen(false);
  }, []);

  const enterMode = useCallback((next: WorkspaceMode) => {
    if (next === "operate") operateToolDismissedRef.current = false;
    setMode(next);
    setSection(next);
    setEntered(true);
    setLauncherOpen(false);
  }, []);

  /** Operate seeds personal graph once per visit when no tool is open. */
  useEffect(() => {
    if (!connected || !entered || section !== "operate" || hasWorkspaceTool(tilesBySection.operate)) return;
    if (operateToolDismissedRef.current) return;
    patchSectionTiles("operate", (prev) => selectToolTiles(prev, makePanelTile("runGraph")));
  }, [connected, entered, section, patchSectionTiles, tilesBySection.operate]);

  /** Settings seeds the first allowed tool when the board is empty. */
  useEffect(() => {
    if (!connected || !entered || section !== "settings" || hasWorkspaceTool(tilesBySection.settings)) return;
    if (settingsToolDismissedRef.current) return;
    const tools = toolsForSettings(session?.scopes, {
      systemPermissions: session?.systemPermissions,
      isAdmin: session?.isAdmin,
    });
    const defaultTool = tools[0] ?? "account";
    patchSectionTiles("settings", (prev) => selectToolTiles(prev, makePanelTile(defaultTool)));
  }, [
    connected,
    entered,
    section,
    patchSectionTiles,
    session?.scopes,
    session?.systemPermissions,
    session?.isAdmin,
    tilesBySection.settings,
  ]);

  const enterLauncherTile = useCallback(
    (next: LauncherTileId) => {
      if (next === "settings") {
        openAccount();
        return;
      }
      enterMode(next);
    },
    [enterMode, openAccount],
  );

  const switchMode = useCallback(
    (next: LauncherTileId) => {
      if (next === "settings") {
        openAccount();
        return;
      }
      if (next === "operate") operateToolDismissedRef.current = false;
      setMode(next);
      setSection(next);
      setLauncherOpen(false);
    },
    [openAccount],
  );

  const selectWorkspaceTool = useCallback(
    (panelId: string, opts?: { selectedRecordId?: string; forceBoard?: boolean }) => {
      setRailDismissNonce((n) => n + 1);
      const objectApiName = parseObjectHomeRailId(panelId);
      if (objectApiName) {
        setActiveRunObjectApiName(objectApiName);
        setActiveRunToolApiName(null);
        setActiveRunSessionToolId(null);
        if (opts?.selectedRecordId) {
          setOperateFocusRecord((prev) => ({
            id: opts.selectedRecordId!,
            epoch: (prev?.epoch ?? 0) + 1,
          }));
        } else {
          setOperateFocusRecord(null);
        }
        patchSectionTiles(section, (prev) => selectToolTiles(prev, makePanelTile("objectHome")));
        return;
      }
      const sessionId = parseSessionToolRailId(panelId);
      if (sessionId) {
        const graphOpen = section === "operate" && tilesBySection.operate.some(
          (tile) => tile.kind === "panel" && tile.panelId === "runGraph",
        );
        if (graphOpen && !opts?.forceBoard) {
          setRunGraphMountRequest((current) => ({
            toolId: panelId,
            epoch: (current?.epoch ?? 0) + 1,
          }));
          return;
        }
        setActiveRunSessionToolId(sessionId);
        setActiveRunToolApiName(null);
        setOperateFocusRecord(null);
        patchSectionTiles(section, (prev) => selectToolTiles(prev, makePanelTile("runTool")));
        return;
      }
      const toolApi = parseToolRailId(panelId);
      if (toolApi) {
        const graphOpen = section === "operate" && tilesBySection.operate.some(
          (tile) => tile.kind === "panel" && tile.panelId === "runGraph",
        );
        if (graphOpen && !opts?.forceBoard) {
          setRunGraphMountRequest((current) => ({
            toolId: panelId,
            epoch: (current?.epoch ?? 0) + 1,
          }));
          return;
        }
        setActiveRunToolApiName(toolApi);
        setActiveRunSessionToolId(null);
        setOperateFocusRecord(null);
        patchSectionTiles(section, (prev) => selectToolTiles(prev, makePanelTile("runTool")));
        return;
      }
      if (panelId === "runGraph") {
        setActiveRunToolApiName(null);
        setActiveRunSessionToolId(null);
        setActiveRunObjectApiName(null);
        setOperateFocusRecord(null);
      }
      if (panelId === "runGraph") {
        setRunGraphEpoch((n) => n + 1);
      }
      if (panelId === "objectHome") {
        setActiveRunObjectApiName(null);
        setOperateFocusRecord(null);
      }
      patchSectionTiles(section, (prev) => {
        if (panelId === "crm") {
          return selectToolTiles(prev, { id: "crm", kind: "crm", colSpan: 2, rowSpan: 1 });
        }
        return selectToolTiles(prev, makePanelTile(panelId as TileId));
      });
    },
    [section, patchSectionTiles, tilesBySection.operate],
  );

  useEffect(() => {
    if (!connected || !entered || section !== "operate") return;
    const onKey = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== "k") return;
      event.preventDefault();
      const graphOpen = tilesBySection.operate.some(
        (tile) => tile.kind === "panel" && tile.panelId === "runGraph",
      );
      if (!graphOpen) selectWorkspaceTool("runGraph");
      window.setTimeout(() => {
        globalThis.document.querySelector<HTMLInputElement>('[data-testid="operate-global-search"]')?.focus();
      }, graphOpen ? 0 : 80);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [connected, entered, section, selectWorkspaceTool, tilesBySection.operate]);

  const closeWorkspaceTool = useCallback(
    (panelId: string) => {
      if (section === "operate") operateToolDismissedRef.current = true;
      if (section === "settings") settingsToolDismissedRef.current = true;
      const sessionId = parseSessionToolRailId(panelId);
      const toolApi = parseToolRailId(panelId);
      const objectApiName = parseObjectHomeRailId(panelId);
      if (objectApiName || panelId === "objectHome") {
        setActiveRunObjectApiName(null);
        patchSectionTiles(section, (prev) =>
          prev.filter((t) => !(t.kind === "panel" && t.panelId === "objectHome")),
        );
        return;
      }
      if (sessionId || toolApi || panelId === "runTool") {
        setActiveRunSessionToolId(null);
        setActiveRunToolApiName(null);
        patchSectionTiles(section, (prev) =>
          prev.filter((t) => !(t.kind === "panel" && t.panelId === "runTool")),
        );
        return;
      }
      patchSectionTiles(section, (prev) =>
        prev.filter(
          (t) =>
            !(t.kind === "panel" && t.panelId === panelId) &&
            !(panelId === "crm" && t.kind === "crm"),
        ),
      );
    },
    [section, patchSectionTiles],
  );

  const closeWorkspaceAgent = useCallback(
    (agentId: string) => {
      patchSectionTiles(section, (prev) => prev.filter((t) => !(t.kind === "chat" && t.chatId === agentId)));
    },
    [section, patchSectionTiles],
  );

  const openTile = useCallback(
    (id: TileId) => {
      if (id === "env" || id === "connect") {
        if (id === "connect") setFocusConnect(true);
        const tools = toolsForSettings(session?.scopes, {
          systemPermissions: session?.systemPermissions,
          isAdmin: session?.isAdmin,
        });
        const panelId = tools.includes("env") ? "env" : (tools[0] ?? "account");
        setSection("settings");
        setEntered(true);
        patchSectionTiles("settings", (prev) => selectToolTiles(prev, makePanelTile(panelId)));
        setLauncherOpen(false);
        return;
      }
      if (id === "agents") {
        setEntered(true);
        setLauncherOpen(false);
        return;
      }
      if (id === "crm" || id === "client") {
        setMode("build");
        setSection("build");
        setEntered(true);
        patchSectionTiles("build", (prev) =>
          selectToolTiles(prev, { id: "crm", kind: "crm", colSpan: 2, rowSpan: 1 }),
        );
        setLauncherOpen(false);
        return;
      }
      const modeFor: Partial<Record<TileId, WorkspaceMode>> = {
        query: "build",
        monitor: "build",
        explorer: "build",
        runTool: "operate",
        objects: "build",
        packages: "build",
        agentSpecs: "build",
        tools: "build",
        automations: "build",
        metadata: "build",
        repo: "build",
        deploy: "build",
        users: "govern",
        integrations: "govern",
        experiences: "govern",
        installAuth: "govern",
        permissions: "govern",
        govern: "govern",
      };
      let panelId: TileId = id;
      if (id === "govern") {
        panelId = "users";
      }
      const nextSection: AppSection = id === "account" || id === "hosting" || id === "inference" ? "settings" : (modeFor[id] ?? section);
      if (nextSection !== "settings") {
        setMode(nextSection as WorkspaceMode);
      }
      setSection(nextSection);
      setEntered(true);
      patchSectionTiles(nextSection, (prev) => selectToolTiles(prev, makePanelTile(panelId)));
      setLauncherOpen(false);
    },
    [section, patchSectionTiles, session?.scopes, session?.systemPermissions, session?.isAdmin],
  );

  const goConnect = useCallback(
    (prefillUrl?: string) => {
      // Unauthenticated shell always shows AuthScreen — only stash a peer prefill.
      if (!isConnected(session)) {
        if (prefillUrl) setConnectPrefillUrl(prefillUrl);
        return;
      }
      if (prefillUrl) setConnectPrefillUrl(prefillUrl);
      setFocusConnect(true);
      setSection("settings");
      patchSectionTiles("settings", () => [makePanelTile("env")]);
      setEntered(true);
      setLauncherOpen(false);
    },
    [session, patchSectionTiles],
  );

  const openBoardFromHandoff = useCallback(
    (h: BoardHandoff) => {
      setMode("build");
      setSection("build");
      setEntered(true);
      patchSectionTiles("build", (prev) => selectAgentTiles(prev, makePrimaryChatTile()));
      setBoardHandoff(h);

      setAgentCatalog((prev) =>
        prev.map((c) => {
          if (c.id !== PRIMARY_OPERATE_CHAT_ID) return c;
          return {
            ...c,
            sessionHandoff: h,
            messages: [
              ...c.messages,
              {
                id: `h-apply-${Date.now()}`,
                role: "agent" as const,
                body: h.rationale ?? `Matching records for ${h.objectApiName ?? "query"}.`,
                createdAt: new Date().toISOString(),
                boardHandoff: h,
                runId: h.runId,
                agentLabel: c.agentName,
              },
            ],
          };
        }),
      );
      if (h.proposedMutations?.length) {
        setChangeStatus("needs_review");
      } else {
        setChangeStatus((s) => (s === "ready" || s === "promoted" ? s : "running"));
      }
      setLauncherOpen(false);
    },
    [patchSectionTiles],
  );

  const onStagedMutations = useCallback((count: number) => {
    setChangeStatus(count > 0 ? "needs_review" : "ready");
  }, []);

  const openAgentChat = useCallback(
    (chat: AgentChat) => {
      openOrFocusAgentChat(chat.id);
    },
    [openOrFocusAgentChat],
  );

  const openSessionTool = useCallback(
    (toolId: string) => {
      setMode("operate");
      setSection("operate");
      setEntered(true);
      setActiveRunSessionToolId(toolId);
      setActiveRunToolApiName(null);
      patchSectionTiles("operate", (prev) => selectToolTiles(prev, makePanelTile("runTool")));
      setToolStoreEpoch((n) => n + 1);
    },
    [patchSectionTiles],
  );

  const openRunGraph = useCallback(() => {
    operateToolDismissedRef.current = false;
    setMode("operate");
    setSection("operate");
    setEntered(true);
    patchSectionTiles("operate", (prev) => selectToolTiles(prev, makePanelTile("runGraph")));
    setRunGraphEpoch((n) => n + 1);
  }, [patchSectionTiles]);

  const pinHandoffToGraph = useCallback(
    async (handoff: BoardHandoff) => {
      await pinBoardHandoffToHomeGraph(bridge.fetch, handoff);
      openRunGraph();
    },
    [bridge.fetch, openRunGraph],
  );

  const stageHandoffProposal = useCallback(
    async (handoff: BoardHandoff) => {
      const mutations = handoff.proposedMutations ?? [];
      if (!mutations.length) throw new Error("Handoff has no proposed mutations");
      await stageProposalOnGraph(bridge.fetch, proposalStore, {
        proposalId: `proposal-${handoff.runId ?? globalThis.crypto.randomUUID()}`,
        runId: handoff.runId,
        mutations,
        rationale: handoff.rationale,
      });
      setChangeStatus("needs_review");
      openRunGraph();
    },
    [bridge.fetch, openRunGraph, proposalStore],
  );

  const refreshRunTools = useCallback(async () => {
    if (!connected || !session?.scopes?.some((s) => s.includes("client") || s === "admin")) {
      return;
    }
    try {
      const tools = await listClientTools(bridge.fetch);
      setRunTools(tools);
    } catch {
      /* keep prior list */
    }
  }, [bridge.fetch, connected, session?.scopes]);

  const finalizeAgentRun = useCallback(
    async (
      final: AgentRun,
      chat: AgentChat | undefined,
      workspaceSection: AppSection,
      opts: { applyEffects?: boolean } = {},
    ) => {
      let run = final;
      const runEffects =
        workspaceSection === "operate" ||
        ((chat?.modes as string[]).includes("run") && !chat?.modes.includes("operate"));
      const activeCtx = buildActiveToolContext(activeRunSessionToolId, toolStoreEpoch);
      const pendingActions =
        runEffects && final.status !== "awaiting_approval"
          ? pendingToolActionsFromRun(final, {
              activeToolId: activeCtx?.toolId,
              activeToolBindings: activeCtx?.dataBindings,
            })
          : [];
      const applyEffects = opts.applyEffects ?? pendingActions.length === 0;
      if (runEffects && final.status !== "awaiting_approval" && applyEffects) {
        const effects = await processRunToolEffects(final, {
          fetch: bridge.fetch,
          mode: "operate",
          activeToolId: activeCtx?.toolId,
          activeToolBindings: activeCtx?.dataBindings,
          proposalStore,
        });
        if (effects.enrichedOutput) {
          run = { ...final, output: effects.enrichedOutput };
        }
        if (effects.toolId) {
          openSessionTool(effects.toolId);
        } else if (effects.graphChanged) {
          openRunGraph();
        } else if (effects.store) {
          setToolStoreEpoch((n) => n + 1);
        }
        if (effects.toolSpecApiName) {
          void refreshRunTools();
        }
        if (effects.proposalId) {
          setChangeStatus("needs_review");
        }
      }
      const messageMode: AppSection = runEffects ? "operate" : workspaceSection;
      return messagesFromRun(run, {
        mode: messageMode,
        agentLabel: chat?.agentName,
        pendingToolActions: applyEffects ? undefined : pendingActions,
      });
    },
    [
      activeRunSessionToolId,
      bridge.fetch,
      openRunGraph,
      openSessionTool,
      proposalStore,
      refreshRunTools,
      toolStoreEpoch,
    ],
  );

  const openChatId = useMemo(
    () => tiles.find((t) => t.kind === "chat" && t.chatId)?.chatId ?? null,
    [tiles],
  );

  const excerptBridge = useMemo(
    () => ({
      addExcerptToOpenChat: (excerpt: ContextExcerpt) => {
        if (!openChatId) return;
        setPendingExcerptsByChat((prev) => ({
          ...prev,
          [openChatId]: [...(prev[openChatId] ?? []), excerpt],
        }));
      },
    }),
    [openChatId],
  );

  const sendInAgentChat = useCallback(
    (chatId: string, text: string, excerpts?: ContextExcerpt[]) => {
      const chat = agentCatalogRef.current.find((c) => c.id === chatId);
      if (!chat || !isAgentBound(chat)) {
        setAgentCatalog((prev) =>
          prev.map((c) =>
            c.id === chatId
              ? {
                  ...c,
                  messages: [
                    ...c.messages,
                    {
                      id: `sys-unbound-${Date.now()}`,
                      role: "system" as const,
                      body: "Choose an agent from the dock before sending.",
                      createdAt: new Date().toISOString(),
                    },
                  ],
                }
              : c,
          ),
        );
        return;
      }
      const effectiveExcerpts = [...(excerpts ?? [])];
      const graphExcerpt = runGraphSelectionExcerpt;
      if (
        section === "operate" &&
        graphExcerpt &&
        !effectiveExcerpts.some((excerpt) => excerpt.id === graphExcerpt.id)
      ) {
        effectiveExcerpts.push(graphExcerpt);
      }
      const createdAt = new Date().toISOString();
      const human = {
        id: `h-${Date.now()}`,
        role: "human" as const,
        body: text,
        createdAt,
        contextExcerpts: effectiveExcerpts.length ? effectiveExcerpts : undefined,
      };

      if (!session?.token) {
        setAgentCatalog((prev) =>
          prev.map((c) => {
            if (c.id !== chatId) return c;
            const reply = {
              id: `a-${Date.now()}`,
              role: "agent" as const,
              body: `Stub · ${c.agentName} received your note. Connect to stream live /agents/runs.`,
              createdAt,
              agentLabel: c.agentName,
            };
            return { ...c, messages: [...c.messages, human, reply] };
          }),
        );
        return;
      }

      const streamMessageId = `stream-${chatId}-${Date.now()}`;
      const placeholder = runningAgentPlaceholder({
        id: streamMessageId,
        agentLabel: chat.agentName,
        createdAt,
      });
      setAgentCatalog((prev) =>
        prev.map((c) => (c.id === chatId ? { ...c, messages: [...c.messages, human, placeholder] } : c)),
      );
      setBusyChatIds((prev) => (prev.includes(chatId) ? prev : [...prev, chatId]));

      void (async () => {
        let conversationId = chat.conversationId;
        try {
          if (!conversationId) {
            conversationId = await syncConversationFromChat(bridge.fetch, chat, section);
            setAgentCatalog((prev) =>
              prev.map((c) => (c.id === chatId ? { ...c, conversationId } : c)),
            );
          }
          if (conversationId) {
            await persistStreamMessages(bridge.fetch, conversationId, [human]).catch(() => {
              /* non-fatal — in-memory transcript still goes on the run input */
            });
          }
          const activeCtx =
            section === "operate" ? buildActiveToolContext(activeRunSessionToolId, toolStoreEpoch) : null;
          const runInput: Record<string, unknown> = {
            chatId,
            conversationId,
          };
          const conversation = conversationTurnsFromMessages(chat.messages);
          if (conversation.length) runInput.conversation = conversation;
          if (effectiveExcerpts.length) runInput.contextExcerpts = effectiveExcerpts;
          if (activeCtx) runInput.activeTool = activeCtx;

          const updateStreamMessage = (patch: Partial<StreamMessage>) => {
            setAgentCatalog((prev) =>
              prev.map((c) => {
                if (c.id !== chatId) return c;
                const index = c.messages.findIndex((m) => m.id === streamMessageId);
                if (index < 0) {
                  return {
                    ...c,
                    messages: [...c.messages, { ...placeholder, ...patch }],
                  };
                }
                const messages = [...c.messages];
                messages[index] = { ...messages[index], ...patch };
                return { ...c, messages };
              }),
            );
          };
          let streamedText = "";
          let tokenFrame = 0;
          const flushTokens = () => {
            tokenFrame = 0;
            updateStreamMessage({ body: streamedText, runStatus: "running" });
          };
          const final = await createAgentRunStream(
            session.baseUrl,
            session.token,
            {
              goal: text,
              playbookApiName: chat?.agentName,
              input: runInput,
              conversationId,
              // Never rely on a server default for tool approval (CIDE-18).
              dryRun: false,
              approved: false,
            },
            {
              onRun: ({ id, status }) =>
                updateStreamMessage({ runId: id, runStatus: status ?? "running" }),
              onToken: ({ delta }) => {
                if (!delta) return;
                streamedText += delta;
                if (!tokenFrame) {
                  tokenFrame = requestAnimationFrame(flushTokens);
                }
              },
              onDone: ({ id, status }) => {
                if (tokenFrame) {
                  cancelAnimationFrame(tokenFrame);
                  tokenFrame = 0;
                }
                updateStreamMessage({
                  runId: id,
                  body: streamedText,
                  runStatus: status ?? "completed",
                });
              },
            },
          );
          if (final.status === "awaiting_approval") {
            throw new Error(STREAM_PARKED_HINT);
          }
          const replies = await finalizeAgentRun(final, chat, section);
          if (final.id && replies.some((m) => m.pendingToolApply)) {
            pendingToolRunsRef.current[final.id] = final;
          }
          if (conversationId) {
            void persistStreamMessages(bridge.fetch, conversationId, replies).catch(() => {
              /* non-fatal */
            });
          }
          setAgentCatalog((prev) =>
            prev.map((c) => {
              if (c.id !== chatId) return c;
              const current = c.messages.find((m) => m.id === streamMessageId) ?? placeholder;
              const merged = mergeStreamedRunReplies(current, replies);
              return {
                ...c,
                messages: [...c.messages.filter((m) => m.id !== streamMessageId), ...merged],
              };
            }),
          );
        } catch (e) {
          setAgentCatalog((prev) =>
            prev.map((c) =>
              c.id === chatId
                ? {
                    ...c,
                    messages: [
                      ...c.messages.filter((m) => m.id !== streamMessageId),
                      {
                        id: `a-${Date.now()}`,
                        role: "system" as const,
                        body: `Run failed: ${String(e)}`,
                        createdAt: new Date().toISOString(),
                      },
                    ],
                  }
                : c,
            ),
          );
        } finally {
          setBusyChatIds((prev) => prev.filter((id) => id !== chatId));
        }
      })();
    },
    [
      activeRunSessionToolId,
      bridge.fetch,
      connected,
      finalizeAgentRun,
      section,
      runGraphSelectionExcerpt,
      session,
      toolStoreEpoch,
    ],
  );

  const askPrimaryAgent = useCallback(
    (prompt: string) => {
      const openChat = tiles.find((t) => t.kind === "chat" && t.chatId);
      const openOperate =
        openChat?.chatId &&
        agentCatalog.find(
          (c) => c.id === openChat.chatId && c.modes.includes("operate") && isAgentBound(c),
        );
      if (openOperate) {
        sendInAgentChat(openOperate.id, prompt);
        return;
      }
      const fallback =
        agentCatalog.find((c) => !c.primary && c.modes.includes("operate") && c.id.startsWith("playbook-")) ??
        agentCatalog.find((c) => !c.primary && c.modes.includes("operate"));
      if (fallback) {
        openOrFocusAgentChat(fallback.id);
        window.setTimeout(() => sendInAgentChat(fallback.id, prompt), 0);
      }
    },
    [agentCatalog, openOrFocusAgentChat, sendInAgentChat, tiles],
  );

  const askRunAgent = useCallback(
    (prompt: string) => {
      const openChat = tiles.find((t) => t.kind === "chat" && t.chatId);
      const openRunChat =
        openChat?.chatId &&
        agentCatalog.find((c) => c.id === openChat.chatId && c.modes.includes("operate") && isAgentBound(c));
      if (openRunChat) {
        sendInAgentChat(openRunChat.id, prompt);
        return;
      }
      const fallback =
        agentCatalog.find((c) => !c.primary && c.modes.includes("operate") && c.id.startsWith("playbook-")) ??
        agentCatalog.find((c) => !c.primary && c.modes.includes("operate"));
      if (fallback) {
        openOrFocusAgentChat(fallback.id);
        window.setTimeout(() => sendInAgentChat(fallback.id, prompt), 0);
      }
    },
    [agentCatalog, openOrFocusAgentChat, sendInAgentChat, tiles],
  );

  const approveInAgentChat = useCallback(
    (runId: string, chatId: string) => {
      if (!connected || !session) return;
      const chat = agentCatalogRef.current.find((c) => c.id === chatId);
      const pendingRun = pendingToolRunsRef.current[runId];
      const pendingLocal = Boolean(
        pendingRun && chat?.messages.some((m) => m.runId === runId && m.pendingToolApply),
      );

      setApproveBusy(true);
      setBusyChatIds((prev) => (prev.includes(chatId) ? prev : [...prev, chatId]));

      if (pendingLocal && pendingRun) {
        void (async () => {
          try {
            const replies = await finalizeAgentRun(pendingRun, chat, section, { applyEffects: true });
            delete pendingToolRunsRef.current[runId];
            setAgentCatalog((prev) =>
              prev.map((c) => {
                if (c.id !== chatId) return c;
                const current =
                  c.messages.find((m) => m.runId === runId && (m.role === "approval" || m.pendingToolApply)) ??
                  c.messages.find((m) => m.runId === runId);
                const merged = current
                  ? mergeStreamedRunReplies(
                      {
                        ...current,
                        role: "agent",
                        pendingToolApply: false,
                        runStatus: "completed",
                      },
                      replies,
                    )
                  : replies;
                return {
                  ...c,
                  messages: [...c.messages.filter((m) => m.runId !== runId), ...merged],
                };
              }),
            );
          } catch (e) {
            setAgentCatalog((prev) =>
              prev.map((c) =>
                c.id === chatId
                  ? {
                      ...c,
                      messages: [
                        ...c.messages,
                        {
                          id: `a-${Date.now()}`,
                          role: "system" as const,
                          body: `Approve failed: ${String(e)}`,
                          createdAt: new Date().toISOString(),
                        },
                      ],
                    }
                  : c,
              ),
            );
          } finally {
            setApproveBusy(false);
            setBusyChatIds((prev) => prev.filter((id) => id !== chatId));
          }
        })();
        return;
      }

      const streamMessageId = `a-${runId}-approve`;
      setApproveBusy(true);
      setBusyChatIds((prev) => (prev.includes(chatId) ? prev : [...prev, chatId]));
      setAgentCatalog((prev) =>
        prev.map((c) => {
          if (c.id !== chatId) return c;
          return {
            ...c,
            messages: c.messages.map((msg) =>
              msg.runId === runId && msg.role === "approval"
                ? {
                    ...msg,
                    id: streamMessageId,
                    role: "agent" as const,
                    runStatus: "running",
                    body: "",
                  }
                : msg,
            ),
          };
        }),
      );
      void (async () => {
        try {
          const updateStreamMessage = (patch: Partial<StreamMessage>) => {
            setAgentCatalog((prev) =>
              prev.map((c) => {
                if (c.id !== chatId) return c;
                const index = c.messages.findIndex((m) => m.id === streamMessageId);
                if (index < 0) return c;
                const messages = [...c.messages];
                messages[index] = { ...messages[index], ...patch };
                return { ...c, messages };
              }),
            );
          };
          let streamedText = "";
          let final = await approveAgentRunStream(session.baseUrl, session.token, runId, {
            onRun: ({ id, status }) =>
              updateStreamMessage({ runId: id ?? runId, runStatus: status ?? "running" }),
            onToken: ({ delta }) => {
              if (!delta) return;
              streamedText += delta;
              updateStreamMessage({ body: streamedText, runStatus: "running" });
            },
            onDone: ({ id, status }) =>
              updateStreamMessage({ runId: id ?? runId, runStatus: status ?? "completed" }),
          });
          if (!isTerminalRunStatus(final.status) && final.status !== "awaiting_approval") {
            final = await pollAgentRun(bridge.fetch, runId, { intervalMs: 600, maxAttempts: 30 });
          }
          const replies = await finalizeAgentRun(final, chat, section);
          setAgentCatalog((prev) =>
            prev.map((c) => {
              if (c.id !== chatId) return c;
              const current = c.messages.find((m) => m.id === streamMessageId);
              const merged = current ? mergeStreamedRunReplies(current, replies) : replies;
              return {
                ...c,
                messages: [...c.messages.filter((m) => m.id !== streamMessageId), ...merged],
              };
            }),
          );
        } catch (e) {
          setAgentCatalog((prev) =>
            prev.map((c) =>
              c.id === chatId
                ? {
                    ...c,
                    messages: [
                      ...c.messages.filter((m) => m.id !== streamMessageId),
                      {
                        id: `a-${Date.now()}`,
                        role: "system" as const,
                        body: `Approve failed: ${String(e)}`,
                        createdAt: new Date().toISOString(),
                      },
                    ],
                  }
                : c,
            ),
          );
        } finally {
          setApproveBusy(false);
          setBusyChatIds((prev) => prev.filter((id) => id !== chatId));
        }
      })();
    },
    [bridge.fetch, connected, finalizeAgentRun, section, session],
  );

  const onPipelineChange = useCallback((nextChecks: CheckItem[], status: ChangeStatus) => {
    setPipelineChecks(nextChecks);
    setChangeStatus(status);
  }, []);

  const renderPanel = (panelId: TileId): ReactNode => {
    switch (panelId) {
      case "connect":
        // Legacy tile id → Environments with connect focus.
        return (
          <EnvPanel
            bridge={bridge}
            key={`env-connect-${connectPrefillUrl ?? "default"}`}
            prefillBaseUrl={connectPrefillUrl ?? undefined}
            onPrefillConsumed={() => setConnectPrefillUrl(null)}
            focusConnect
            onFocusConnectConsumed={() => setFocusConnect(false)}
            onSwitchEnv={(id) => void switchEnv(id)}
            onConnectPeer={(url) => goConnect(url)}
          />
        );
      case "env":
        return (
          <EnvPanel
            bridge={bridge}
            key={`env-${connectPrefillUrl ?? "default"}`}
            prefillBaseUrl={connectPrefillUrl ?? undefined}
            onPrefillConsumed={() => setConnectPrefillUrl(null)}
            focusConnect={focusConnect}
            onFocusConnectConsumed={() => setFocusConnect(false)}
            onSwitchEnv={(id) => void switchEnv(id)}
            onConnectPeer={(url) => goConnect(url)}
          />
        );
      case "users":
        return <UsersPanel bridge={bridge} />;
      case "integrations":
        return <IntegrationsPanel bridge={bridge} />;
      case "experiences":
        return <ExperiencesPanel bridge={bridge} />;
      case "installAuth":
        return <InstallAuthPanel bridge={bridge} />;
      case "permissions":
        return <PermissionsPanel bridge={bridge} />;
      case "govern":
        return <UsersPanel bridge={bridge} />;
      case "repo":
        return <RepoPanel bridge={bridge} />;
      case "objects":
        return <ObjectManagerPanel bridge={bridge} refreshKey={envEpoch} />;
      case "packages":
        return <PackagesPanel bridge={bridge} refreshKey={envEpoch} />;
      case "agentSpecs":
        return (
          <AgentsPanel
            bridge={bridge}
            refreshKey={envEpoch}
            onCatalogChanged={() => setPlaybookEpoch((n) => n + 1)}
          />
        );
      case "tools":
        return (
          <ToolsPanel
            bridge={bridge}
            refreshKey={envEpoch}
            onToolsChanged={() => void refreshRunTools()}
            onStarterPackInstalled={() => setPlaybookEpoch((n) => n + 1)}
          />
        );
      case "automations":
        // Legacy deep-link — redirect UX lives in Metadata; keep panel for old tests/links.
        return (
          <AutomationsPanel
            bridge={bridge}
            refreshKey={envEpoch}
            focusPath={automationFocus}
            onFocusConsumed={() => setAutomationFocus(null)}
          />
        );
      case "metadata":
        return (
          <MetadataPanel
            bridge={bridge}
            focusPath={metadataFocus}
            onFocusConsumed={() => setMetadataFocus(null)}
          />
        );
      case "deploy":
        return (
          <DeployPanel
            key={`deploy-${envEpoch}`}
            bridge={bridge}
            onPipelineChange={onPipelineChange}
            onMoreChanges={() => enterMode("build")}
          />
        );
      case "client":
        return <ClientPanel bridge={bridge} />;
      case "query":
        return (
          <QueryPanel
            bridge={bridge}
            refreshKey={envEpoch + queryFocusEpoch}
            onAskAgent={askPrimaryAgent}
          />
        );
      case "monitor":
        return <MonitorPanel bridge={bridge} refreshKey={envEpoch} />;
      case "explorer":
        return (
          <ExplorerPanel
            bridge={bridge}
            refreshKey={envEpoch}
            onOpenInQuery={(objectApiName) => {
              try {
                sessionStorage.setItem("one.operate.queryObject", objectApiName);
              } catch {
                /* ignore */
              }
              selectWorkspaceTool("query");
              setQueryFocusEpoch((n) => n + 1);
            }}
            onAskAgent={askPrimaryAgent}
          />
        );
      case "runGraph":
        return (
          <RunGraphHome
            fetchFn={bridge.fetch}
            bridge={bridge}
            refreshKey={envEpoch + runGraphEpoch}
            mountableTools={[
              ...listSessionTools().map((tool) => ({
                id: sessionToolRailId(tool.id),
                kind: "working" as const,
                label: tool.title,
                workingToolId: tool.id,
              })),
              ...runTools.map((tool) => ({
                id: toolRailId(tool.apiName),
                kind: "published" as const,
                label: tool.label || tool.apiName,
                toolSpecApiName: tool.apiName,
              })),
            ]}
            onOpenObjectHome={() => selectWorkspaceTool("objectHome")}
            onAskRunAgent={askRunAgent}
            proposalStore={proposalStore}
            onProposalResolved={(status, pendingCount) => {
              setChangeStatus(
                pendingCount > 0 ? "needs_review" : status === "applied" ? "applied" : "ready",
              );
            }}
            onSelectionContextChange={setRunGraphSelectionExcerpt}
            mountRequest={runGraphMountRequest}
            onOpenTool={(node) => {
              if (node.toolRef?.toolSpecApiName) selectWorkspaceTool(toolRailId(node.toolRef.toolSpecApiName), { forceBoard: true });
              else if (node.toolRef?.workingToolId) selectWorkspaceTool(sessionToolRailId(node.toolRef.workingToolId), { forceBoard: true });
            }}
            onPublishedTool={() => { void refreshRunTools(); }}
          />
        );
      case "objectHome":
        return (
          <RunObjectHomePanel
            bridge={bridge}
            refreshKey={envEpoch}
            initialObjectApiName={activeRunObjectApiName}
            initialSelectedId={operateFocusRecord?.id}
            selectedIdEpoch={operateFocusRecord?.epoch ?? 0}
            onPinnedToGraph={openRunGraph}
          />
        );
      case "runTool": {
        const meta = runTools.find((t) => t.apiName === activeRunToolApiName);
        return (
          <RunToolPanel
            apiName={activeRunToolApiName || meta?.apiName || "Tool"}
            label={meta?.label || activeRunToolApiName || "Tool"}
            description={meta?.description}
            fetchFn={connected ? bridge.fetch : undefined}
            sessionToolId={activeRunSessionToolId}
            storeEpoch={toolStoreEpoch}
            onAskAgent={askRunAgent}
          />
        );
      }
      case "account":
        return <AccountSettingsPanel bridge={bridge} />;
      case "hosting":
        return <HostingPanel bridge={bridge} />;
      case "inference":
        return <InferencePanel bridge={bridge} />;
      default:
        return null;
    }
  };

  if (!ready) {
    return <BootSkeleton />;
  }

  return (
    <ThemeContext.Provider value={theme}>
      <AgentExcerptProvider bridge={excerptBridge}>
      <div className="app shell" data-testid="customer-ide-shell">
        <a className="skip-to-canvas" href="#workspace-main">
          Skip to workspace
        </a>
        <header className="top-bar">
          <p className="brand">
            Majesta One <span>Control</span>
          </p>
          <div className="top-bar-center">
            {!connected ? (
              <p className="top-mode muted" data-testid="active-mode">
                Sign in
              </p>
            ) : entered ? (
              <ModeTitle
                section={section}
                launcherOpen={launcherOpen}
                onToggleLauncher={() => setLauncherOpen((o) => !o)}
              />
            ) : (
              <p className="top-mode muted" data-testid="active-mode">
                Choose a mode
              </p>
            )}
          </div>
          <div className="top-actions">
            {connected ? (
              <EnvSwitcher
                session={session}
                onSwitch={(id) => void switchEnv(id)}
                onAddEnvironment={() => goConnect()}
              />
            ) : null}
            <button
              type="button"
              className="secondary theme-toggle icon-btn"
              data-testid="theme-toggle"
              onClick={() => setTheme((t) => toggleTheme(t))}
              aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
              title="Toggle theme (Ctrl/Cmd+Shift+L)"
            >
              {theme === "dark" ? <IconSun size={15} /> : <IconMoon size={15} />}
            </button>
            {connected ? (
              <SessionChip session={session} onOpenAccount={() => goConnect()} />
            ) : (
              <span className="session-chip" data-testid="session-chip">
                Not connected
              </span>
            )}
          </div>
        </header>

        {!connected ? (
          <AuthScreen
            bridge={bridge}
            prefillBaseUrl={connectPrefillUrl ?? undefined}
            onPrefillConsumed={() => setConnectPrefillUrl(null)}
          />
        ) : !entered ? (
          <ModeLauncher
            onSelect={enterLauncherTile}
            allowedModes={allowedModes}
            allowAccount={allowAccount}
          />
        ) : (
          <div
            className={workspaceRailClass}
            data-testid={section === "settings" ? "settings-workspace" : "workspace-chrome"}
            data-workspace-empty={workspaceEmpty ? "true" : "false"}
            data-tools-docked={toolsRailDocked ? "true" : "false"}
            data-agents-docked={agentsRailDocked ? "true" : "false"}
          >
            <WorkspaceToolRail
              tools={(() => {
                void toolStoreEpoch;
                if (section === "settings") {
                  return toolsForSettings(session?.scopes, {
                    systemPermissions: session?.systemPermissions,
                    isAdmin: session?.isAdmin,
                  }).map((id) => ({
                    id,
                    label: TILE_META[id].label,
                    summary: TILE_META[id].summary,
                  }));
                }
                const staticIds = toolsForMode(mode, session?.scopes, {
                  systemPermissions: session?.systemPermissions,
                  isAdmin: session?.isAdmin,
                });
                if (mode === "operate") {
                  const sessionEntries = listSessionTools().map((t) => ({
                    id: sessionToolRailId(t.id),
                    label: t.title,
                    summary: "Session working Tool for this install.",
                  }));
                  const staticEntries = staticIds
                    .filter((id) => id === "runGraph" || id === "objectHome")
                    .map((id) => ({
                      id,
                      label: TILE_META[id].label,
                      summary: TILE_META[id].summary,
                    }));
                  const metaEntries = runTools.map((t) => ({
                    id: toolRailId(t.apiName),
                    label: t.label || t.apiName,
                    summary: t.description || "Declarative ToolSpec for Run mode.",
                    name: t.apiName,
                  }));
                  return [...sessionEntries, ...staticEntries, ...metaEntries];
                }
                return staticIds.map((id) => ({
                  id,
                  label: TILE_META[id].label,
                  summary: TILE_META[id].summary,
                }));
              })()}
              openToolIds={tiles.flatMap((t) => {
                if (t.kind === "panel" && t.panelId === "runTool" && activeRunSessionToolId) {
                  return [sessionToolRailId(activeRunSessionToolId)];
                }
                if (t.kind === "panel" && t.panelId === "runTool" && activeRunToolApiName) {
                  return [toolRailId(activeRunToolApiName)];
                }
                if (t.kind === "panel" && t.panelId === "objectHome") {
                  return ["objectHome"];
                }
                if (t.kind === "panel" && t.panelId) return [t.panelId];
                if (t.kind === "crm") return ["crm"];
                return [];
              })}
              // Tools swap when one is open; only block add when board is full with no tool to replace.
              atCap={tiles.length >= MAX_WORKSPACE_TILES && !hasWorkspaceTool(tiles)}
              onSelectTool={selectWorkspaceTool}
              onCloseTool={closeWorkspaceTool}
              pinned={toolsRailPinned}
              forceExpanded={workspaceEmpty}
              dismissNonce={railDismissNonce}
              onPinnedChange={setToolsRailPinned}
              dragPayloadForTool={mode === "operate" ? (tool) => {
                const toolSpecApiName = parseToolRailId(tool.id);
                const workingToolId = parseSessionToolRailId(tool.id);
                if (!toolSpecApiName && !workingToolId) return null;
                return {
                  type: "operate-tool" as const,
                  railId: tool.id,
                  label: tool.label,
                  ...(toolSpecApiName ? { toolSpecApiName } : { workingToolId: workingToolId! }),
                };
              } : undefined}
            />
            <main className="workspace-main panel-fade" id="workspace-main" data-testid="workspace-main">
              <WorkspaceCanvas
                mode={section}
                tiles={tiles}
                catalog={agentCatalog}
                onTilesChange={setTiles}
                onOpenChat={openAgentChat}
                onSendInChat={sendInAgentChat}
                pendingExcerptsByChat={pendingExcerptsByChat}
                onPendingExcerptsChange={(chatId, excerpts) =>
                  setPendingExcerptsByChat((prev) => ({ ...prev, [chatId]: excerpts }))
                }
                onOpenBoard={openBoardFromHandoff}
                onOpenTile={openTile}
                onOpenSessionTool={openSessionTool}
                onApproveRun={approveInAgentChat}
                onAttachAgent={openOrFocusAgentChat}
                onSelectTool={selectWorkspaceTool}
                busyChatIds={busyChatIds}
                approveBusy={approveBusy}
                renderPanel={renderPanel}
                bridge={bridge}
                scopes={session?.scopes}
                systemPermissions={session?.systemPermissions}
                isAdmin={session?.isAdmin}
                handoff={boardHandoff}
                onHandoffConsumed={() => setBoardHandoff(null)}
                onStagedMutations={onStagedMutations}
                onStageProposal={stageHandoffProposal}
                onPinToGraph={pinHandoffToGraph}
                onAskAgent={askPrimaryAgent}
                onOpenInQuery={(objectApiName) => {
                  try {
                    sessionStorage.setItem("one.operate.queryObject", objectApiName);
                  } catch {
                    /* ignore */
                  }
                  selectWorkspaceTool("query");
                  setQueryFocusEpoch((n) => n + 1);
                }}
              />
            </main>
            <AgentStreamDock
              changeTitle={section === "settings" ? "Account assistants" : "Customer IDE working session"}
              changeStatus={changeStatus}
              mode={section}
              agentChats={agentCatalog}
              pinnedChatIds={pinnedChatIds}
              onOpenTile={openTile}
              onOpenBoard={openBoardFromHandoff}
              onAttachAgent={openOrFocusAgentChat}
              onCloseAgent={closeWorkspaceAgent}
              onSendToPrimary={(text) => sendInAgentChat(PRIMARY_OPERATE_CHAT_ID, text)}
              catalogOnly
              connected={connected}
              onGoConnect={() => goConnect()}
              bridge={bridge}
              pinned={agentsRailPinned}
              forceExpanded={workspaceEmpty}
              dismissNonce={railDismissNonce}
              onPinnedChange={setAgentsRailPinned}
            />
          </div>
        )}
        {connected && entered && launcherOpen ? (
          <ModeLauncher
            overlay
            onSelect={switchMode}
            onDismiss={() => setLauncherOpen(false)}
            allowedModes={allowedModes}
            allowAccount={allowAccount}
          />
        ) : null}
        {connected && entered ? (
          <ChangeStatusBar
            status={changeStatus}
            checks={pipelineChecks}
            showChecks={pipelineChecks.some((c) => c.state !== "pending")}
          />
        ) : (
          <ChangeStatusBar status="draft" checks={[]} showChecks={false} />
        )}
      </div>
      </AgentExcerptProvider>
    </ThemeContext.Provider>
  );
}

export type AppBridge = {
  session: Session | null;
  setSession: (s: Session | null) => Promise<void>;
  fetch: (path: string, init?: RequestInit) => Promise<unknown>;
  /** Electron deep-link oauth callback (`one-control://…`). */
  onOAuthCallback?: (handler: (url: string) => void) => () => void;
};
