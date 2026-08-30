import { useEffect, useMemo, useState } from "react";
import type { AppBridge } from "../App";
import type { BoardHandoff } from "../operate/types";
import { sectionLabel } from "../agents/sections";
import {
  agentsForMode,
  statusLabel,
  type AgentChat,
  type ChangeStatus,
  type TileId,
  type AppSection,
} from "./types";
import { IconAgents, IconPin, IconSearch } from "../icons/Icons";
import { EmptyState } from "../ui";
import { catalogCardCopy } from "./catalogCard";
import { useHoverRail } from "./useHoverRail";
import { RAIL_FLYOUT_PX } from "./workspaceRailLayout";

const AGENT_EMPTY_COPY: Record<AppSection, { title: string; description: string }> = {
  operate: {
    title: "No Operate agents on this install",
    description: "Enable AgentSpecs on the install or connect an external runtime via MCP.",
  },
  build: {
    title: "No Build agents on this install",
    description: "Author AgentSpecs under Build → Agents or connect an external runtime via MCP.",
  },
  govern: {
    title: "No Govern agents on this install",
    description: "Add Govern-section AgentSpecs for identity and install policy guidance.",
  },
  settings: {
    title: "No Settings agents on this install",
    description: "Add Settings-section AgentSpecs for Account, hosting, and inference guidance.",
  },
};

/**
 * Right-rail Agents catalog (hover flyout). Opens 1:1 chats in the workspace —
 * no mini-composer on the hover strip (chat lives in the board tile).
 */
