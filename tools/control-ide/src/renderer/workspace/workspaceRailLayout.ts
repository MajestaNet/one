/** Collapsed hover-strip width (px). */
export const RAIL_STRIP_PX = 44;
/** Catalog flyout width (px) — hover overlay and docked catalog share this. */
export const RAIL_FLYOUT_PX = 240;
/** In-flow docked column = strip + flyout so pin does not change catalog size. */
export const RAIL_DOCK_PX = RAIL_STRIP_PX + RAIL_FLYOUT_PX;

/**
 * Live workspace grid is always 3 columns: tool rail | board | agent rail.
 * Hover open/close must overlay inside those columns — never change
 * `grid-template-columns`. Pin / empty-board dock is the only width change,
 * via `tools-rail-docked` / `agents-rail-docked`.
 */

export type WorkspaceRailDockingInput = {
  connected: boolean;
  entered: boolean;
  tileCount: number;
  /** Operate/Settings seed a default tool; do not flash empty-board docks. */
  seedingDefaultTool: boolean;
  toolsPinned: boolean;
  agentsPinned: boolean;
};

export type WorkspaceRailDocking = {
  empty: boolean;
  toolsDocked: boolean;
  agentsDocked: boolean;
};

/** Pure dock flags for the workspace grid — empty board docks both; pin is per-rail. */
export function workspaceRailDocking(input: WorkspaceRailDockingInput): WorkspaceRailDocking {
  const empty = Boolean(
    input.connected && input.entered && input.tileCount === 0 && !input.seedingDefaultTool,
  );
  return {
    empty,
    toolsDocked: input.toolsPinned || empty,
    agentsDocked: input.agentsPinned || empty,
  };
}

export function workspaceRailClassNames(docking: WorkspaceRailDocking, extra?: string): string {
  return [
    "workspace",
    "workspace-single",
    docking.toolsDocked ? "tools-rail-docked" : "",
    docking.agentsDocked ? "agents-rail-docked" : "",
    extra ?? "",
  ]
    .filter(Boolean)
    .join(" ");
}
