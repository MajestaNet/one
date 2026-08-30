import { afterEach, describe, expect, it } from "vitest";
import { deleteSavedView, loadSavedViews, persistSavedViews, upsertSavedView, viewsForObject } from "./views";

describe("saved views", () => {
  afterEach(() => {
    localStorage.removeItem("one.control.operate.savedViews");
  });

  it("persists and filters by object", () => {
    persistSavedViews([]);
    const view = {
      id: "v1",
      name: "Customers",
      objectApiName: "Account",
      filters: [{ field: "Type", op: "eq" as const, value: "Customer" }],
      sort: [{ field: "Name", direction: "asc" as const }],
      limit: 50,
    };
    const next = upsertSavedView([], view);
    expect(loadSavedViews()).toHaveLength(1);
    expect(viewsForObject(next, "Account")).toHaveLength(1);
    expect(viewsForObject(next, "Contact")).toHaveLength(0);
    expect(deleteSavedView(next, "v1")).toHaveLength(0);
  });
});
