import { describe, expect, it } from "vitest";
import {
  BULK_COMPOSITE_MAX,
  buildBulkPatchRequests,
  summarizeCompositeResponse,
} from "./bulkComposite";

describe("buildBulkPatchRequests", () => {
  it("caps at 25 unique ids", () => {
    const ids = Array.from({ length: 30 }, (_, i) => `id-${i}`);
    const { requests, deferred } = buildBulkPatchRequests("Account", ids, { OwnerId: "u1" });
    expect(requests).toHaveLength(BULK_COMPOSITE_MAX);
    expect(deferred).toBe(5);
    expect(requests[0]).toEqual({
      method: "PATCH",
      object: "Account",
      id: "id-0",
      referenceId: "row1",
      body: { OwnerId: "u1" },
    });
  });
});

describe("summarizeCompositeResponse", () => {
  it("counts mixed 200 and 403", () => {
    const summary = summarizeCompositeResponse({
      compositeResponse: [
        { referenceId: "row1", status: 200 },
        { referenceId: "row2", status: 403 },
      ],
    });
    expect(summary.updated).toBe(1);
    expect(summary.forbidden).toBe(1);
    expect(summary.message).toBe("1 updated, 1 forbidden");
  });
});
