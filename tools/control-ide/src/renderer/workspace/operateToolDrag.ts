export const OPERATE_TOOL_DRAG_MIME = "application/x-one-operate-tool";

export type OperateToolDragPayload = {
  type: "operate-tool";
  railId: string;
  label: string;
  toolSpecApiName?: string;
  workingToolId?: string;
};

export function parseOperateToolDragPayload(raw: string): OperateToolDragPayload | null {
  try {
    const value = JSON.parse(raw) as Record<string, unknown>;
    if (value.type !== "operate-tool" || typeof value.railId !== "string" || typeof value.label !== "string") {
      return null;
    }
    return {
      type: "operate-tool",
      railId: value.railId,
      label: value.label,
      toolSpecApiName: typeof value.toolSpecApiName === "string" ? value.toolSpecApiName : undefined,
      workingToolId: typeof value.workingToolId === "string" ? value.workingToolId : undefined,
    };
  } catch {
    return null;
  }
}
