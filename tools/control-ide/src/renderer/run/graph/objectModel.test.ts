import { describe, expect, it } from "vitest";
import type { DescribeObject } from "../../operate/types";
import type { GlobalDescribeObject } from "../../operate/describeCache";
import { mergeAccessibleObjectModel, MODEL_EDGE_PREFIX } from "./objectModel";
import type { RunGraphDocument } from "./types";

const catalog: GlobalDescribeObject[] = [
  { apiName: "Account", label: "Account", pluralLabel: "Accounts" },
  { apiName: "Contact", label: "Contact", pluralLabel: "Contacts" },
];

describe("accessible object graph model", () => {
  it("adds every accessible object and derives reference relationships without replacing user topology", () => {
    const document: RunGraphDocument = {
      apiVersion: "one.runGraph/v1",
      id: "home",
      title: "My graph",
      nodes: [
        { id: "accounts", kind: "collection", label: "Accounts", ref: { objectApiName: "Account" } },
        { id: "note", kind: "insight", text: "Protect this edge" },
      ],
      edges: [{ id: "user-edge", from: "note", to: "accounts", kind: "watches" }],
    };
    const describes = new Map<string, DescribeObject>([
      ["Account", { apiName: "Account", fields: [{ apiName: "PrimaryContactId", referenceTo: "Contact" }] }],
      ["Contact", { apiName: "Contact", fields: [
        { apiName: "AccountId", referenceTo: "Account" },
        { apiName: "ManagerId", referenceTo: "Contact" },
      ] }],
    ]);

    const merged = mergeAccessibleObjectModel(document, catalog, describes);
    expect(merged.addedObjects).toBe(1);
    expect(merged.relationshipCount).toBe(2);
    expect(merged.document.nodes.filter((node) => node.kind === "collection")).toHaveLength(2);
    expect(merged.document.nodes.every((node) => node.layout)).toBe(true);
    expect(merged.document.edges).toEqual(expect.arrayContaining([
      document.edges[0],
      expect.objectContaining({ id: expect.stringMatching(/^model:/), kind: "relates" }),
    ]));
    expect(merged.document.edges.filter((edge) => edge.id.startsWith(MODEL_EDGE_PREFIX))).toHaveLength(2);

    const unchanged = mergeAccessibleObjectModel(merged.document, catalog, describes);
    expect(unchanged.addedObjects).toBe(0);
    expect(unchanged.changed).toBe(false);
  });
});
