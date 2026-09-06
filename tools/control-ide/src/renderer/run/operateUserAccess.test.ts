import { describe, expect, it } from "vitest";
import {
  canMountOperateUserCollection,
  filterOperateCatalog,
  isCapabilityRequiredError,
  pruneInaccessibleUserCollections,
} from "./operateUserAccess";
import { RUN_GRAPH_API_VERSION, type RunGraphDocument } from "./graph/types";

describe("operate User collection gating", () => {
  it("hides Users without identity.users", () => {
    expect(canMountOperateUserCollection({ systemPermissions: ["ide.operate"] })).toBe(false);
    expect(
      filterOperateCatalog(
        [
          { apiName: "Account" },
          { apiName: "User" },
          { apiName: "Opportunity" },
        ],
        { systemPermissions: ["ide.operate"] },
      ).map((o) => o.apiName),
    ).toEqual(["Account", "Opportunity"]);
  });

  it("keeps Users when identity.users or admin", () => {
    expect(canMountOperateUserCollection({ systemPermissions: ["identity.users"] })).toBe(true);
    expect(canMountOperateUserCollection({ isAdmin: true })).toBe(true);
    expect(
      filterOperateCatalog([{ apiName: "User" }], { systemPermissions: ["identity.users"] }),
    ).toEqual([{ apiName: "User" }]);
  });

  it("prunes persisted User collection nodes without the cap", () => {
    const document: RunGraphDocument = {
      apiVersion: RUN_GRAPH_API_VERSION,
      id: "home",
      title: "My graph",
      nodes: [
        { id: "users", kind: "collection", label: "Users", ref: { objectApiName: "User" } },
        { id: "accounts", kind: "collection", label: "Accounts", ref: { objectApiName: "Account" } },
      ],
      edges: [{ id: "e1", from: "users", to: "accounts", kind: "relates" }],
    };
    const pruned = pruneInaccessibleUserCollections(document, { systemPermissions: ["ide.operate"] });
    expect(pruned.nodes.map((n) => n.id)).toEqual(["accounts"]);
    expect(pruned.edges).toEqual([]);
  });

  it("detects CAPABILITY_REQUIRED 403s", () => {
    expect(
      isCapabilityRequiredError(
        new Error("403 /client/v1/principals?principalType=user: CAPABILITY_REQUIRED capability identity.users required"),
      ),
    ).toBe(true);
    expect(isCapabilityRequiredError(new Error("no rows"))).toBe(false);
  });
});
