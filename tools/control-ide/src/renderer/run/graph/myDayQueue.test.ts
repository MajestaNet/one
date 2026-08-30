import { describe, expect, it } from "vitest";
import {
  markMyDayItemDoneEdges,
  myDayPromotionEndpoints,
  promoteMyDayItemEdges,
  rankMyDayQueue,
} from "./myDayQueue";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";

function queueGraph(): RunGraphDocument {
  return {
    apiVersion: RUN_GRAPH_API_VERSION,
    id: "home",
    title: "My graph",
    nodes: [
      { id: "my-day", kind: "cluster", label: "My day" },
      { id: "source", kind: "insight", text: "Morning review" },
      { id: "blocked", kind: "record", ref: { objectApiName: "Case", recordId: "case-1" } },
      { id: "next-low", kind: "record", ref: { objectApiName: "Task", recordId: "task-1" } },
      { id: "next-high", kind: "record", ref: { objectApiName: "Task", recordId: "task-2" } },
      { id: "watched", kind: "record", ref: { objectApiName: "Account", recordId: "account-1" } },
      { id: "member", kind: "question", text: "Review this" },
    ],
    edges: [
      { id: "member-edge", from: "my-day", to: "member", kind: "owns" },
      { id: "watch-edge", from: "source", to: "watched", kind: "watches" },
      { id: "next-low-edge", from: "source", to: "next-low", kind: "next", weight: 2 },
      { id: "next-high-edge", from: "source", to: "next-high", kind: "next", weight: 9 },
      { id: "blocked-edge", from: "source", to: "blocked", kind: "blocks" },
      { id: "blocked-next-edge", from: "my-day", to: "blocked", kind: "next", weight: 100 },
      { id: "unrelated", from: "next-low", to: "next-high", kind: "relates" },
    ],
  };
}

describe("rankMyDayQueue", () => {
  it("ranks blocks, weighted next, watches, then My day membership and deduplicates nodes", () => {
    const queue = rankMyDayQueue(queueGraph());

    expect(queue.map((item) => [item.node.id, item.reason, item.weight])).toEqual([
      ["blocked", "blocks", 100],
      ["next-high", "next", 9],
      ["next-low", "next", 2],
      ["watched", "watches", undefined],
      ["member", "my-day", undefined],
    ]);
    expect(queue[0]?.nextEdgeIds).toEqual(["blocked-next-edge"]);
  });

  it("treats work-edge targets as queue entries and ignores invalid next weights", () => {
    const graph = queueGraph();
    graph.edges.push({ id: "invalid-next", from: "source", to: "watched", kind: "next", weight: Number.NaN });

    const queue = rankMyDayQueue(graph);
    expect(queue.some((item) => item.node.id === "source")).toBe(false);
    expect(queue.find((item) => item.node.id === "watched")).toMatchObject({
      reason: "next",
      weight: undefined,
    });
  });

  it("marks done by removing inbound next/blocks/watches and My day membership edges", () => {
    const graph = queueGraph();
    graph.edges.push({ id: "blocked-member", from: "my-day", to: "blocked", kind: "owns" });

    const completed = markMyDayItemDoneEdges(graph, "blocked");

    expect(completed.map((edge) => edge.id)).not.toEqual(expect.arrayContaining([
      "blocked-next-edge",
      "blocked-member",
      "blocked-edge",
    ]));
    expect(completed).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "unrelated", kind: "relates" }),
      expect.objectContaining({ id: "watch-edge", kind: "watches" }),
    ]));
    expect(rankMyDayQueue({ ...graph, edges: completed }).some((item) => item.node.id === "blocked")).toBe(false);
    expect(graph.nodes).toHaveLength(7);
  });

  it("clears watches-only queue entries on mark done", () => {
    const graph = queueGraph();
    const completed = markMyDayItemDoneEdges(graph, "watched");
    expect(completed.some((edge) => edge.id === "watch-edge")).toBe(false);
    expect(rankMyDayQueue({ ...graph, edges: completed }).some((item) => item.node.id === "watched")).toBe(false);
  });

  it("promotes existing next work by weight and watched work by adding a next edge", () => {
    const graph = queueGraph();
    const queue = rankMyDayQueue(graph);
    const next = queue.find((item) => item.node.id === "next-low")!;
    const watched = queue.find((item) => item.node.id === "watched")!;

    expect(promoteMyDayItemEdges(graph, next).find((edge) => edge.id === "next-low-edge"))
      .toMatchObject({ weight: 101 });
    expect(myDayPromotionEndpoints(watched)).toEqual({ from: "source", to: "watched" });
    expect(promoteMyDayItemEdges(graph, watched, "promoted-watch").at(-1)).toEqual({
      id: "promoted-watch",
      from: "source",
      to: "watched",
      kind: "next",
      weight: 101,
    });
  });
});
