import {
  Fragment,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import type { AppBridge } from "../App";
import { AgentChatPane } from "./AgentChatPane";
import { CrmPanel } from "../panels/CrmPanel";
import type { BoardHandoff } from "../operate/types";
import {
  MAX_WORKSPACE_TILES,
  PRIMARY_OPERATE_CHAT_ID,
  TILE_META,
  defaultChatLayout,
  isChatTile,
  isToolTile,
  primaryChatLayout,
  type AgentChat,
  type AppSection,
  type TileId,
  type WorkspaceTile,
} from "./types";
import { IconDropTarget } from "../icons/Icons";
import { EmptyState } from "../ui";

const MIN_RATIO = 0.15;
const SPLIT_HANDLE_PX = 10;

/** Tool 2/3 · agent 1/3 when both slices are open. */
export const TOOL_AGENT_SPLIT: readonly [number, number] = [2 / 3, 1 / 3];

/** @deprecated Prefer MAX_WORKSPACE_TILES; chats share the 2-slice cap (max 1 agent). */
export function maxChatTilesForMode(_mode: AppSection): number {
  return MAX_WORKSPACE_TILES;
}

function equalRatios(n: number): number[] {
  if (n <= 0) return [];
  return Array.from({ length: n }, () => 1 / n);
}

/** Default column weights for the board. Agent joining a tool is 1/3, not half. */
export function defaultSliceRatios(tiles: WorkspaceTile[]): number[] {
  if (tiles.length <= 0) return [];
  if (tiles.length === 1) return [1];
  if (tiles.length === 2 && tiles.some(isToolTile) && tiles.some(isChatTile)) {
    return [...TOOL_AGENT_SPLIT];
  }
  return equalRatios(tiles.length);
}

export function formatSliceRatios(ratios: number[]): string {
  return ratios.map((r) => r.toFixed(3)).join(" ");
}

export function WorkspaceCanvas({
  mode,
  tiles,
  catalog,
  onTilesChange,
  onOpenChat: _onOpenChat,
  onSendInChat,
  pendingExcerptsByChat,
  onPendingExcerptsChange,
  onOpenBoard,
  onOpenTile,
  onOpenSessionTool,
  onApproveRun,
  onAttachAgent,
  onSelectTool: _onSelectTool,
  busyChatIds,
  approveBusy,
  renderPanel,
  bridge,
  scopes: _scopes,
  systemPermissions: _systemPermissions,
  isAdmin: _isAdmin,
  handoff,
  onHandoffConsumed,
  onStagedMutations,
  onStageProposal,
  onPinToGraph,
  onAskAgent,
  onOpenInQuery,
}: {
  mode: AppSection;
  tiles: WorkspaceTile[];
  catalog: AgentChat[];
  onTilesChange: (tiles: WorkspaceTile[]) => void;
  onOpenChat: (chat: AgentChat) => void;
  onSendInChat: (chatId: string, text: string, excerpts?: import("./contextExcerpt").ContextExcerpt[]) => void;
  pendingExcerptsByChat?: Record<string, import("./contextExcerpt").ContextExcerpt[]>;
  onPendingExcerptsChange?: (chatId: string, excerpts: import("./contextExcerpt").ContextExcerpt[]) => void;
  onOpenBoard?: (handoff: BoardHandoff) => void;
  onOpenTile?: (id: TileId) => void;
  onOpenSessionTool?: (toolId: string) => void;
  onApproveRun?: (runId: string, chatId: string) => void;
  onAttachAgent?: (agentId: string) => void;
  /** Reserved for empty-state tool CTAs (prompt-only empty board for now). */
  onSelectTool?: (id: string) => void;
  busyChatIds?: string[];
  approveBusy?: boolean;
  renderPanel: (panelId: TileId) => ReactNode;
  bridge?: AppBridge;
  scopes?: string[];
  systemPermissions?: string[];
  isAdmin?: boolean;
  handoff?: BoardHandoff | null;
  onHandoffConsumed?: () => void;
  onStagedMutations?: (count: number) => void;
  onStageProposal?: (handoff: BoardHandoff) => Promise<void>;
  onPinToGraph?: (handoff: BoardHandoff) => Promise<void>;
  /** Route a short prompt into the primary Operate chat (tool → agent composition). */
  onAskAgent?: (prompt: string) => void;
  /** Open Query panel for an object from chat handoff. */
  onOpenInQuery?: (objectApiName: string) => void;
}) {
  const [ratios, setRatios] = useState<number[]>(() => defaultSliceRatios(tiles));
  const [resizing, setResizing] = useState(false);
  const splitDrag = useRef<{
    handleIndex: number;
    pointerId: number;
    startX: number;
    startLeft: number;
    startRight: number;
  } | null>(null);
  const slicesRef = useRef<HTMLDivElement>(null);
  const ratiosRef = useRef(ratios);
  ratiosRef.current = ratios;
  const tilesRef = useRef(tiles);
  tilesRef.current = tiles;

  useEffect(() => {
    setRatios(defaultSliceRatios(tilesRef.current));
  }, [tiles.length]);

  const removeTile = (id: string) => onTilesChange(tiles.filter((t) => t.id !== id));

  const emptyDescription =
    mode === "operate"
      ? "Open a Tool from the left rail, or ask an agent on the right to compose one."
      : mode === "settings"
        ? "Open Account, Hosting, Inference, or Environments from the left rail, or choose a Settings agent."
        : "Open a tool from the left rail, or choose an agent from the right catalog.";

  const renderTileBody = (tile: WorkspaceTile) => {
    if (tile.kind === "chat" && tile.chatId) {
      const chat = catalog.find((c) => c.id === tile.chatId);
      if (!chat) return null;
      return (
        <AgentChatPane
          chat={chat}
          onSend={onSendInChat}
          onOpenBoard={onOpenBoard}
          onOpenTile={onOpenTile}
          onOpenSessionTool={onOpenSessionTool}
          onApprove={onApproveRun ? (runId) => onApproveRun(runId, chat.id) : undefined}
          onAttachAgent={chat.primary || mode === "operate" ? onAttachAgent : undefined}
          onStageMutations={onStagedMutations}
          onStageProposal={onStageProposal}
          onPinToGraph={onPinToGraph}
          onOpenInQuery={onOpenInQuery}
          acceptAgentDrop={false}
          busy={busyChatIds?.includes(chat.id)}
          approveBusy={approveBusy}
          pendingExcerpts={pendingExcerptsByChat?.[chat.id] ?? []}
          onPendingExcerptsChange={
            onPendingExcerptsChange
              ? (excerpts) => onPendingExcerptsChange(chat.id, excerpts)
              : undefined
          }
        />
      );
    }
    if (tile.kind === "crm") {
      return (
        <CrmPanel
          bridge={bridge}
          onClose={() => removeTile(tile.id)}
          handoff={handoff}
          onHandoffConsumed={onHandoffConsumed}
          onStagedMutations={onStagedMutations}
          onAskAgent={onAskAgent}
        />
      );
    }
    if (tile.kind === "panel" && tile.panelId) return renderPanel(tile.panelId);
    return null;
  };

  const tileLabel = (tile: WorkspaceTile) => {
    if (tile.kind === "chat") return "Chat";
    if (tile.kind === "crm") return "CRM";
    if (tile.panelId) return TILE_META[tile.panelId].label;
    return "Panel";
  };

  const tileChrome = (tile: WorkspaceTile) => (
    <div className="workspace-tile-chrome">
      <span className="visually-hidden" data-testid={`resize-${tile.id}`}>
        {tileLabel(tile)}
      </span>
      <button
        type="button"
        className="secondary icon-btn workspace-tile-close"
        aria-label={
          tile.kind === "chat" && tile.chatId === PRIMARY_OPERATE_CHAT_ID
            ? "Close Operate session"
            : `Close ${tileLabel(tile)}`
        }
        data-testid={`close-tile-${tile.id}`}
        onClick={() => removeTile(tile.id)}
      >
        ×
      </button>
    </div>
  );

  const endSplitDrag = useCallback(() => {
    splitDrag.current = null;
    setResizing(false);
  }, []);

  const applySplitMove = useCallback((clientX: number) => {
    if (!splitDrag.current || !slicesRef.current) return;
    const width = slicesRef.current.getBoundingClientRect().width;
    if (width <= 0) return;
    const { handleIndex, startX, startLeft, startRight } = splitDrag.current;
    const pair = startLeft + startRight;
    const delta = (clientX - startX) / width;
    const nextLeft = Math.min(pair - MIN_RATIO, Math.max(MIN_RATIO, startLeft + delta));
    const nextRight = pair - nextLeft;
    setRatios((prev) => {
      const next = [...prev];
      next[handleIndex] = nextLeft;
      next[handleIndex + 1] = nextRight;
      return next;
    });
  }, []);

  const onSplitPointerDown = useCallback(
    (handleIndex: number, e: ReactPointerEvent<HTMLDivElement>) => {
      // Primary button only. jsdom PointerEvents may omit `button`; treat that as primary.
      // Non-zero buttons (right/middle) must not start a resize.
      if (e.button > 0) return;
      e.preventDefault();
      e.stopPropagation();
      const current = ratiosRef.current;
      const left = current[handleIndex] ?? 0;
      const right = current[handleIndex + 1] ?? 0;
      splitDrag.current = {
        handleIndex,
        pointerId: e.pointerId,
        startX: e.clientX,
        startLeft: left,
        startRight: right,
      };
      // Capture immediately so Monaco / graph canvases cannot swallow the drag.
      try {
        e.currentTarget.setPointerCapture(e.pointerId);
      } catch {
        /* jsdom / older Electron */
      }
      setResizing(true);
    },
    [],
  );

  const onSplitPointerMove = useCallback(
    (e: ReactPointerEvent<HTMLDivElement>) => {
      if (!splitDrag.current) return;
      e.preventDefault();
      applySplitMove(e.clientX);
    },
    [applySplitMove],
  );

  const onSplitPointerUp = useCallback(
    (e: ReactPointerEvent<HTMLDivElement>) => {
      if (!splitDrag.current) return;
      try {
        if (e.currentTarget.hasPointerCapture(e.pointerId)) {
          e.currentTarget.releasePointerCapture(e.pointerId);
        }
      } catch {
        /* jsdom / older Electron */
      }
      endSplitDrag();
    },
    [endSplitDrag],
  );

  useEffect(() => {
    if (!resizing) return;
    const onMove = (e: PointerEvent) => {
      if (!splitDrag.current) return;
      if (e.buttons === 0) {
        endSplitDrag();
        return;
      }
      applySplitMove(e.clientX);
    };
    const onUp = () => {
      endSplitDrag();
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onUp);
    window.addEventListener("blur", onUp);
    return () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
      window.removeEventListener("blur", onUp);
    };
  }, [resizing, applySplitMove, endSplitDrag]);

  const countClass =
    tiles.length === 0
      ? "count-0"
      : tiles.length === 1
        ? "count-1"
        : tiles.length === 2
          ? "count-2"
          : tiles.length === 3
            ? "count-3"
            : "count-4";

  const sliceStyle: CSSProperties | undefined =
    tiles.length >= 2
      ? {
          gridTemplateColumns: ratios
            .flatMap((r, i) =>
              i === 0 ? [`minmax(0, ${r}fr)`] : [`${SPLIT_HANDLE_PX}px`, `minmax(0, ${r}fr)`],
            )
            .join(" "),
        }
      : undefined;

  const splitHandle = (handleIndex: number) => {
    const left = ratios[handleIndex] ?? 0;
    return (
      <div
        className="workspace-split-handle"
        data-testid={handleIndex === 0 ? "workspace-split-handle" : `workspace-split-handle-${handleIndex}`}
        role="separator"
        aria-orientation="vertical"
        aria-valuenow={Math.round(left * 100)}
        aria-valuemin={Math.round(MIN_RATIO * 100)}
        aria-valuemax={Math.round((1 - MIN_RATIO) * 100)}
        onPointerDown={(e) => onSplitPointerDown(handleIndex, e)}
        onPointerMove={onSplitPointerMove}
        onPointerUp={onSplitPointerUp}
        onPointerCancel={onSplitPointerUp}
      />
    );
  };

  return (
    <section className="workspace-shell" data-testid="workspace-canvas" aria-label="Workspace">
      <div
        className={`workspace-board ${tiles.length === 0 ? "is-empty" : ""} ${tiles.length === 1 ? "single-tile" : ""} ${resizing ? "is-resizing" : ""}`}
        data-testid="workspace-board"
        data-resizing={resizing ? "true" : "false"}
      >
        {tiles.length === 0 ? (
          <div className="workspace-empty" data-testid="workspace-empty">
            <EmptyState
              icon={<IconDropTarget size={48} />}
              title="Add a tool or Agent"
              description={emptyDescription}
            />
          </div>
        ) : (
          <div
            ref={slicesRef}
            className={`workspace-slices ${countClass}`}
            data-testid="workspace-slices"
            data-count={tiles.length}
            data-ratios={formatSliceRatios(ratios)}
            style={sliceStyle}
          >
            {tiles.map((tile, index) => (
              <Fragment key={tile.id}>
                {index > 0 ? splitHandle(index - 1) : null}
                <div
                  className={`workspace-tile ${tiles.length === 1 ? "is-solo" : ""}${tile.kind === "panel" && tile.panelId === "runGraph" ? " is-immersive" : ""}`}
                  data-testid={`workspace-tile-${tile.id}`}
                >
                  {tile.kind === "panel" && tile.panelId === "runGraph" ? null : tileChrome(tile)}
                  <div className="workspace-tile-body">{renderTileBody(tile)}</div>
                </div>
              </Fragment>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

/** Helper used by App when dropping a chat — default tall layout. */
export function makeChatTile(chatId: string): WorkspaceTile {
  return { id: `chat-${chatId}`, kind: "chat", chatId, ...defaultChatLayout() };
}

export function makePrimaryChatTile(chatId = PRIMARY_OPERATE_CHAT_ID): WorkspaceTile {
  return { id: `chat-${chatId}`, kind: "chat", chatId, ...primaryChatLayout() };
}

export function makeCrmTile(): WorkspaceTile {
  return { id: "crm", kind: "crm", colSpan: 2, rowSpan: 1 };
}

export function makePanelTile(panelId: TileId): WorkspaceTile {
  return { id: `panel-${panelId}`, kind: "panel", panelId, colSpan: 2, rowSpan: 2 };
}
