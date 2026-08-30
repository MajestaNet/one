import { describe, expect, it } from "vitest";
import { demoAccountsTool, demoPipelineTool } from "./fixtures";
import { TOOL_DOCUMENT_API_VERSION } from "./types";
import { validateToolDocument } from "./validate";

describe("validateToolDocument", () => {
  it("accepts the accounts fixture", () => {
    const result = validateToolDocument(demoAccountsTool());
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.document.apiVersion).toBe(TOOL_DOCUMENT_API_VERSION);
      expect(result.document.nodes.length).toBeGreaterThan(0);
    }
  });

  it("accepts the spatial pipeline fixture", () => {
    const result = validateToolDocument(demoPipelineTool());
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.document.layout.mode).toBe("spatial");
    }
  });

  it("rejects unknown node kinds (ADR-021)", () => {
    const bad = {
      ...demoAccountsTool(),
      nodes: [
        {
          id: "evil",
          kind: "rawHtml",
          props: { html: "<script>alert(1)</script>" },
        },
      ],
    };
    const result = validateToolDocument(bad);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.issues.some((i) => i.path.includes("kind") && /unknown/i.test(i.message))).toBe(
        true,
      );
    }
  });

  it("rejects wrong apiVersion", () => {
    const result = validateToolDocument({ ...demoAccountsTool(), apiVersion: "nope" });
    expect(result.ok).toBe(false);
  });

  it("accepts layout.kind as alias for layout.mode", () => {
    const doc = demoAccountsTool();
    const raw = {
      ...doc,
      layout: { kind: "sections", sections: doc.layout.sections },
    };
    const result = validateToolDocument(raw);
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.document.layout.mode).toBe("sections");
  });
});
