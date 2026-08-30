import { useEffect, useMemo, useState } from "react";
import { IconPin, IconSearch, IconTools } from "../icons/Icons";
import { catalogCardCopy } from "./catalogCard";
import { TILE_META, type TileId } from "./types";
import { useHoverRail } from "./useHoverRail";
import { RAIL_FLYOUT_PX } from "./workspaceRailLayout";
import { OPERATE_TOOL_DRAG_MIME, type OperateToolDragPayload } from "./operateToolDrag";

/** Left-rail tool entry — static TileId or live `tool:<apiName>` / `session:<id>`. */
export type RailToolItem = {
  id: string;
  label: string;
  /** Short description under the title (matches agent catalog cards). */
  summary?: string;
  /** Optional uppercase kicker; defaults to a cleaned id. */
  name?: string;
};

function toolKicker(tool: RailToolItem): string {
  if (tool.name?.trim()) return tool.name.trim();
  if (tool.id.startsWith("tool:")) return tool.id.slice(5);
  if (tool.id.startsWith("session:")) return tool.id.slice(8);
  return tool.id;
}

function toolSummary(tool: RailToolItem): string {
  if (tool.summary?.trim()) return tool.summary.trim();
  const meta = TILE_META[tool.id as TileId];
  return meta?.summary ?? "Open this tool in the workspace.";
}

function matchesQuery(tool: RailToolItem, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return (
    tool.label.toLowerCase().includes(q) ||
    toolKicker(tool).toLowerCase().includes(q) ||
    toolSummary(tool).toLowerCase().includes(q) ||
    tool.id.toLowerCase().includes(q)
  );
}

export function WorkspaceToolRail({
  tools,
  openToolIds,
  atCap,
  onSelectTool,
  onCloseTool,
  pinned: pinnedProp,
  forceExpanded = false,
  dismissNonce,
  onPinnedChange,
  dragPayloadForTool,
}: {
  tools: RailToolItem[];
  openToolIds: string[];
  atCap: boolean;
  onSelectTool: (id: string) => void;
  /** Remove an open tool panel from the workspace. */
  onCloseTool?: (id: string) => void;
  /** Controlled pin; when omitted the rail keeps its own pin state. */
  pinned?: boolean;
  /** Dock both catalogs when the workspace has no tool or agent. */
  forceExpanded?: boolean;
  dismissNonce?: number;
  /** Fired when the rail is click-pinned (workspace should resize). */
  onPinnedChange?: (pinned: boolean) => void;
  /** Operate-only palette payload. When provided, the card can be dropped on My graph. */
  dragPayloadForTool?: (tool: RailToolItem) => OperateToolDragPayload | null;
}) {
  const [query, setQuery] = useState("");
  const { open, pinned, docked, setPinned, togglePinned, collapseHover, rootPointerHandlers } =
    useHoverRail({
      pinned: pinnedProp,
      onPinnedChange,
      forceExpanded,
      dismissNonce,
    });
  const visible = useMemo(() => tools.filter((t) => matchesQuery(t, query)), [tools, query]);
  const anyActive = openToolIds.length > 0;

  useEffect(() => {
    if (!open) setQuery("");
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (pinned) setPinned(false);
        setQuery("");
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, pinned, setPinned]);

  return (
    <aside
      className={`workspace-tool-rail hover-rail ${open ? "is-open" : ""} ${docked ? "is-docked" : ""} ${pinned ? "is-pinned" : ""}`}
      data-testid="workspace-tool-rail"
      data-docked={docked ? "true" : "false"}
      data-pinned={pinned ? "true" : "false"}
      aria-label="Workspace tools"
      {...rootPointerHandlers}
    >
      <div className="workspace-tool-rail-strip hover-rail-strip" aria-hidden={open && !docked}>
        <button
          type="button"
          className={`stream-expand icon-rail ${anyActive ? "active" : ""} ${pinned ? "pinned" : ""}`}
          data-testid="tool-rail-expand"
          title={pinned ? "Unpin tools" : "Pin tools catalog"}
          aria-label="Tool catalog"
          aria-expanded={open}
          aria-pressed={pinned}
          tabIndex={open && !docked ? -1 : 0}
          onClick={togglePinned}
        >
          <IconTools size={16} />
          <span>Tools</span>
        </button>
      </div>

      {open ? (
        <div
          className="workspace-tool-rail-flyout hover-rail-flyout"
          data-testid="workspace-tool-rail-flyout"
          data-flyout-width={RAIL_FLYOUT_PX}
        >
          <div className="workspace-tool-rail-catalog" data-testid="workspace-tool-rail-catalog">
            <p className="stream-section-title hover-rail-dock-title">
              Tools
              {docked ? (
                <IconPin size={12} className={`hover-rail-pin-glyph ${pinned ? "is-active" : ""}`} />
              ) : null}
            </p>
            <label className="rail-search" data-testid="tool-search">
              <IconSearch size={14} />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search by name…"
                aria-label="Search tools by name"
              />
            </label>
            <div className="workspace-tool-rail-list rail-scroll" data-testid="workspace-tool-rail-list">
              {visible.length === 0 ? (
                <p className="muted stream-empty-agents" data-testid="tool-rail-empty">
                  No tools{query ? " match your search" : ""}.
                </p>
              ) : (
                visible.map((tool) => {
                  const active = openToolIds.includes(tool.id);
                  const blocked = !active && atCap;
                  const dragPayload = dragPayloadForTool?.(tool) ?? null;
                  const card = catalogCardCopy(tool.label, tool.name?.trim() ? toolKicker(tool) : undefined);
                  return (
                    <div
                      key={tool.id}
                      className={`tool-catalog-card ${active ? "active" : ""} ${blocked ? "blocked" : ""} ${dragPayload ? "is-draggable" : ""}`}
                      draggable={Boolean(dragPayload)}
                      onDragStart={(event) => {
                        if (!dragPayload) return;
                        event.dataTransfer.effectAllowed = "copyLink";
                        event.dataTransfer.setData(OPERATE_TOOL_DRAG_MIME, JSON.stringify(dragPayload));
                      }}
                    >
                      <button
                        type="button"
                        className="tool-catalog-open"
                        data-testid={`tool-rail-${tool.id}`}
                        disabled={blocked}
                        onClick={() => {
                          collapseHover();
                          onSelectTool(tool.id);
                        }}
                      >
                        {card.kicker ? <p className="tool-catalog-kicker muted">{card.kicker}</p> : null}
                        <p className="tool-catalog-label">{card.title}</p>
                        <p className="muted tool-catalog-summary">{toolSummary(tool)}</p>
                        {active ? <span className="agent-pinned-badge">In workspace</span> : null}
                        {dragPayload ? <span className="tool-catalog-drag-hint">Drag onto My graph</span> : null}
                      </button>
                      {active && onCloseTool ? (
                        <button
                          type="button"
                          className="workspace-rail-close"
                          data-testid={`tool-rail-close-${tool.id}`}
                          aria-label={`Remove ${tool.label} from workspace`}
                          title="Remove from workspace"
                          onClick={() => onCloseTool(tool.id)}
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
