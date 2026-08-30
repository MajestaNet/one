import { describe, expect, it, vi } from "vitest";
import { listTools, parseSessionToolRailId, parseToolRailId, sessionToolRailId, toolRailId, toolSpecToDocument } from "./tools";
import { demoAccountsTool } from "./fixtures";

describe("run/tools", () => {
  it("builds and parses tool rail ids", () => {
    expect(toolRailId("Open_Pipeline")).toBe("tool:Open_Pipeline");
    expect(parseToolRailId("tool:Open_Pipeline")).toBe("Open_Pipeline");
    expect(parseToolRailId("objectHome")).toBeNull();
    expect(sessionToolRailId("run-1")).toBe("session:run-1");
    expect(parseSessionToolRailId("session:run-1")).toBe("run-1");
  });

  it("lists active tools sorted by sortOrder then label", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      tools: [
        { apiName: "B", label: "Bravo", sortOrder: 2, active: true },
        { apiName: "A", label: "Alpha", sortOrder: 1, active: true },
        { apiName: "Z", label: "Hidden", sortOrder: 0, active: false },
      ],
    });
    const tools = await listTools(fetchFn);
    expect(fetchFn).toHaveBeenCalledWith("/metadata/v1/tools");
    expect(tools.map((t) => t.apiName)).toEqual(["A", "B"]);
  });

  it("lists Client tools for Run rail (permission-filtered)", async () => {
    const { listClientTools } = await import("./tools");
    const fetchFn = vi.fn().mockResolvedValue({
      tools: [
        { apiName: "Sales_Open_Pipeline", label: "Pipeline", sortOrder: 1 },
        { apiName: "A", label: "Alpha", sortOrder: 0 },
      ],
    });
    const tools = await listClientTools(fetchFn);
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/tools");
    expect(tools.map((t) => t.apiName)).toEqual(["A", "Sales_Open_Pipeline"]);
  });

  it("maps nested document payloads", () => {
    const fixture = demoAccountsTool();
    const doc = toolSpecToDocument({
      apiName: "X",
      label: "X",
      document: fixture,
    });
    expect(doc?.nodes.length).toBe(fixture.nodes.length);
  });
});