export function AgentStreamDock({
  changeTitle,
  changeStatus,
  mode,
  agentChats,
  pinnedChatIds,
  onOpenTile: _onOpenTile,
  onOpenBoard: _onOpenBoard,
  onAttachAgent,
  onCloseAgent,
  onSendToPrimary: _onSendToPrimary,
  onComposerSubmit: _onComposerSubmit,
  catalogOnly = true,
  connected = true,
  onGoConnect: _onGoConnect,
  bridge: _bridge,
  selectedPlaybook: _selectedPlaybook,
  pinned: pinnedProp,
  forceExpanded = false,
  dismissNonce,
  onPinnedChange,
  /** @deprecated Hover flyout replaces click collapse; ignored. */
  collapsed: _collapsed,
  /** @deprecated Hover flyout replaces click collapse; ignored. */
  onToggleCollapsed: _onToggleCollapsed,
}: {
  /** @deprecated Hover flyout — prop kept for call-site compat. */
  collapsed?: boolean;
  /** @deprecated Hover flyout — prop kept for call-site compat. */
  onToggleCollapsed?: () => void;
  changeTitle: string;
  changeStatus: ChangeStatus;
  mode: AppSection;
  agentChats: AgentChat[];
  pinnedChatIds: string[];
  onOpenTile: (id: TileId) => void;
  onOpenBoard?: (handoff: BoardHandoff) => void;
  onAttachAgent?: (agentId: string) => void;
  /** Remove an open agent chat tile from the workspace. */
  onCloseAgent?: (agentId: string) => void;
  /** @deprecated Dock is catalog-only; chat composer lives on workspace tiles. */
  onSendToPrimary?: (text: string) => void;
  /** @deprecated Dock is catalog-only; chat composer lives on workspace tiles. */
  onComposerSubmit?: (text: string) => void;
  /** @deprecated Operate catalog no longer shows drag-hint copy. */
  agentsDropEnabled?: boolean;
  /** Prefer catalog-first empty states. Defaults true. */
  catalogOnly?: boolean;
  connected?: boolean;
  onGoConnect?: () => void;
  bridge?: AppBridge;
  /** @deprecated Settings agents open in the shared workspace board. */
  selectedPlaybook?: string;
  /** Controlled pin; when omitted the dock keeps its own pin state. */
  pinned?: boolean;
  /** Dock both catalogs when the workspace has no tool or agent. */
  forceExpanded?: boolean;
  dismissNonce?: number;
  /** Fired when the rail is click-pinned (workspace should resize). */
  onPinnedChange?: (pinned: boolean) => void;
}) {
  const [query, setQuery] = useState("");
  const { open: flyoutOpen, pinned, docked, setPinned, togglePinned, collapseHover, rootPointerHandlers } =
    useHoverRail({
      pinned: pinnedProp,
      onPinnedChange,
      forceExpanded,
      dismissNonce,
    });
  const anyActive = pinnedChatIds.length > 0;

  const visibleAgents = useMemo(
    () => agentsForMode(agentChats, mode, query),
    [agentChats, mode, query],
  );

  useEffect(() => {
    if (!flyoutOpen) setQuery("");
  }, [flyoutOpen]);

  useEffect(() => {
    if (!flyoutOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && pinned) setPinned(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [flyoutOpen, pinned, setPinned]);

  const emptyCopy = AGENT_EMPTY_COPY[mode];

  return (
    <aside
      className={`agent-stream hover-rail ${flyoutOpen ? "is-open" : ""} ${docked ? "is-docked" : ""} ${pinned ? "is-pinned" : ""} catalog-only`}
      data-testid="agent-stream"
      data-docked={docked ? "true" : "false"}
      data-pinned={pinned ? "true" : "false"}
      aria-label="Agents"
      {...rootPointerHandlers}
    >
      <div className="agent-stream-strip hover-rail-strip">
        <button
          type="button"
          className={`stream-expand icon-rail ${anyActive ? "active" : ""} ${pinned ? "pinned" : ""}`}
          data-testid="agent-stream-expand"
          title={pinned ? "Unpin agents" : "Pin agents catalog"}
          aria-label="Agents"
          aria-expanded={flyoutOpen}
          aria-pressed={pinned}
          onClick={togglePinned}
        >
          <IconAgents size={16} />
          <span>Agents</span>
        </button>
      </div>

      {flyoutOpen ? (
        <div
          className="agent-stream-flyout hover-rail-flyout"
          data-testid="agent-stream-flyout"
          data-flyout-width={RAIL_FLYOUT_PX}
        >
          {docked ? null : (
            <header className="stream-header">
              <div>
                <p className="stream-kicker">Change</p>
                <h2 className="stream-title">{changeTitle}</h2>
                <p className="muted stream-status">{statusLabel(changeStatus)}</p>
              </div>
            </header>
          )}

          <div className="stream-agent-catalog" data-testid="stream-agent-cards">
            <p className="stream-section-title hover-rail-dock-title">
              Agents
              {docked ? (
                <IconPin size={12} className={`hover-rail-pin-glyph ${pinned ? "is-active" : ""}`} />
              ) : null}
            </p>
            <label className="rail-search" data-testid="agent-search">
              <IconSearch size={14} />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search by name…"
                aria-label="Search agents by name"
              />
            </label>
            <div className="stream-agent-list rail-scroll" data-testid="stream-agent-list">
              {visibleAgents.length === 0 ? (
                catalogOnly && !query && connected ? (
                  <div data-testid={`stream-agents-byo-empty-${mode}`}>
                    <EmptyState title={emptyCopy.title} description={emptyCopy.description} />
                  </div>
                ) : (
                  <p className="muted stream-empty-agents" data-testid="stream-agents-empty">
                    No {sectionLabel(mode)} agents
                    {query ? " match your search" : ""}.
                  </p>
                )
              ) : (
                visibleAgents.map((chat) => {
                  const pinnedInWorkspace = pinnedChatIds.includes(chat.id);
                  const badge = chat.demoStub
                    ? null
                    : chat.id.startsWith("playbook-")
                      ? "Live"
                      : connected
                        ? "Seed"
                        : null;
                  const card = catalogCardCopy(chat.title || chat.agentName, chat.agentName);
                  return (
                    <div
                      key={chat.id}
                      className={`agent-catalog-card ${pinnedInWorkspace ? "pinned" : ""}`}
                      data-testid={`agent-drag-${chat.id}`}
                    >
                      <button
                        type="button"
                        className="agent-catalog-open"
                        data-testid={`agent-use-${chat.id}`}
                        onClick={() => {
                          collapseHover();
                          onAttachAgent?.(chat.id);
                        }}
                        disabled={!onAttachAgent}
                      >
                        <div className="agent-drag-title-row">
                          <p className="agent-drag-title">{card.title}</p>
                          {badge ? (
                            <span
                              className={`agent-badge agent-badge-${badge.toLowerCase()}`}
                              data-testid={`agent-badge-${chat.id}`}
                            >
                              {badge}
                            </span>
                          ) : null}
                        </div>
                        {card.kicker ? <p className="agent-drag-name muted">{card.kicker}</p> : null}
                        <p className="muted agent-drag-summary">{chat.summary}</p>
                        {pinnedInWorkspace ? <span className="agent-pinned-badge">In workspace</span> : null}
                      </button>
                      {pinnedInWorkspace && onCloseAgent ? (
                        <button
                          type="button"
                          className="workspace-rail-close"
                          data-testid={`agent-close-${chat.id}`}
                          aria-label={`Remove ${chat.agentName || chat.title} from workspace`}
                          title="Remove from workspace"
                          onClick={() => onCloseAgent(chat.id)}
                        >
                          ×
                        </button>
                      ) : null}
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      ) : null}
    </aside>
  );
}
