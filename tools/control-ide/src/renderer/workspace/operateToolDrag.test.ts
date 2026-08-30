import { describe, expect, it } from "vitest";
import { parseOperateToolDragPayload } from "./operateToolDrag";

describe("Operate Tool drag payload", () => {
  it("accepts only a typed, labelled rail Tool", () => {
    expect(parseOperateToolDragPayload(JSON.stringify({
      type: "operate-tool",
      railId: "tool:AccountBrief",
      label: "Account brief",
      toolSpecApiName: "AccountBrief",
    }))).toEqual({
      type: "operate-tool",
      railId: "tool:AccountBrief",
      label: "Account brief",
      toolSpecApiName: "AccountBrief",
      workingToolId: undefined,
    });
    expect(parseOperateToolDragPayload("not json")).toBeNull();
    expect(parseOperateToolDragPayload(JSON.stringify({ type: "operate-tool" }))).toBeNull();
  });
});
