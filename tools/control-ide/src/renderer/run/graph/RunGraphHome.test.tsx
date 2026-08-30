import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RunGraphHome } from "./RunGraphHome";
import { ProposalStagingStore } from "./proposalStaging";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./types";
import { describeCache } from "../../operate/describeCache";

vi.mock("./RunGraphView", () => ({
  RunGraphView: (props: {
    view: { nodes: Array<{ id: string }>; edges: Array<{ id: string }> };
    onViewportChange?: (viewport: { x: number; y: number; zoom: number }) => void;
    onSelectedNodeIdsChange?: (nodeIds: string[]) => void;
    onSelectedEdgeIdChange?: (edgeId: string | null) => void;
    onStartConnect?: (nodeId: string) => void;
    onConnectNodes?: (sourceId: string, targetId: string) => void;
  }) => (
    <div data-testid="mock-graph-view">
      {props.view.nodes.map((node) => <span key={node.id}>{node.id}</span>)}
      <button onClick={() => props.onViewportChange?.({ x: 12, y: 24, zoom: 0.8 })}>Move graph</button>
      <button onClick={() => props.onSelectedNodeIdsChange?.(props.view.nodes[0] ? [props.view.nodes[0].id] : [])}>Select first</button>
      <button onClick={() => props.view.nodes[0] && props.onStartConnect?.(props.view.nodes[0].id)}>Start connect</button>
      <button onClick={() => {
        if (props.view.nodes[0] && props.view.nodes[1]) {
          props.onConnectNodes?.(props.view.nodes[0].id, props.view.nodes[1].id);
        }
      }}>Connect nodes</button>
      <button onClick={() => props.onSelectedEdgeIdChange?.(props.view.edges[0]?.id ?? null)}>Select first wire</button>
    </div>
  ),
}));

function describeCatalog() {
  return { objects: [{ apiName: "Account", label: "Account" }, { apiName: "Contact", label: "Contact" }] };
}

function envelope(document: RunGraphDocument, revision = 1) {
  return { id: "row-1", graphKey: "home", title: document.title, document, revision };
}

const empty: RunGraphDocument = {
  apiVersion: RUN_GRAPH_API_VERSION, id: "home", title: "My graph", nodes: [], edges: [],
};

afterEach(() => {
  cleanup();
  describeCache.clear();
});

