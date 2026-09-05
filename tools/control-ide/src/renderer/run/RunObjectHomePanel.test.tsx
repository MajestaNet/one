import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { defaultFieldColumns, RunObjectHomePanel } from "./RunObjectHomePanel";
import { describeCache } from "../operate/describeCache";

const ACCOUNT_ID = "00000000-0000-4000-8000-000000000111";

afterEach(() => {
  cleanup();
  describeCache.clear();
  vi.restoreAllMocks();
});

describe("defaultFieldColumns", () => {
  it("defaults to five data columns without forcing Id", () => {
    const cols = defaultFieldColumns(
      [
        { apiName: "Name", label: "Name", fieldType: "text" },
        { apiName: "Industry", label: "Industry", fieldType: "text" },
        { apiName: "Phone", label: "Phone", fieldType: "text" },
        { apiName: "Website", label: "Website", fieldType: "text" },
        { apiName: "Type", label: "Type", fieldType: "text" },
        { apiName: "Extra", label: "Extra", fieldType: "text" },
      ],
      5,
    );
    expect(cols).toHaveLength(5);
    expect(cols.map((c) => c.key)).toEqual(["Name", "Industry", "Phone", "Website", "Type"]);
    expect(cols.some((c) => c.key === "id" || c.key === "Id")).toBe(false);
  });

  it("prefers LastName then FirstName for Contact", () => {
    const cols = defaultFieldColumns(
      [
        { apiName: "Salutation", label: "Salutation", fieldType: "picklist" },
        { apiName: "FirstName", label: "First Name", fieldType: "text" },
        { apiName: "MiddleName", label: "Middle Name", fieldType: "text" },
        { apiName: "LastName", label: "Last Name", fieldType: "text" },
        { apiName: "Email", label: "Email", fieldType: "email" },
      ],
      5,
    );
    expect(cols.map((c) => c.key).slice(0, 3)).toEqual(["LastName", "FirstName", "Email"]);
  });
});

