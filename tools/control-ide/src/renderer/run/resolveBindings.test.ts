import { describe, expect, it, vi } from "vitest";
import {
  applyBindingToNode,
  resolveToolDocumentBindings,
  stripBakedRecordPayloads,
} from "./resolveBindings";
import type { ToolDocument, ToolNode } from "./types";
import { TOOL_DOCUMENT_API_VERSION } from "./types";

function baseDoc(nodes: ToolNode[], bindings: ToolDocument["dataBindings"] = []): ToolDocument {
  return {
    apiVersion: TOOL_DOCUMENT_API_VERSION,
    id: "T",
    title: "T",
    layout: { mode: "sections", sections: [{ id: "s", nodeIds: nodes.map((n) => n.id) }] },
    dataBindings: bindings,
    nodes,
  };
}

describe("resolveBindings", () => {
  it("strips baked rows/fields from record-bearing nodes", () => {
    const node: ToolNode = {
      id: "t",
      kind: "recordTable",
      bindingId: "b1",
      props: { rows: [{ Name: "Secret" }], columns: [{ key: "Name" }] },
    };
    const stripped = stripBakedRecordPayloads(node);
    expect(stripped.props.rows).toBeUndefined();
    expect(stripped.props.columns).toEqual([{ key: "Name" }]);
  });

  it("sanitizeToolDocumentForMetadata drops live payloads before Metadata promote", async () => {
    const { sanitizeToolDocumentForMetadata } = await import("./resolveBindings");
    const doc = baseDoc(
      [
        {
          id: "table",
          kind: "recordTable",
          bindingId: "b1",
          props: { columns: [{ key: "Name" }], rows: [{ Name: "LEAK" }] },
        },
        {
          id: "note",
          kind: "markdownNote",
          props: { text: "safe" },
        },
      ],
      [{ id: "b1", objectApiName: "Account", query: { limit: 10 } }],
    );
    const durable = sanitizeToolDocumentForMetadata(doc);
    expect(durable.nodes[0].props.rows).toBeUndefined();
    expect(durable.nodes[0].props.columns).toEqual([{ key: "Name" }]);
    expect(durable.nodes[1].props.text).toBe("safe");
    expect(durable.dataBindings?.[0].objectApiName).toBe("Account");
  });

  it("applies Client query results to bound nodes only", async () => {
    const fetchFn = vi.fn().mockResolvedValue({
      records: [{ id: "a1", Name: "Acme" }],
    });
    const doc = baseDoc(
      [
        {
          id: "table",
          kind: "recordTable",
          bindingId: "b1",
          props: { rows: [{ Name: "BAKED_SHOULD_NOT_SHOW" }] },
        },
        {
          id: "other",
          kind: "recordTable",
          props: { rows: [{ Name: "also-baked" }] },
        },
      ],
      [{ id: "b1", objectApiName: "Account", query: { limit: 10 } }],
    );
    const { document, errors } = await resolveToolDocumentBindings(doc, fetchFn);
    expect(errors).toEqual([]);
    expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/query",
      expect.objectContaining({ method: "POST" }),
    );
    const table = document.nodes.find((n) => n.id === "table")!;
    expect(table.props.rows).toEqual([expect.objectContaining({ Name: "Acme", id: "a1" })]);
    const other = document.nodes.find((n) => n.id === "other")!;
    expect(other.props.rows).toBeUndefined();
  });

  it("leaves empty nodes when a binding query fails (no baked fallback)", async () => {
    const fetchFn = vi.fn().mockRejectedValue(new Error("forbidden"));
    const doc = baseDoc(
      [
        {
          id: "table",
          kind: "recordTable",
          bindingId: "b1",
          props: { rows: [{ Name: "BAKED" }] },
        },
      ],
      [{ id: "b1", objectApiName: "Account", query: {} }],
    );
    const { document, errors } = await resolveToolDocumentBindings(doc, fetchFn);
    expect(errors[0]).toMatch(/forbidden/);
    expect(document.nodes[0].props.rows).toBeUndefined();
  });

  it("applyBindingToNode fills recordCard from first row", () => {
    const node: ToolNode = { id: "c", kind: "recordCard", props: {} };
    const next = applyBindingToNode(node, [{ id: "1", Name: "X" }], "Account");
    expect(next.props.fields).toEqual({ id: "1", Name: "X" });
    expect(next.props.recordId).toBe("1");
  });
});