describe("RunGraphHome", () => {
  it("shows the loading and empty states with the List View CTA", async () => {
    let release: ((value: unknown) => void) | undefined;
    const fetchFn = vi.fn((path: string) => {
      if (path === "/client/v1/describe") return Promise.resolve({ objects: [] });
      return new Promise((resolve) => { release = resolve; });
    });
    const open = vi.fn();
    render(<RunGraphHome fetchFn={fetchFn} onOpenObjectHome={open} />);
    expect(screen.getByTestId("run-graph-loading")).toBeTruthy();
    release?.(envelope(empty));
    await screen.findByText("Your graph is ready");
    fireEvent.click(screen.getByTestId("run-graph-open-list-view"));
    expect(open).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("run-graph-empty-add"));
    expect(screen.getByTestId("run-graph-composer")).toBeTruthy();
  });

  it("adds a note and mounts an accessible Tool directly from My graph", async () => {
    let document: RunGraphDocument = { ...empty, nodes: [], edges: [] };
    let revision = 1;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") return describeCatalog();
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      } else if (init?.method) {
        throw new Error(`unexpected ${init.method}`);
      }
      return envelope(document, revision);
    });

    const props = {
      fetchFn,
      mountableTools: [
        {
          id: "tool:AccountBrief",
          kind: "published" as const,
          label: "Account brief",
          toolSpecApiName: "AccountBrief",
        },
      ],
    };
    const { rerender } = render(
      <RunGraphHome
        {...props}
      />,
    );
    await screen.findByText("Your graph is ready");

    fireEvent.click(screen.getByText("+ Add"));
    fireEvent.change(screen.getByLabelText("Note text"), { target: { value: "Review renewals today" } });
    fireEvent.click(screen.getByText("Add note"));
    await screen.findByText("Note added to My graph.");
    expect(document.nodes).toEqual(expect.arrayContaining([
      expect.objectContaining({ kind: "insight", text: "Review renewals today" }),
    ]));

    rerender(<RunGraphHome {...props} mountRequest={{ toolId: "tool:AccountBrief", epoch: 1 }} />);
    await screen.findByText("Account brief added to your graph.");
    expect(document.nodes).toEqual(expect.arrayContaining([
      expect.objectContaining({ kind: "tool", toolRef: { toolSpecApiName: "AccountBrief" } }),
    ]));

    const writes = fetchFn.mock.calls.filter(([, init]) => init?.method === "PUT");
    expect(writes).toHaveLength(2);
    expect(writes[0]?.[1]?.headers).toEqual({ "If-Match": '"1"' });
    expect(writes[1]?.[1]?.headers).toEqual({ "If-Match": '"2"' });
  });

  it("adds an object collection from My graph and opens its list in focus", async () => {
    let document: RunGraphDocument = { ...empty, nodes: [], edges: [] };
    let revision = 1;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") {
        return { objects: [{ apiName: "Account", label: "Account" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return { apiName: "Account", fields: [{ apiName: "Name", label: "Name", fieldType: "text" }] };
      }
      if (path === "/client/v1/query") {
        return { records: [{ id: "00000000-0000-4000-8000-000000000111", Name: "Acme" }] };
      }
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      } else if (init?.method) {
        throw new Error(`unexpected ${init.method}`);
      }
      return envelope(document, revision);
    });

    render(
      <RunGraphHome
        fetchFn={fetchFn}
        bridge={{
          session: { baseUrl: "http://localhost:8080", token: "t", scopes: ["client"], activeInstallId: "inst-col" },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
      />,
    );
    await screen.findByText("Your graph is ready");
    await screen.findByText("Graph ready with 1 accessible object.");
    expect(document.nodes).toEqual(expect.arrayContaining([
      expect.objectContaining({ kind: "collection", ref: { objectApiName: "Account" } }),
    ]));
    fireEvent.click(screen.getByText("Select first"));
    expect(await screen.findByTestId("run-graph-collection-focus")).toBeTruthy();
  });

  it("lands a search hit in its collection without pinning another record", async () => {
    let document: RunGraphDocument = { ...empty, nodes: [], edges: [] };
    let revision = 1;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") return { objects: [{ apiName: "Account", label: "Account", pluralLabel: "Accounts" }] };
      if (path === "/client/v1/search" && init?.method === "POST") {
        return { hits: [{ id: "account-1", object: "Account", title: "Acme SearchCo" }] };
      }
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      }
      return envelope(document, revision);
    });

    render(<RunGraphHome fetchFn={fetchFn} />);
    await screen.findByText("Your graph is ready");
    fireEvent.change(screen.getByTestId("operate-global-search"), { target: { value: "acme" } });
    fireEvent.click(await screen.findByText("Acme SearchCo"));
    await screen.findByText("Opened Account in your graph.");

    expect(document.nodes).toEqual([
      expect.objectContaining({ kind: "collection", ref: { objectApiName: "Account" } }),
    ]);
    expect(document.nodes.some((node) => node.kind === "record")).toBe(false);
    expect(screen.getByTestId("run-graph-focus")).toBeTruthy();
  });

  it("opens mounted Tools and persists viewport with the loaded revision", async () => {
    let document: RunGraphDocument = {
      ...empty,
      nodes: [{ id: "tool-1", kind: "tool", toolRef: { toolSpecApiName: "AccountBrief" } }],
    };
    let revision = 4;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (!init?.method) return envelope(document, revision);
      if (init.method === "PATCH") {
        document = { ...document, ...(JSON.parse(String(init.body)) as Partial<RunGraphDocument>) };
        revision += 1;
        return envelope(document, revision);
      }
      if (path === "/client/v1/describe") return describeCatalog();
      throw new Error(`unexpected ${path}`);
    });
    const open = vi.fn();
    render(<RunGraphHome fetchFn={fetchFn} onOpenTool={open} />);
    await screen.findByTestId("mock-graph-view");
    fireEvent.click(screen.getByText("Select first"));
    fireEvent.click(screen.getByText("Open as board"));
    expect(open).toHaveBeenCalledWith(expect.objectContaining({ id: "tool-1" }));
    fireEvent.click(screen.getByText("Move graph"));
    await waitFor(() => expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/run-graphs/home",
      expect.objectContaining({ method: "PATCH", headers: { "If-Match": '"4"' } }),
    ));
  });

  it("opens node focus from selection and connects nodes through graph.link", async () => {
    let document: RunGraphDocument = {
      ...empty,
      nodes: [
        { id: "insight-1", kind: "insight", text: "Review renewal risk" },
        { id: "question-1", kind: "question", text: "Who owns the follow-up?" },
      ],
      edges: [],
    };
    let revision = 1;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") return describeCatalog();
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PUT") {
        document = JSON.parse(String(init.body)) as RunGraphDocument;
        revision += 1;
      } else if (init?.method) {
        throw new Error(`unexpected ${init.method}`);
      }
      return envelope(document, revision);
    });

    render(<RunGraphHome fetchFn={fetchFn} />);
    await screen.findByTestId("mock-graph-view");
    fireEvent.click(screen.getByText("Select first"));
    expect(screen.getByTestId("run-graph-focus")).toBeTruthy();
    expect(screen.getByDisplayValue("Review renewal risk")).toBeTruthy();

    fireEvent.click(screen.getByText("Start connect"));
    fireEvent.change(screen.getByLabelText("Connection type"), { target: { value: "watches" } });
    fireEvent.click(screen.getByText("Connect nodes"));
    await waitFor(() => expect(document.edges).toEqual([
      expect.objectContaining({ from: "insight-1", to: "question-1", kind: "watches" }),
    ]));
    expect(fetchFn.mock.calls.some(([, init]) => {
      if (init?.method !== "PUT") return false;
      const saved = JSON.parse(String(init.body)) as RunGraphDocument;
      return saved.edges.some((edge) => edge.kind === "watches");
    })).toBe(true);

    await screen.findByTestId("run-graph-edge-focus");
    fireEvent.click(screen.getByText("Remove connection"));
    await waitFor(() => expect(document.edges).toHaveLength(0));
  });

  it("uses My day queue actions to select focus and mutate topology only", async () => {
    const originalNodes: RunGraphDocument["nodes"] = [
      { id: "my-day", kind: "cluster", label: "My day" },
      { id: "source", kind: "insight", text: "Morning review" },
      { id: "follow-up", kind: "question", text: "Send recap" },
      { id: "watched", kind: "insight", text: "Watch renewal" },
    ];
    let document: RunGraphDocument = {
      ...empty,
      nodes: originalNodes,
      edges: [
        { id: "next-1", from: "source", to: "follow-up", kind: "next", weight: 8 },
        { id: "member-1", from: "my-day", to: "follow-up", kind: "owns" },
        { id: "watch-1", from: "source", to: "watched", kind: "watches" },
      ],
    };
    let revision = 1;
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") return describeCatalog();
      if (path !== "/client/v1/run-graphs/home") throw new Error(`unexpected ${path}`);
      if (init?.method === "PATCH") {
        const patch = JSON.parse(String(init.body)) as Partial<RunGraphDocument>;
        document = { ...document, ...patch };
        revision += 1;
      } else if (init?.method) {
        throw new Error(`unexpected ${init.method}`);
      }
      return envelope(document, revision);
    });

    render(<RunGraphHome fetchFn={fetchFn} />);
    await screen.findByTestId("mock-graph-view");
    fireEvent.change(screen.getByLabelText("Graph view"), { target: { value: "my-day" } });

    expect(screen.getByTestId("run-graph-my-day-queue")).toBeTruthy();
    expect(screen.getByText("weight 8", { exact: false })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Open Send recap focus" }));
    expect(screen.getByDisplayValue("Send recap")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Mark Send recap done" }));
    await waitFor(() => expect(document.edges).toEqual([
      { id: "watch-1", from: "source", to: "watched", kind: "watches" },
    ]));
    expect(document.nodes).toEqual(originalNodes);

    fireEvent.click(screen.getByRole("button", { name: "Promote Watch renewal next" }));
    await waitFor(() => expect(document.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({ from: "source", to: "watched", kind: "watches" }),
      expect.objectContaining({ from: "source", to: "watched", kind: "next", weight: 1 }),
    ])));
    expect(document.nodes).toEqual(originalNodes);
    const queuePatches = fetchFn.mock.calls
      .filter(([, init]) => init?.method === "PATCH")
      .map(([, init]) => JSON.parse(String(init?.body)) as Record<string, unknown>);
    expect(queuePatches).toHaveLength(2);
    expect(queuePatches.every((patch) => Object.keys(patch).join(",") === "edges")).toBe(true);
  });

  it("offers Object Home and curator actions when My day has no topology", async () => {
    const fetchFn = vi.fn().mockResolvedValue(envelope(empty));
    const openObjectHome = vi.fn();
    const askRunAgent = vi.fn();

    render(
      <RunGraphHome
        fetchFn={fetchFn}
        onOpenObjectHome={openObjectHome}
        onAskRunAgent={askRunAgent}
      />,
    );
    await screen.findByText("Your graph is ready");
    fireEvent.change(screen.getByLabelText("Graph view"), { target: { value: "my-day" } });
    fireEvent.click(screen.getByTestId("my-day-rebuild"));
    fireEvent.click(screen.getByTestId("my-day-open-object-home"));
    fireEvent.click(screen.getByTestId("my-day-ask-curator"));

    expect(openObjectHome).toHaveBeenCalledOnce();
    expect(askRunAgent).toHaveBeenCalledTimes(2);
    expect(askRunAgent).toHaveBeenCalledWith(expect.stringMatching(/curator.*graph\.get.*My day cluster.*blocks, next, and watches.*graph\.\*/i));
  });

  it("applies a proposal then removes its graph pin and reports Applied", async () => {
    let document: RunGraphDocument = {
      ...empty,
      nodes: [{ id: "proposal-node", kind: "proposal", proposalId: "proposal-1" }],
      edges: [],
    };
    let revision = 1;
    const store = new ProposalStagingStore();
    store.stage({
      proposalId: "proposal-1",
      mutations: [{ op: "update", object: "Account", id: "a-1", data: { Name: "Acme" } }],
    });
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/run-graphs/home") {
        if (init?.method === "PATCH") {
          document = { ...document, ...(JSON.parse(String(init.body)) as Partial<RunGraphDocument>) };
          revision += 1;
        }
        return envelope(document, revision);
      }
      if (path === "/client/v1/sobjects/Account/a-1" && init?.method === "PATCH") return {};
      if (path === "/client/v1/describe") return describeCatalog();
      throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
    });
    const onProposalResolved = vi.fn();

    render(
      <RunGraphHome
        fetchFn={fetchFn}
        proposalStore={store}
        onProposalResolved={onProposalResolved}
      />,
    );
    await screen.findByTestId("mock-graph-view");
    fireEvent.click(screen.getByText("Select first"));
    fireEvent.click(screen.getByText("Approve and apply"));

    await waitFor(() => expect(document.nodes).toHaveLength(0));
    expect(store.size).toBe(0);
    expect(onProposalResolved).toHaveBeenCalledWith("applied", 0);
    const graphPatch = fetchFn.mock.calls.find(
      ([path, init]) => path === "/client/v1/run-graphs/home" && init?.method === "PATCH",
    );
    expect(JSON.parse(String(graphPatch?.[1]?.body))).toEqual({ nodes: [], edges: [] });
  });

  it("executes visible signal bindings and pins survivors without persisting rows", async () => {
    let document: RunGraphDocument = {
      ...empty,
      nodes: [{ id: "signal-1", kind: "signal", bindingId: "renewals" }],
      edges: [],
      dataBindings: [{
        id: "renewals",
        objectApiName: "Opportunity",
        fields: ["Name"],
        filters: [{ field: "IsClosed", op: "eq", value: false }],
        limit: 5,
      }],
    };
    let revision = 1;
    const putBodies: RunGraphDocument[] = [];
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/query" && init?.method === "POST") {
        return {
          records: [{
            id: "00000000-0000-4000-8000-000000000111",
            Name: "Acme renewal",
          }],
        };
      }
      if (path === "/client/v1/run-graphs/home") {
        if (init?.method === "PUT") {
          document = JSON.parse(String(init.body)) as RunGraphDocument;
          putBodies.push(document);
          revision += 1;
        }
        return envelope(document, revision);
      }
      if (path === "/client/v1/describe") return describeCatalog();
      throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
    });

    render(<RunGraphHome fetchFn={fetchFn} />);
    await waitFor(() => expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/query",
      expect.objectContaining({ method: "POST" }),
    ));
    fireEvent.click(screen.getByText("Select first"));
    expect(await screen.findByText("Acme renewal")).toBeTruthy();
    fireEvent.click(screen.getByText("Pin survivors"));

    await waitFor(() => expect(document.nodes).toHaveLength(2));
    expect(document.nodes[1]).toMatchObject({
      kind: "record",
      ref: {
        objectApiName: "Opportunity",
        recordId: "00000000-0000-4000-8000-000000000111",
      },
    });
    expect(putBodies).toHaveLength(1);
    expect(JSON.stringify(putBodies[0])).not.toContain("Acme renewal");
    expect((putBodies[0] as unknown as { rows?: unknown }).rows).toBeUndefined();
  });

  it("expands a Tool linked to focused work by an opens edge", async () => {
    const document: RunGraphDocument = {
      ...empty,
      nodes: [
        { id: "question-1", kind: "question", text: "Prepare renewal" },
        { id: "tool-1", kind: "tool", toolRef: { toolSpecApiName: "RenewalPlaybook" } },
      ],
      edges: [{ id: "opens-1", from: "tool-1", to: "question-1", kind: "opens" }],
    };
    const open = vi.fn();
    render(
      <RunGraphHome
        fetchFn={vi.fn().mockResolvedValue(envelope(document))}
        onOpenTool={open}
      />,
    );
    await screen.findByTestId("mock-graph-view");
    fireEvent.click(screen.getByText("Select first"));
    fireEvent.click(screen.getByText("Open linked Tool"));
    expect(open).toHaveBeenCalledWith(expect.objectContaining({
      id: "tool-1",
      toolRef: { toolSpecApiName: "RenewalPlaybook" },
    }));
  });
});
