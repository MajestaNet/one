import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToolDocumentView } from "./ToolDocumentView";
import { demoAccountsTool, demoPipelineTool } from "./fixtures";
import { toolSpecToDocument } from "./tools";

afterEach(() => {
  cleanup();
});

describe("ToolDocumentView", () => {
  it("renders section table and markdown from the demo fixture", () => {
    render(<ToolDocumentView document={demoAccountsTool()} />);
    expect(screen.getByTestId("canvas-document")).toBeTruthy();
    expect(screen.getByTestId("canvas-node-stat-open")).toBeTruthy();
    expect(screen.getByTestId("canvas-stat-value-stat-open").textContent).toBe("3");
    expect(screen.getByTestId("run-table-table")).toBeTruthy();
    expect(screen.getAllByText("Acme Corp").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId("run-tool-markdown")).toBeTruthy();
  });

  it("renders spatial pipeline shell with lanes", () => {
    render(<ToolDocumentView document={demoPipelineTool()} />);
    expect(screen.getByTestId("canvas-spatial-viewport")).toBeTruthy();
    expect(screen.getByTestId("canvas-node-lane-prospect")).toBeTruthy();
    expect(screen.getAllByTestId("run-pipeline-lane").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId("run-pipeline-card-c1").textContent).toMatch(/Acme renewal/);
  });
});

describe("toolSpecToDocument", () => {
  it("synthesizes a document from Metadata ToolSpec fields", () => {
    const doc = toolSpecToDocument({
      apiName: "Open_Pipeline",
      label: "Open pipeline",
      layout: demoAccountsTool().layout,
      nodes: demoAccountsTool().nodes,
      dataBindings: demoAccountsTool().dataBindings,
    });
    expect(doc?.id).toBe("Open_Pipeline");
    expect(doc?.title).toBe("Open pipeline");
    expect(doc?.nodes.some((n) => n.kind === "recordTable")).toBe(true);
  });

  it("returns null when a malicious kind is present", () => {
    const doc = toolSpecToDocument({
      apiName: "Bad",
      label: "Bad",
      layout: { mode: "sections", sections: [] },
      nodes: [{ id: "x", kind: "iframe", props: { src: "https://evil.example" } }],
    });
    expect(doc).toBeNull();
  });
});

describe("getTool path", () => {
  it("is covered via tools helper import smoke", async () => {
    const { getTool, listTools } = await import("./tools");
    const fetchFn = vi.fn().mockResolvedValue({ tools: [] });
    await listTools(fetchFn);
    expect(fetchFn).toHaveBeenCalledWith("/metadata/v1/tools");
    fetchFn.mockResolvedValue({ apiName: "T", label: "T", layout: { mode: "sections" }, nodes: [] });
    await getTool(fetchFn, "T");
    expect(fetchFn).toHaveBeenCalledWith("/metadata/v1/tools/T");
  });
});
