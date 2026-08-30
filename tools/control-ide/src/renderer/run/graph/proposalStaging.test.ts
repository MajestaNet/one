import { describe, expect, it, vi } from "vitest";
import {
  applyProposalMutations,
  proposalInputFromRunOutput,
  ProposalStagingStore,
  stageProposalFromRunOutput,
} from "./proposalStaging";
import { processRunToolEffects } from "../runToolEffects";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";

function hasForbiddenGraphKey(value: unknown): boolean {
  if (Array.isArray(value)) return value.some(hasForbiddenGraphKey);
  if (!value || typeof value !== "object") return false;
  return Object.entries(value as Record<string, unknown>).some(
    ([key, child]) =>
      ["operations", "data", "rows", "fields", "mutations"].includes(key) ||
      hasForbiddenGraphKey(child),
  );
}

describe("ProposalStagingStore", () => {
  it("keeps mutation payloads in a session map and returns defensive copies", () => {
    const store = new ProposalStagingStore();
    const staged = store.stage({
      proposalId: "p-1",
      runId: "run-1",
      mutations: [{ op: "update", object: "Account", id: "a-1", data: { Name: "Acme" } }],
      createdAt: 42,
    });

    staged.mutations[0]!.data!.Name = "Changed";
    expect(store.get("p-1")).toMatchObject({
      proposalId: "p-1",
      runId: "run-1",
      createdAt: 42,
      mutations: [{ op: "update", object: "Account", id: "a-1", data: { Name: "Acme" } }],
    });
    expect(store.size).toBe(1);
    expect(store.drop("p-1")).toBe(true);
    expect(store.size).toBe(0);
  });
});

describe("proposal graph staging", () => {
  it("extracts proposal evidence and PUTs only reference topology", async () => {
    let document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    };
    let revision = 1;
    const putBodies: unknown[] = [];
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PUT") {
        putBodies.push(JSON.parse(String(init.body)));
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      }
      return { id: "home", graphKey: "home", title: "My graph", document, revision };
    });
    const output = {
      summary: "Update the renewal owner",
      proposal: {
        proposalId: "proposal-42",
        mutations: [
          { op: "update", object: "Opportunity", id: "opp-1", data: { OwnerId: "user-2" } },
        ],
      },
    };
    expect(proposalInputFromRunOutput("run-42", output)).toMatchObject({
      proposalId: "proposal-42",
      runId: "run-42",
    });

    const store = new ProposalStagingStore();
    const staged = await stageProposalFromRunOutput(fetchFn, store, "run-42", output);

    expect(staged?.proposalId).toBe("proposal-42");
    expect(store.get("proposal-42")?.mutations[0]?.data).toEqual({ OwnerId: "user-2" });
    expect(putBodies).toHaveLength(1);
    expect(hasForbiddenGraphKey(putBodies[0])).toBe(false);
    const proposalNode = document.nodes.find((node) => node.kind === "proposal");
    expect(proposalNode).toEqual({
      id: expect.any(String),
      kind: "proposal",
      proposalId: "proposal-42",
    });
    expect(document.edges).toEqual([
      expect.objectContaining({ kind: "explains", to: proposalNode?.id }),
    ]);
  });

  it("applies create, update, and delete through Client paths", async () => {
    const fetchFn = vi.fn().mockResolvedValue({});
    const result = await applyProposalMutations(fetchFn, {
      proposalId: "p-1",
      createdAt: 1,
      mutations: [
        { op: "create", object: "Task", data: { Subject: "Call" } },
        { op: "update", object: "Account", id: "a/1", data: { Name: "Acme" } },
        { op: "delete", object: "Contact", id: "c-1" },
      ],
    });

    expect(result).toMatchObject({ complete: true, appliedCount: 3, total: 3, appliedThrough: 3 });
    expect(fetchFn.mock.calls).toEqual([
      ["/client/v1/sobjects/Task", { method: "POST", body: JSON.stringify({ Subject: "Call" }) }],
      ["/client/v1/sobjects/Account/a%2F1", { method: "PATCH", body: JSON.stringify({ Name: "Acme" }) }],
      ["/client/v1/sobjects/Contact/c-1", { method: "DELETE" }],
    ]);
  });

  it("records partial Client failure and resumes from appliedThrough", async () => {
    const store = new ProposalStagingStore();
    store.stage({
      proposalId: "p-partial",
      mutations: [
        { op: "create", object: "Task", data: { Subject: "One" } },
        { op: "update", object: "Account", id: "a-1", data: { Name: "Two" } },
        { op: "delete", object: "Contact", id: "c-1" },
      ],
    });
    const fetchFn = vi.fn(async (path: string) => {
      if (path.includes("/Account/")) throw new Error("AuthZ denied");
      return {};
    });
    const staging = store.get("p-partial")!;
    await expect(applyProposalMutations(fetchFn, staging, store)).rejects.toMatchObject({
      name: "ProposalApplyPartialError",
      result: { appliedCount: 1, total: 3, complete: false, appliedThrough: 1 },
    });
    expect(store.get("p-partial")?.appliedThrough).toBe(1);
    expect(fetchFn).toHaveBeenCalledTimes(2);

    const resumeFetch = vi.fn().mockResolvedValue({});
    const resumed = await applyProposalMutations(resumeFetch, store.get("p-partial")!, store);
    expect(resumed).toMatchObject({ complete: true, appliedCount: 3, total: 3 });
    expect(resumeFetch.mock.calls).toEqual([
      ["/client/v1/sobjects/Account/a-1", { method: "PATCH", body: JSON.stringify({ Name: "Two" }) }],
      ["/client/v1/sobjects/Contact/c-1", { method: "DELETE" }],
    ]);
  });

  it("validates the entire proposal before the first Client mutation", async () => {
    const fetchFn = vi.fn();
    await expect(applyProposalMutations(fetchFn, {
      proposalId: "p-invalid",
      createdAt: 1,
      mutations: [
        { op: "create", object: "Task", data: { Subject: "Would otherwise commit" } },
        { op: "update", object: "Account", data: { Name: "Missing id" } },
      ],
    })).rejects.toThrow(/id is required for update/);
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("stages completed Run output and reports a visible graph change", async () => {
    let document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [],
      edges: [],
    };
    let revision = 1;
    const fetchFn = vi.fn(async (_path: string, init?: RequestInit) => {
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      }
      return { id: "home", graphKey: "home", title: "My graph", document, revision };
    });
    const store = new ProposalStagingStore();

    const effects = await processRunToolEffects({
      id: "run-7",
      status: "completed",
      output: {
        proposedMutations: [
          { op: "update", object: "Account", id: "a-7", data: { Industry: "Tech" } },
        ],
      },
    }, {
      fetch: fetchFn,
      mode: "operate",
      synthesizeWhenPrompted: false,
      proposalStore: store,
    });

    expect(effects).toMatchObject({ graphChanged: true, proposalId: "proposal-run-7" });
    expect(store.get("proposal-run-7")?.mutations).toHaveLength(1);
    expect(document.nodes).toEqual([
      expect.objectContaining({ kind: "proposal", proposalId: "proposal-run-7" }),
    ]);
  });
});
