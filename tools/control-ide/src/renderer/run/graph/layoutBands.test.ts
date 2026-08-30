import { describe, expect, it } from "vitest";
import { bandFor, nextBandPosition, tidyPositions } from "./layoutBands";
import type { RunGraphNode } from "./types";

describe("graph layout bands", () => {
  it("keeps objects, Tools, and active work in stable horizontal bands", () => {
    const nodes: RunGraphNode[] = [
      ...Array.from({ length: 6 }, (_, index) => ({
        id: `collection-${index}`,
        kind: "collection" as const,
        label: `Object ${index}`,
      })),
      { id: "tool", kind: "tool", label: "Brief" },
      { id: "note", kind: "insight", text: "Follow up" },
    ];

    const positions = tidyPositions(nodes);
    expect(bandFor(nodes[0])).toBe("collections");
    expect(bandFor(nodes[6])).toBe("tools");
    expect(bandFor(nodes[7])).toBe("working");
    expect(positions.tool.y).toBeGreaterThan(positions["collection-5"].y);
    expect(positions.note.y).toBeGreaterThan(positions.tool.y);
    expect(tidyPositions(nodes)).toEqual(positions);
  });

  it("places a new node in its own band without overlapping existing nodes", () => {
    const nodes: RunGraphNode[] = [
      { id: "note-1", kind: "insight" },
      { id: "note-2", kind: "question" },
    ];
    const next = nextBandPosition({ nodes }, "insight");
    expect(Object.values(tidyPositions(nodes))).not.toContainEqual(next);
  });
});