describe("RunObjectHomePanel", () => {
  it("shows an honest empty state for an admin when the object has no records", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return { sobjects: [{ name: "Account", label: "Account", labelPlural: "Accounts" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return {
          apiName: "Account",
          label: "Account",
          fields: [{ apiName: "Name", label: "Name", fieldType: "text" }],
        };
      }
      if (path === "/client/v1/query") return { records: [] };
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunObjectHomePanel
        bridge={{
          session: {
            baseUrl: "http://localhost:8080",
            token: "t",
            scopes: ["client"],
            isAdmin: true,
            activeInstallId: "inst-1",
          },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
      />,
    );

    expect(await screen.findByTestId("run-object-home-empty")).toBeTruthy();
    expect(screen.getByText("No Account records yet")).toBeTruthy();
    expect(screen.getByText(/object definitions without sample customer data/i)).toBeTruthy();
    expect(screen.queryByText(/No records visible under your sharing rules/i)).toBeNull();

    await user.click(screen.getByRole("button", { name: "Create Account" }));
    expect(screen.getByTestId("run-object-home-form")).toBeTruthy();
  });

  it("loads Client describe + query and opens record via get-by-id", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") {
        return { sobjects: [{ name: "Account", label: "Account", labelPlural: "Accounts" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return {
          apiName: "Account",
          label: "Account",
          fields: [
            { apiName: "Name", label: "Name", fieldType: "text" },
            { apiName: "Industry", label: "Industry", fieldType: "text" },
            { apiName: "Phone", label: "Phone", fieldType: "text" },
            { apiName: "Website", label: "Website", fieldType: "text" },
            { apiName: "Type", label: "Type", fieldType: "text" },
          ],
        };
      }
      if (path === "/client/v1/query" && init?.method === "POST") {
        const body = JSON.parse(String(init.body ?? "{}")) as { filters?: unknown[] };
        if (body.filters?.length) {
          return { records: [{ id: ACCOUNT_ID, Name: "Acme", Industry: "Mfg" }] };
        }
        return {
          records: [
            { id: ACCOUNT_ID, Name: "Acme", Industry: "Mfg", Phone: "1", Website: "a.com", Type: "Customer" },
            { id: "a2", Name: "Globex", Industry: "Tech", Phone: "2", Website: "g.com", Type: "Partner" },
          ],
        };
      }
      if (path === `/client/v1/sobjects/Account/${ACCOUNT_ID}` && init?.method === "PATCH") {
        return { ok: true };
      }
      if (path === `/client/v1/sobjects/Account/${ACCOUNT_ID}`) {
        return { id: ACCOUNT_ID, Name: "Acme", Industry: "Mfg", Phone: "555" };
      }
      if (path === "/client/v1/sobjects/Account" && init?.method === "POST") {
        return { id: "a2", Name: "Globex" };
      }
      if (path === "/client/v1/sobjects/Account/a2") {
        return { id: "a2", Name: "Globex", Industry: "Technology" };
      }
      if (path === "/client/v1/run-graphs/home" && (!init?.method || init.method === "GET")) {
        return {
          graphKey: "home",
          revision: 1,
          document: {
            apiVersion: "one.runGraph/v1",
            id: "home",
            title: "My graph",
            nodes: [],
            edges: [],
          },
        };
      }
      if (path === "/client/v1/run-graphs/home" && init?.method === "PUT") {
        return { graphKey: "home", revision: 2, document: JSON.parse(String(init.body)) };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunObjectHomePanel
        bridge={{
          session: {
            baseUrl: "http://localhost:8080",
            token: "t",
            scopes: ["client"],
            activeInstallId: "inst-1",
          },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
      />,
    );

    expect(screen.getByText("List View")).toBeTruthy();
    await waitFor(() => expect(screen.getByTestId("run-object-home-table")).toBeTruthy());
    expect(screen.getByText("Acme")).toBeTruthy();
    expect(screen.getByTestId("run-list-select-all")).toBeTruthy();
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/query", expect.any(Object));

    await user.click(screen.getByTestId("run-list-filters-toggle"));
    expect(screen.getByTestId("run-list-filters-menu")).toBeTruthy();
    await user.clear(screen.getByTestId("run-list-filter-value"));
    await user.type(screen.getByTestId("run-list-filter-value"), "Acme");
    await user.click(screen.getByTestId("run-list-filter-apply"));
    await waitFor(() => {
      const queryCall = fetchFn.mock.calls.find(
        ([path, init]) => path === "/client/v1/query" && String(init?.body ?? "").includes("filters"),
      );
      expect(queryCall).toBeTruthy();
    });

    await user.click(screen.getByTestId(`run-object-home-row-${ACCOUNT_ID}`));
    await waitFor(() => expect(screen.getByTestId("run-object-home-record")).toBeTruthy());
    expect(fetchFn).toHaveBeenCalledWith(`/client/v1/sobjects/Account/${ACCOUNT_ID}`);
    expect(screen.getByText("555")).toBeTruthy();

    await user.click(screen.getByTestId("run-object-home-pin"));
    await waitFor(() => expect(screen.getByTestId("run-object-home-pin-status")).toBeTruthy());
    const pinPut = fetchFn.mock.calls.find(
      ([path, init]) => path === "/client/v1/run-graphs/home" && init?.method === "PUT",
    );
    expect(pinPut).toBeTruthy();
    const pinBody = JSON.parse(String(pinPut?.[1]?.body)) as { nodes: Array<{ kind: string; ref?: { objectApiName?: string; recordId?: string } }> };
    expect(pinBody.nodes.some((n) => n.kind === "record" && n.ref?.recordId === ACCOUNT_ID)).toBe(true);

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Acme updated");
    await user.click(screen.getByRole("button", { name: /^Save$/i }));
    await waitFor(() =>
      expect(fetchFn).toHaveBeenCalledWith(
        `/client/v1/sobjects/Account/${ACCOUNT_ID}`,
        expect.objectContaining({ method: "PATCH" }),
      ),
    );
    const patchCall = fetchFn.mock.calls.find(([path, init]) => path.endsWith(`/${ACCOUNT_ID}`) && init?.method === "PATCH");
    const patchBody = JSON.parse(String(patchCall?.[1]?.body));
    expect(patchBody.Name).toBe("Acme updated");

    await user.click(screen.getByRole("button", { name: /New Account/i }));
    await user.type(screen.getByLabelText("Name"), "Globex");
    await user.click(screen.getByRole("button", { name: /^Save$/i }));
    await waitFor(() =>
      expect(fetchFn).toHaveBeenCalledWith(
        "/client/v1/sobjects/Account",
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });

  it("applies agent-supplied filters via initialFilters", async () => {
    const fetchFn = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return { sobjects: [{ name: "Account", label: "Account" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return {
          apiName: "Account",
          fields: [{ apiName: "Name", label: "Name", fieldType: "text" }],
        };
      }
      if (path === "/client/v1/query") {
        return { records: [{ id: "a1", Name: "Filtered" }] };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunObjectHomePanel
        bridge={{
          session: {
            baseUrl: "http://localhost:8080",
            token: "t",
            scopes: ["client"],
            activeInstallId: "inst-1",
          },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
        initialFilters={[{ field: "Name", op: "eq", value: "Filtered" }]}
        filtersEpoch={1}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("run-list-active-filters")).toBeTruthy());
    await waitFor(() => {
      const body = fetchFn.mock.calls.find(([p]) => p === "/client/v1/query")?.[1]?.body;
      expect(String(body)).toContain("Filtered");
    });
  });

  it("bulk-assigns selected rows via composite and surfaces mixed 200/403", async () => {
    const user = userEvent.setup();
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") {
        return { sobjects: [{ name: "Account", label: "Account" }] };
      }
      if (path === "/client/v1/describe/Account") {
        return {
          apiName: "Account",
          fields: [
            { apiName: "Name", label: "Name", fieldType: "text" },
            { apiName: "Status", label: "Status", fieldType: "picklist", picklistValues: ["New", "Closed"] },
          ],
        };
      }
      if (path === "/client/v1/query") {
        return {
          records: [
            { id: "a1", Name: "One", Status: "New" },
            { id: "a2", Name: "Two", Status: "New" },
          ],
        };
      }
      if (path === "/client/v1/composite" && init?.method === "POST") {
        const body = JSON.parse(String(init.body ?? "{}")) as { compositeRequest?: unknown[] };
        expect(body.compositeRequest).toHaveLength(2);
        return {
          compositeResponse: [
            { referenceId: "row1", status: 200 },
            { referenceId: "row2", status: 403 },
          ],
        };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunObjectHomePanel
        bridge={{
          session: {
            baseUrl: "http://localhost:8080",
            token: "t",
            scopes: ["client"],
            activeInstallId: "inst-1",
          },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
      />,
    );

    await waitFor(() => expect(screen.getByTestId("run-object-home-table")).toBeTruthy());
    const rowChecks = screen.getAllByLabelText("Select row for chat");
    await user.click(rowChecks[0]);
    await user.click(rowChecks[1]);
    expect(screen.getByTestId("run-object-home-bulk")).toBeTruthy();
    expect(screen.getByTestId("run-object-home-bulk-count").textContent).toMatch(/2 selected/);

    await user.type(screen.getByTestId("run-object-home-bulk-owner"), "user-9");
    await user.click(screen.getByTestId("run-object-home-bulk-assign"));
    await waitFor(() => expect(screen.getByTestId("run-object-home-bulk-result").textContent).toMatch(/1 updated, 1 forbidden/));
    const compositeCall = fetchFn.mock.calls.find(([p]) => p === "/client/v1/composite");
    expect(compositeCall).toBeTruthy();
  });

  it("lists User via principals instead of DataEngine query", async () => {
    const fetchFn = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return {
          sobjects: [{ name: "User", label: "User", labelPlural: "Users", storageMode: "kernel" }],
        };
      }
      if (path === "/client/v1/describe/User") {
        return {
          apiName: "User",
          storageMode: "kernel",
          fields: [
            { apiName: "DisplayName", label: "Display Name", fieldType: "text" },
            { apiName: "Email", label: "Email", fieldType: "email" },
          ],
        };
      }
      if (path === "/client/v1/principals?principalType=user") {
        return { principals: [{ id: "u1", displayName: "Ada Lovelace", email: "ada@x.com" }] };
      }
      throw new Error(`unexpected ${path}`);
    });

    render(
      <RunObjectHomePanel
        bridge={{
          session: {
            baseUrl: "http://localhost:8080",
            token: "t",
            scopes: ["client"],
            isAdmin: true,
            activeInstallId: "inst-1",
          },
          setSession: async () => undefined,
          fetch: fetchFn,
        }}
        initialObjectApiName="User"
        lockObject
      />,
    );

    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeTruthy());
    expect(fetchFn).toHaveBeenCalledWith("/client/v1/principals?principalType=user");
    expect(fetchFn.mock.calls.some(([path]) => path === "/client/v1/query")).toBe(false);
  });
});
