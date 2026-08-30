import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  RAIL_DOCK_PX,
  RAIL_FLYOUT_PX,
  RAIL_STRIP_PX,
  workspaceRailClassNames,
  workspaceRailDocking,
} from "./workspaceRailLayout";

const stylesCss = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), "../styles.css"), "utf8");

describe("workspaceRailLayout", () => {
  it("keeps docked column equal to strip plus flyout so pin does not resize the catalog", () => {
    expect(RAIL_DOCK_PX).toBe(RAIL_STRIP_PX + RAIL_FLYOUT_PX);
    expect(RAIL_FLYOUT_PX).toBe(240);
    expect(RAIL_STRIP_PX).toBe(44);
    expect(RAIL_DOCK_PX).toBe(284);
  });

  it("docks both rails only on a connected empty board", () => {
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 0,
        seedingDefaultTool: false,
        toolsPinned: false,
        agentsPinned: false,
      }),
    ).toEqual({ empty: true, toolsDocked: true, agentsDocked: true });
  });

  it("does not flash empty docks while a default tool is seeding", () => {
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 0,
        seedingDefaultTool: true,
        toolsPinned: false,
        agentsPinned: false,
      }),
    ).toEqual({ empty: false, toolsDocked: false, agentsDocked: false });
  });

  it("retracts both after a tile is selected unless a rail is pinned", () => {
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 1,
        seedingDefaultTool: false,
        toolsPinned: false,
        agentsPinned: false,
      }),
    ).toEqual({ empty: false, toolsDocked: false, agentsDocked: false });
  });

  it("pins one rail without docking the other", () => {
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 1,
        seedingDefaultTool: false,
        toolsPinned: true,
        agentsPinned: false,
      }),
    ).toEqual({ empty: false, toolsDocked: true, agentsDocked: false });
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 1,
        seedingDefaultTool: false,
        toolsPinned: false,
        agentsPinned: true,
      }),
    ).toEqual({ empty: false, toolsDocked: false, agentsDocked: true });
  });

  it("keeps a user pin after the board becomes empty (both still docked)", () => {
    expect(
      workspaceRailDocking({
        connected: true,
        entered: true,
        tileCount: 0,
        seedingDefaultTool: false,
        toolsPinned: true,
        agentsPinned: false,
      }),
    ).toEqual({ empty: true, toolsDocked: true, agentsDocked: true });
  });

  it("does not let agent hover/collapsed classes rewrite the 3-column workspace grid", () => {
    expect(stylesCss).not.toMatch(/\.workspace:has\(\.agent-stream\.collapsed\)/);
    expect(stylesCss).not.toMatch(/workspace-single:has\(\.agent-stream/);
    expect(stylesCss).not.toMatch(/\.workspace-slices\.count-2[^{]*\{[^}]*grid-template-columns:\s*none/);
    expect(stylesCss).toMatch(
      /\.workspace-slices\.count-2\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*2fr\)\s+10px\s+minmax\(0,\s*1fr\)/,
    );
    expect(stylesCss).toMatch(
      /\.workspace\.workspace-single\s*\{[^}]*grid-template-columns:\s*var\(--rail-strip\)\s+minmax\(0,\s*1fr\)\s+var\(--rail-strip\)/,
    );
    expect(stylesCss).toMatch(
      /\.workspace\.workspace-single\.tools-rail-docked\s*\{[^}]*grid-template-columns:\s*var\(--rail-dock\)\s+minmax\(0,\s*1fr\)\s+var\(--rail-strip\)/,
    );
    expect(stylesCss).toMatch(
      /\.workspace\.workspace-single\.agents-rail-docked\s*\{[^}]*grid-template-columns:\s*var\(--rail-strip\)\s+minmax\(0,\s*1fr\)\s+var\(--rail-dock\)/,
    );
    expect(stylesCss).toMatch(
      /\.workspace\.boot-skel-workspace\s*\{[^}]*grid-template-columns:\s*112px\s+168px/,
    );
  });

  it("emits stable grid class names for each dock combination", () => {
    expect(
      workspaceRailClassNames({ empty: false, toolsDocked: false, agentsDocked: false }),
    ).toBe("workspace workspace-single");
    expect(
      workspaceRailClassNames({ empty: false, toolsDocked: true, agentsDocked: false }),
    ).toBe("workspace workspace-single tools-rail-docked");
    expect(
      workspaceRailClassNames({ empty: false, toolsDocked: false, agentsDocked: true }),
    ).toBe("workspace workspace-single agents-rail-docked");
    expect(
      workspaceRailClassNames({ empty: true, toolsDocked: true, agentsDocked: true }),
    ).toBe("workspace workspace-single tools-rail-docked agents-rail-docked");
  });
});
