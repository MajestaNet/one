import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { ObjectManagerPanel } from "./ObjectManagerPanel";
import { upsertEnvironment } from "../session";

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(window, "one");
  vi.restoreAllMocks();
});

function bridge(fetchImpl?: AppBridge["fetch"], repoPath?: string): AppBridge {
  let session = upsertEnvironment(null, {
    installId: "dev",
    installRole: "test",
    baseUrl: "http://api",
    token: "jwt",
  });
  if (repoPath) session = { ...session, repoPath };
  return {
    session,
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn(),
  };
}

describe("ObjectManagerPanel", () => {
  it("lists objects in a searchable table and opens detail with fields", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        objects: [
          { apiName: "Account", label: "Account", pluralLabel: "Accounts", ownership: "managed" },
          { apiName: "Zulu__c", label: "Zulu", pluralLabel: "Zulus", ownership: "custom" },
        ],
      })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Account",
        label: "Account",
        pluralLabel: "Accounts",
        ownership: "managed",
        fields: [{ apiName: "Name", label: "Name", fieldType: "text", required: true }],
      });
    render(<ObjectManagerPanel bridge={bridge(fetch)} />);
    const list = await screen.findByTestId("om-object-list");
    expect(within(list).getByText("Accounts")).toBeTruthy();
    expect(within(list).getAllByText("Account").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId("om-search")).toBeTruthy();
    await user.type(screen.getByTestId("om-search"), "zul");
    expect(screen.queryByTestId("om-row-Account")).toBeNull();
    expect(screen.getByTestId("om-row-Zulu__c")).toBeTruthy();
    await user.clear(screen.getByTestId("om-search"));
    await user.click(screen.getByRole("button", { name: /Open Account/i }));
    expect(await screen.findByTestId("om-field-table")).toBeTruthy();
    expect(within(screen.getByTestId("om-field-table")).getAllByText("Name").length).toBeGreaterThanOrEqual(1);
    expect(within(screen.getByTestId("om-field-table")).getByText("text")).toBeTruthy();
    await user.click(screen.getByTestId("om-back"));
    expect(await screen.findByTestId("om-object-list")).toBeTruthy();
  });

  it("creates a customer object with flexible storage and mirrors yaml when repoPath set", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({ objects: [] })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Order",
        pluralLabel: "Orders",
        storageMode: "flexible",
        ownership: "custom",
      })
      .mockResolvedValueOnce({
        objects: [{ apiName: "Order__c", label: "Order", pluralLabel: "Orders", ownership: "custom" }],
      })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Order",
        ownership: "custom",
        fields: [],
      });
    render(<ObjectManagerPanel bridge={bridge(fetch, "/tmp/repo")} />);
    await screen.findByTestId("om-object-list");
    await user.click(screen.getByRole("button", { name: /New object/i }));
    const form = await screen.findByTestId("om-new-object");
    expect(form.className).toMatch(/om-new-object-form/);
    const actions = screen.getByTestId("om-new-object-actions");
    expect(form.lastElementChild).toBe(actions);
    expect(within(form).getByRole("button", { name: /Create object/i }).closest("[data-testid=om-new-object-actions]")).toBeTruthy();
    const inputs = form.querySelectorAll("input");
    // Label, Plural label, API name
    await user.type(inputs[0], "Order");
    await user.type(inputs[2], "Order__c");
    await user.click(screen.getByRole("button", { name: /Create object/i }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(fetch).toHaveBeenCalledWith(
      "/metadata/v1/objects",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"storageMode":"flexible"'),
      }),
    );
    expect(writeText.mock.calls[0][0]).toBe("/tmp/repo");
    expect(writeText.mock.calls[0][1]).toBe("metadata/objects/Order__c.yaml");
    expect(String(writeText.mock.calls[0][2])).toContain("apiName: Order__c");
    expect(String(writeText.mock.calls[0][2])).toContain("storageMode: flexible");
  });

  it("shows connect empty state without session token", () => {
    render(
      <ObjectManagerPanel
        bridge={{ session: null, setSession: vi.fn(), fetch: vi.fn() }}
      />,
    );
    expect(screen.getByText(/Connect an environment/i)).toBeTruthy();
  });

  it("creates a field on selected object and mirrors yaml", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(true);
    window.one = {
      getSession: vi.fn(),
      setSession: vi.fn(),
      gitStatus: vi.fn(),
      listTree: vi.fn(),
      readText: vi.fn(),
      writeText,
    };
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        objects: [{ apiName: "Order__c", label: "Order", pluralLabel: "Orders", ownership: "custom" }],
      })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Order",
        ownership: "custom",
        fields: [],
      })
      .mockResolvedValueOnce({
        apiName: "Region__c",
        label: "Region",
        fieldType: "text",
        ownership: "custom",
      })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Order",
        ownership: "custom",
        fields: [{ apiName: "Region__c", label: "Region", fieldType: "text" }],
      });
    render(<ObjectManagerPanel bridge={bridge(fetch, "/tmp/repo")} />);
    await screen.findByTestId("om-object-list");
    await user.click(screen.getByRole("button", { name: /Open Order/i }));
    await screen.findByTestId("om-field-table");
    await user.click(screen.getByRole("button", { name: /New field/i }));
    const form = await screen.findByTestId("om-new-field");
    const inputs = form.querySelectorAll("input");
    await user.type(inputs[0], "Region__c");
    await user.type(inputs[1], "Region");
    await user.click(screen.getByRole("button", { name: /Create field/i }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(writeText.mock.calls[0][1]).toBe("metadata/fields/Order__c/Region__c.yaml");
    expect(fetch).toHaveBeenCalledWith(
      "/metadata/v1/fields",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"fieldType":"text"'),
      }),
    );
  });

  it("saves customer object label", async () => {
    const user = userEvent.setup();
    const fetch = vi
      .fn()
      .mockResolvedValueOnce({
        objects: [{ apiName: "Order__c", label: "Order", pluralLabel: "Orders", ownership: "custom" }],
      })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Order",
        pluralLabel: "Orders",
        ownership: "custom",
        fields: [],
      })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Orders Renamed",
        pluralLabel: "Orders",
        ownership: "custom",
      })
      .mockResolvedValueOnce({
        objects: [{ apiName: "Order__c", label: "Orders Renamed", pluralLabel: "Orders", ownership: "custom" }],
      })
      .mockResolvedValueOnce({ fieldTypes: [{ apiName: "text", label: "Text" }] })
      .mockResolvedValueOnce({
        apiName: "Order__c",
        label: "Orders Renamed",
        ownership: "custom",
        fields: [],
      });
    render(<ObjectManagerPanel bridge={bridge(fetch)} />);
    await user.click(await screen.findByRole("button", { name: /Open Order/i }));
    const labelInput = await screen.findByDisplayValue("Order");
    await user.clear(labelInput);
    await user.type(labelInput, "Orders Renamed");
    await user.click(screen.getByRole("button", { name: /Save object/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/metadata/v1/objects/Order__c",
        expect.objectContaining({ method: "PATCH" }),
      ),
    );
  });
});
