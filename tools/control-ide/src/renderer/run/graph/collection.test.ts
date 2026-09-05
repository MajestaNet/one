import { describe, expect, it } from "vitest";
import { bindingFiltersAsQuery, collectionIdentity, collectionListFilters } from "./collection";
import { RUN_GRAPH_API_VERSION, type RunGraphNode } from "./types";
import { validateRunGraphDocument } from "./validate";

describe("collection helpers", () => {
  it("identity ignores empty slice keys", () => {
    expect(collectionIdentity({ objectApiName: "Account" })).toBe("Account::::");
    expect(collectionIdentity({ objectApiName: "Account", searchQ: "Acme" })).toBe("Account::::Acme");
  });

  it("maps binding filters and searchQ into Client query filters", () => {
    expect(bindingFiltersAsQuery({
      id: "open",
      objectApiName: "Account",
      filters: [{ field: "Type", op: "eq", value: "Customer" }, { field: "bad" }],
    })).toEqual([{ field: "Type", op: "eq", value: "Customer" }]);
    const node: RunGraphNode = {
      id: "accounts",
      kind: "collection",
      ref: { objectApiName: "Account" },
      searchQ: "Acme",
    };
    expect(collectionListFilters(node, {
      id: "open",
      objectApiName: "Account",
      filters: [{ field: "Type", op: "eq", value: "Customer" }],
    })).toEqual([
      { field: "Name", op: "like", value: "Acme" },
      { field: "Type", op: "eq", value: "Customer" },
    ]);
    const contact: RunGraphNode = {
      id: "contacts",
      kind: "collection",
      ref: { objectApiName: "Contact" },
      searchQ: "Shah",
    };
    expect(collectionListFilters(contact)).toEqual([{ field: "LastName", op: "like", value: "Shah" }]);
  });
});

describe("collection document validation", () => {
  it("accepts a collection and rejects baked rows plus mismatched bindings", () => {
    const valid = validateRunGraphDocument({
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [
        {
          id: "accounts",
          kind: "collection",
          ref: { objectApiName: "Account" },
          bindingId: "open-accounts",
          searchQ: "Acme",
        },
      ],
      edges: [],
      dataBindings: [{ id: "open-accounts", objectApiName: "Account", filters: [{ field: "Type", op: "eq", value: "Customer" }] }],
    });
    expect(valid.ok).toBe(true);

    const mismatch = validateRunGraphDocument({
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [{ id: "contacts", kind: "collection", ref: { objectApiName: "Contact" }, bindingId: "open-accounts" }],
      edges: [],
      dataBindings: [{ id: "open-accounts", objectApiName: "Account" }],
    });
    expect(mismatch.ok).toBe(false);
  });
});
