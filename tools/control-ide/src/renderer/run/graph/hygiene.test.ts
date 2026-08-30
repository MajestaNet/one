import { describe, expect, it } from "vitest";
import { orphanDerivedRecordIds, tidyAttentionDocument } from "./hygiene";
import type { RunGraphDocument } from "./types";

describe("graph attention hygiene", () => {
  it("removes stale and unattended derived pins while preserving intentional and My day work", () => {
    const document: RunGraphDocument = {
      apiVersion: "one.runGraph/v1",
      id: "home",
      title: "My graph",
      nodes: [
        { id: "my-day", kind: "cluster", label: "My day" },
        { id: "accounts", kind: "collection", ref: { objectApiName: "Account" } },
        { id: "orphan", kind: "record", ref: { objectApiName: "Account", recordId: "orphan" } },
        { id: "watched", kind: "record", ref: { objectApiName: "Account", recordId: "watched" } },
        { id: "today", kind: "record", ref: { objectApiName: "Account", recordId: "today" } },
        { id: "manual", kind: "record", ref: { objectApiName: "Account", recordId: "manual" } },
        { id: "stale", kind: "insight", text: "Unavailable" },
      ],
      edges: [
        { id: "derive-orphan", from: "orphan", to: "accounts", kind: "derivedFrom" },
        { id: "derive-watched", from: "watched", to: "accounts", kind: "derivedFrom" },
        { id: "watch", from: "manual", to: "watched", kind: "watches" },
        { id: "derive-today", from: "today", to: "accounts", kind: "derivedFrom" },
        { id: "today-member", from: "my-day", to: "today", kind: "owns" },
        { id: "stale-edge", from: "stale", to: "manual", kind: "relates" },
      ],
    };

    expect([...orphanDerivedRecordIds(document)]).toEqual(["orphan"]);
    const result = tidyAttentionDocument(document, new Set(["stale"]));
    expect(result.removed).toBe(2);
    expect(result.document.nodes.map((node) => node.id)).not.toEqual(expect.arrayContaining(["orphan", "stale"]));
    expect(result.document.nodes.map((node) => node.id)).toEqual(expect.arrayContaining(["watched", "today", "manual"]));
    expect(result.document.edges.some((edge) => edge.from === "orphan" || edge.to === "stale")).toBe(false);
  });
});
