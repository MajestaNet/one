import { afterEach, describe, expect, it } from "vitest";
import { toolCreate } from "./agentTools";
import { demoAccountsTool } from "./fixtures";
import { buildActiveToolContext, summarizeToolDocument } from "./activeToolContext";

afterEach(() => {
  localStorage.clear();
});

describe("activeToolContext", () => {
  it("summarizeToolDocument maps bindings and nodes", () => {
    const doc = demoAccountsTool();
    const summary = summarizeToolDocument(doc);
    expect(summary.toolId).toBe(doc.id);
    expect(summary.title).toBe(doc.title);
    expect(summary.dataBindings?.length).toBeGreaterThan(0);
    expect(summary.nodesSummary.some((n) => n.kind === "stat")).toBe(true);
  });

  it("buildActiveToolContext returns null without id", () => {
    expect(buildActiveToolContext(null)).toBeNull();
    expect(buildActiveToolContext(undefined)).toBeNull();
    expect(buildActiveToolContext("missing-tool")).toBeNull();
  });

  it("buildActiveToolContext loads from tool store", () => {
    toolCreate({ document: demoAccountsTool() });
    const ctx = buildActiveToolContext("demo-top-accounts");
    expect(ctx?.toolId).toBe("demo-top-accounts");
    expect(ctx?.title).toMatch(/account/i);
  });
});
