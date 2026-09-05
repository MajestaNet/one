import { describe, expect, it, vi } from "vitest";
import {
  createRecord,
  getRecord,
  isKernelIdentityObject,
  listIdentityField,
  principalToRecord,
  queryRecords,
  recordWritePayload,
  updateRecord,
} from "./recordClient";

describe("recordClient", () => {
  it("treats User as kernel identity", () => {
    expect(isKernelIdentityObject("User")).toBe(true);
    expect(isKernelIdentityObject("Account")).toBe(false);
    expect(isKernelIdentityObject("Custom__c", "kernel")).toBe(true);
  });

  it("picks LastName for Contact list/search filters", () => {
    expect(listIdentityField("Contact")).toBe("LastName");
    expect(listIdentityField("Account")).toBe("Name");
    expect(listIdentityField("User")).toBe("DisplayName");
  });

  it("omits blank optional fields from write payloads", () => {
    expect(
      recordWritePayload(
        [
          { apiName: "LastName", fieldType: "text", required: true },
          { apiName: "Email", fieldType: "email" },
          { apiName: "AccountId", fieldType: "lookup" },
          { apiName: "Id", fieldType: "text" },
        ],
        { LastName: "Shah", Email: "", AccountId: "  ", Id: "should-skip" },
      ),
    ).toEqual({ LastName: "Shah" });
  });

  it("lists User via principals instead of /query", async () => {
    const fetchFn = vi.fn(async (path: string) => {
      expect(path).toBe("/client/v1/principals?principalType=user");
      return {
        principals: [
          { id: "u1", email: "ada@x.com", displayName: "Ada", userName: "ada" },
          { id: "u2", email: "bob@x.com", displayName: "Bob" },
        ],
      };
    });
    const q = await queryRecords(fetchFn, {
      object: "User",
      filters: [{ field: "DisplayName", op: "like", value: "Ada" }],
      limit: 50,
    });
    expect(q.records).toHaveLength(1);
    expect(q.records[0]).toMatchObject({ Id: "u1", Email: "ada@x.com", DisplayName: "Ada", Name: "Ada" });
  });

  it("queries flexible objects through DataEngine", async () => {
    const fetchFn = vi.fn(async () => ({ records: [{ Id: "c1", LastName: "Shah" }] }));
    const q = await queryRecords(fetchFn, { object: "Contact", limit: 50 });
    expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/query",
      expect.objectContaining({ method: "POST" }),
    );
    expect(JSON.parse(String(fetchFn.mock.calls[0]?.[1]?.body))).toMatchObject({ object: "Contact", limit: 50 });
    expect(q.records[0].LastName).toBe("Shah");
  });

  it("gets and patches User through principals", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/principals/u1" && !init?.method) {
        return { id: "u1", email: "ada@x.com", displayName: "Ada" };
      }
      if (path === "/client/v1/principals/u1" && init?.method === "PATCH") {
        return { id: "u1", displayName: "Ada Lovelace" };
      }
      throw new Error(path);
    });
    await expect(getRecord(fetchFn, "User", "u1")).resolves.toMatchObject({ Id: "u1", Email: "ada@x.com" });
    await updateRecord(fetchFn, "User", "u1", { DisplayName: "Ada Lovelace" });
    const patch = JSON.parse(String(fetchFn.mock.calls[1]?.[1]?.body));
    expect(patch).toEqual({ displayName: "Ada Lovelace" });
    expect(patch.roleApiNames).toBeUndefined();
  });

  it("creates User through principals with a default role", async () => {
    const fetchFn = vi.fn(async () => ({ id: "u3", email: "new@x.com", displayName: "New" }));
    await createRecord(fetchFn, "User", { Email: "new@x.com", DisplayName: "New" });
    expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/principals",
      expect.objectContaining({ method: "POST" }),
    );
    expect(JSON.parse(String(fetchFn.mock.calls[0]?.[1]?.body))).toMatchObject({
      email: "new@x.com",
      displayName: "New",
      roleApiNames: ["StandardUser"],
    });
  });

  it("maps nested principal name onto GivenName/FamilyName", () => {
    const rec = principalToRecord({
      id: "u1",
      email: "a@x.com",
      name: { givenName: "Ada", familyName: "Lovelace" },
    });
    expect(rec.GivenName).toBe("Ada");
    expect(rec.FamilyName).toBe("Lovelace");
  });
});
