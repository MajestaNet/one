import { describe, expect, it, vi } from "vitest";
import { enrichRunOutputFromSummary } from "./agentEffectsFromSummary";
import { processRunToolEffects } from "./runToolEffects";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument, type RunGraphEnvelope } from "./graph/types";
import { ProposalStagingStore } from "./graph/proposalStaging";

describe("enrichRunOutputFromSummary", () => {
  it("promotes oneEffects fence into structured keys", () => {
    const enriched = enrichRunOutputFromSummary({
      summary: `Pinned.\n\n\`\`\`oneEffects\n${JSON.stringify({
        graphCalls: [{ tool: "graph.pin", input: { ref: { objectApiName: "Account", recordId: "a1" } } }],
        proposedMutations: [{ op: "update", object: "Account", id: "a1", data: { Name: "Acme" } }],
      })}\n\`\`\``,
    });
    expect(enriched?.summary).toBe("Pinned.");
    expect(enriched?.graphCalls).toEqual([
      { tool: "graph.pin", input: { ref: { objectApiName: "Account", recordId: "a1" } } },
    ]);
    expect(enriched?.proposedMutations).toHaveLength(1);
  });

  it("does not clobber existing structured keys", () => {
    const enriched = enrichRunOutputFromSummary({
      summary: "```json\n" + JSON.stringify({ graphCalls: [{ tool: "graph.link" }] }) + "\n```",
      graphCalls: [{ tool: "graph.get" }],
    });
    expect(enriched?.graphCalls).toEqual([{ tool: "graph.get" }]);
  });
});

describe("processRunToolEffects summary bridge", () => {
  it("stages proposals and applies graphCalls parsed from summary text", async () => {
    let envelope: RunGraphEnvelope = {
      id: "graph-row-1",
      graphKey: "home",
      title: "My graph",
      revision: 1,
      document: {
        apiVersion: RUN_GRAPH_API_VERSION,
        id: "home",
        title: "My graph",
        nodes: [],
        edges: [],
      },
    };
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/run-graphs/home" && !init?.method) return envelope;
      if (path === "/client/v1/run-graphs/home" && init?.method === "PUT") {
        const document = JSON.parse(String(init.body)) as RunGraphDocument;
        envelope = { ...envelope, revision: envelope.revision + 1, document };
        return envelope;
      }
      throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
    });
    const store = new ProposalStagingStore();

    const effects = await processRunToolEffects(
      {
        id: "run-effects",
        status: "completed",
        output: {
          summary: [
            "Staged an update and annotated the graph.",
            "```oneEffects",
            JSON.stringify({
              graphCalls: [{ tool: "graph.annotate", input: { text: "Follow up", kind: "insight" } }],
              proposedMutations: [
                { op: "update", object: "Account", id: "a-9", data: { Industry: "Tech" } },
              ],
            }),
            "```",
          ].join("\n"),
        },
      },
      {
        fetch: fetchFn,
        mode: "operate",
        synthesizeWhenPrompted: false,
        proposalStore: store,
      },
    );

    expect(effects.proposalId).toBe("proposal-run-effects");
    expect(store.get("proposal-run-effects")?.mutations).toHaveLength(1);
    expect(effects.graphChanged).toBe(true);
    expect(effects.graphResults?.some((row) => row.tool === "graph.annotate" && row.result.ok)).toBe(true);
    expect(envelope.document.nodes.some((node) => node.kind === "proposal")).toBe(true);
    expect(envelope.document.nodes.some((node) => node.kind === "insight")).toBe(true);
  });
});
