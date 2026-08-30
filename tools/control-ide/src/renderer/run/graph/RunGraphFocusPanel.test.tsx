import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RunGraphFocusPanel } from "./RunGraphFocusPanel";
import { ProposalStagingStore } from "./proposalStaging";
import type { RunGraphNode } from "./types";
import { describeCache } from "../../operate/describeCache";

afterEach(() => {
  cleanup();
  describeCache.clear();
});

describe("RunGraphFocusPanel", () => {
  it("hydrates and edits a record through Client without writing fields to the graph", async () => {
    const node: RunGraphNode = {
      id: "account-1",
      kind: "record",
      ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000111" },
    };
    let industry = "Manufacturing";
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe/Account") {
        return {
          apiName: "Account",
          fields: [
            { apiName: "Name", label: "Name", fieldType: "text", required: true },
            { apiName: "Industry", label: "Industry", fieldType: "text" },
            { apiName: "Id", label: "Id", fieldType: "text" },
          ],
        };
      }
      if (path === "/metadata/v1/packages") return { packages: [] };
      if (path.endsWith("/sobjects/Account/00000000-0000-4000-8000-000000000111")) {
        if (init?.method === "PATCH") {
          industry = String((JSON.parse(String(init.body)) as Record<string, unknown>).Industry);
          return {};
        }
        return { Id: node.ref?.recordId, data: { Name: "Acme", Industry: industry } };
      }
      throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
    });
    const saved = vi.fn();

    render(
      <RunGraphFocusPanel
        node={node}
        fetchFn={fetchFn}
        onClose={() => undefined}
        onRecordSaved={saved}
      />,
    );

    expect(await screen.findByDisplayValue("Acme")).toBeTruthy();
    fireEvent.click(screen.getByText("Open full record"));
    fireEvent.change(screen.getByLabelText("Industry"), { target: { value: "Technology" } });
    fireEvent.click(screen.getByText("Save record"));

    await screen.findByText("Record saved.");
    const patchCall = fetchFn.mock.calls.find(([, init]) => init?.method === "PATCH");
    expect(patchCall?.[0]).toContain("/client/v1/sobjects/Account/");
    expect(JSON.parse(String(patchCall?.[1]?.body))).toEqual({
      Name: "Acme",
      Industry: "Technology",
    });
    await waitFor(() => expect(saved).toHaveBeenCalledWith(node));
    expect(fetchFn.mock.calls.some(([path]) => path.includes("run-graphs"))).toBe(false);
  });

  it("applies a staged proposal through Client then resolves its graph pin", async () => {
    const node: RunGraphNode = { id: "proposal-node", kind: "proposal", proposalId: "proposal-1" };
    const store = new ProposalStagingStore();
    store.stage({
      proposalId: "proposal-1",
      mutations: [{ op: "update", object: "Account", id: "a-1", data: { Name: "Acme" } }],
    });
    const fetchFn = vi.fn().mockResolvedValue({});
    const resolved = vi.fn().mockResolvedValue(undefined);

    render(
      <RunGraphFocusPanel
        node={node}
        fetchFn={fetchFn}
        proposalStore={store}
        onResolveProposal={resolved}
        onClose={() => undefined}
      />,
    );
    expect(screen.getByTestId("run-graph-proposal-review").textContent).toMatch(/Account.*a-1/s);
    fireEvent.click(screen.getByText("Approve and apply"));

    await waitFor(() => expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/sobjects/Account/a-1",
      { method: "PATCH", body: JSON.stringify({ Name: "Acme" }) },
    ));
    await waitFor(() => expect(resolved).toHaveBeenCalledWith("applied"));
  });

  it("rejects without calling Client and removes unavailable session proposals", async () => {
    const node: RunGraphNode = { id: "proposal-node", kind: "proposal", proposalId: "missing" };
    const fetchFn = vi.fn();
    const resolved = vi.fn().mockResolvedValue(undefined);

    render(
      <RunGraphFocusPanel
        node={node}
        fetchFn={fetchFn}
        proposalStore={new ProposalStagingStore()}
        onResolveProposal={resolved}
        onClose={() => undefined}
      />,
    );
    expect(screen.getByTestId("run-graph-proposal-unavailable")).toBeTruthy();
    fireEvent.click(screen.getByText("Remove unavailable proposal"));
    await waitFor(() => expect(resolved).toHaveBeenCalledWith("rejected"));
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("shows live signal rows and pins survivors through graph callback", async () => {
    const node: RunGraphNode = { id: "signal-1", kind: "signal", bindingId: "renewals" };
    const pin = vi.fn().mockResolvedValue(1);
    render(
      <RunGraphFocusPanel
        node={node}
        fetchFn={vi.fn()}
        signalResult={{
          nodeId: node.id,
          bindingId: "renewals",
          objectApiName: "Opportunity",
          rows: [{ id: "00000000-0000-4000-8000-000000000111", Name: "Acme renewal" }],
          fetchedAt: 1,
        }}
        onPinSignalRows={pin}
        onClose={() => undefined}
      />,
    );

    expect(screen.getByTestId("run-graph-signal-live").textContent).toMatch(/Acme renewal/);
    fireEvent.click(screen.getByText("Pin survivors"));
    await waitFor(() => expect(pin).toHaveBeenCalledWith(expect.objectContaining({
      objectApiName: "Opportunity",
      rows: [expect.objectContaining({ Name: "Acme renewal" })],
    })));
    expect(await screen.findByText(/Ensured 1 survivor pin in your personal graph/)).toBeTruthy();
  });

  it("renders the object list in focus for a collection node", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") {
        return { objects: [{ apiName: "Account", label: "Account" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return { apiName: "Account", fields: [{ apiName: "Name", label: "Name", fieldType: "text" }] };
      }
      if (path === "/client/v1/query") {
        expect(JSON.parse(String(init?.body))).toMatchObject({
          object: "Account",
          filters: [{ field: "Name", op: "like", value: "Acme" }],
        });
        return { records: [{ id: "00000000-0000-4000-8000-000000000111", Name: "Acme" }] };
      }
      throw new Error(`unexpected ${init?.method ?? "GET"} ${path}`);
    });
    render(
      <RunGraphFocusPanel
        node={{
          id: "accounts",
          kind: "collection",
          ref: { objectApiName: "Account" },
          label: "Accounts",
          searchQ: "Acme",
        }}
        fetchFn={fetchFn}
        bridge={{
          session: { baseUrl: "http://localhost:8080", token: "t", scopes: ["client"], activeInstallId: "inst-1" },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
        onClose={() => undefined}
      />,
    );
    expect(await screen.findByTestId("run-graph-collection-focus")).toBeTruthy();
    expect(screen.getByTestId("run-object-home-locked-object").textContent).toMatch(/Account/);
    expect(await screen.findByText("Acme")).toBeTruthy();
    expect(screen.queryByText("List View")).toBeNull();
  });

  it("opens a Tool linked from focus through an opens edge", () => {
    const open = vi.fn();
    const linkedTool: RunGraphNode = {
      id: "tool-1",
      kind: "tool",
      toolRef: { toolSpecApiName: "RenewalPlaybook" },
    };
    render(
      <RunGraphFocusPanel
        node={{ id: "question-1", kind: "question", text: "How should we renew?" }}
        fetchFn={vi.fn()}
        linkedTool={linkedTool}
        onOpenTool={open}
        onClose={() => undefined}
      />,
    );
    expect(screen.getByTestId("run-graph-opens-tool").textContent).toMatch(/RenewalPlaybook/);
    fireEvent.click(screen.getByText("Open linked Tool"));
    expect(open).toHaveBeenCalledWith(linkedTool);
  });
});
