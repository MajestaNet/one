import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityFeed } from "./ActivityFeed";
import { RecordForm, requiredMissing } from "./RecordForm";
import { RelatedLists } from "./RelatedLists";
import type { AppBridge } from "../App";

afterEach(() => {
  vi.restoreAllMocks();
});

function bridge(fetchImpl: AppBridge["fetch"]): AppBridge {
  return {
    session: { baseUrl: "http://localhost:8080", token: "t", scopes: ["client"] },
    setSession: vi.fn(),
    fetch: fetchImpl,
  };
}

describe("RecordForm", () => {
  it("renders picklist, checkbox, textarea, and number fields", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <RecordForm
        mode="create"
        values={{ Name: "", Active: false, Notes: "", Amount: "" }}
        onChange={onChange}
        fields={[
          { apiName: "Name", label: "Name", fieldType: "text", required: true },
          { apiName: "Stage", label: "Stage", fieldType: "picklist", picklistValues: ["A", "B"] },
          { apiName: "Active", label: "Active", fieldType: "boolean" },
          { apiName: "Notes", label: "Notes", fieldType: "textarea" },
          { apiName: "Amount", label: "Amount", fieldType: "currency" },
          { apiName: "Id", label: "Id", fieldType: "text" },
        ]}
      />,
    );
    expect(screen.getByLabelText(/Name \*/i)).toBeTruthy();
    await user.selectOptions(screen.getByLabelText(/^Stage/i), "B");
    expect(onChange).toHaveBeenCalled();
    await user.click(screen.getByLabelText(/^Active/i));
    await user.type(screen.getByLabelText(/^Notes/i), "hi");
    await user.type(screen.getByLabelText(/^Amount/i), "12");
    expect(requiredMissing([{ apiName: "Name", required: true }], {})).toEqual(["Name"]);
  });

  it("shows empty state when no editable fields", () => {
    render(<RecordForm mode="edit" values={{}} onChange={() => undefined} fields={[{ apiName: "Id" }]} />);
    expect(screen.getByText(/No editable fields/i)).toBeTruthy();
  });
});

describe("RelatedLists", () => {
  it("loads related rows and opens on click", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const fetch = vi.fn().mockResolvedValue({
      records: [{ id: "c1", LastName: "Shah" }],
    });
    render(
      <RelatedLists
        bridge={bridge(fetch)}
        parentId="a1"
        defs={[
          { objectApiName: "Contact", lookupField: "AccountId", label: "Contacts" },
          { objectApiName: "Opportunity", lookupField: "AccountId", label: "Opportunities", packageName: "sales" },
        ]}
        onOpenRelated={onOpen}
      />,
    );
    expect(await screen.findByText(/Shah/i)).toBeTruthy();
    expect(fetch).toHaveBeenCalledWith(
      "/client/v1/query",
      expect.objectContaining({ method: "POST" }),
    );
    await user.click(screen.getByRole("button", { name: /Shah/i }));
    expect(onOpen).toHaveBeenCalledWith("Contact", "c1");
    await user.click(screen.getByRole("tab", { name: /Opportunities/i }));
    await waitFor(() => expect(fetch.mock.calls.length).toBeGreaterThan(1));
  });

  it("surfaces query errors and offline copy", async () => {
    const fetch = vi.fn().mockRejectedValue(new Error("boom"));
    render(
      <RelatedLists
        bridge={bridge(fetch)}
        parentId="a1"
        defs={[{ objectApiName: "Contact", lookupField: "AccountId", label: "Contacts" }]}
      />,
    );
    expect(await screen.findByText(/boom/i)).toBeTruthy();
    render(
      <RelatedLists
        parentId="a1"
        defs={[{ objectApiName: "Contact", lookupField: "AccountId", label: "Contacts" }]}
      />,
    );
    expect(screen.getAllByText(/Connect to load related lists/i).length).toBeGreaterThanOrEqual(1);
  });
});

describe("ActivityFeed", () => {
  it("returns null when activities package disabled", () => {
    const { container } = render(
      <ActivityFeed
        bridge={bridge(vi.fn())}
        parentType="Account"
        parentId="a1"
        activitiesEnabled={false}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("lists feed items", async () => {
    const fetch = vi.fn().mockResolvedValue({
      items: [
        {
          kind: "activity",
          objectApiName: "Task",
          id: "t1",
          subject: "Follow up",
          status: "Open",
        },
      ],
    });
    render(
      <ActivityFeed
        bridge={bridge(fetch)}
        parentType="Account"
        parentId="a1"
        activitiesEnabled
      />,
    );
    expect(await screen.findByText(/Follow up/i)).toBeTruthy();
    expect(screen.getByText(/Task/i)).toBeTruthy();
    expect(String(fetch.mock.calls[0][0])).toMatch(/\/client\/v1\/activity-feed\?/);
  });

  it("shows connect hint offline and surfaces load errors", async () => {
    render(
      <ActivityFeed parentType="Account" parentId="a1" activitiesEnabled />,
    );
    expect(screen.getByText(/Connect to load the Activity feed/i)).toBeTruthy();
    const fetch = vi.fn().mockRejectedValue(new Error("denied"));
    render(
      <ActivityFeed
        bridge={bridge(fetch)}
        parentType="Account"
        parentId="a1"
        activitiesEnabled
      />,
    );
    expect(await screen.findByText(/denied/i)).toBeTruthy();
  });
});
