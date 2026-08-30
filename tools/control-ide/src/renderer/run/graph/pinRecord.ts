import { executeGraphBridge, type GraphBridgeContext, type GraphBridgeResult } from "./agentGraphTools";
import type { GraphBridgeError, GraphPinInput } from "./agentGraphContracts";
import type { RunGraphFetch } from "./api";
import type { BoardHandoff } from "../../operate/types";

/** Pin a Client record ref onto the principal home Run graph (idempotent). */
export async function pinRecordToHomeGraph(
  fetch: RunGraphFetch,
  input: GraphPinInput,
): Promise<GraphBridgeResult | GraphBridgeError> {
  return executeGraphBridge("graph.pin", input, { fetch } satisfies GraphBridgeContext);
}

/** Pins an Operate working set into Run attention without copying record fields. */
export async function pinBoardHandoffToHomeGraph(
  fetch: RunGraphFetch,
  handoff: BoardHandoff,
): Promise<{ nodeIds: string[] }> {
  const objectApiName = handoff.objectApiName?.trim();
  const recordIds = [...new Set((handoff.recordIds ?? []).map((id) => id.trim()).filter(Boolean))];
  if (!objectApiName) throw new Error("Board handoff needs an object before it can be pinned");
  if (!recordIds.length) throw new Error("Board handoff has no record ids to pin");

  const nodeIds: string[] = [];
  for (const recordId of recordIds) {
    const result = await pinRecordToHomeGraph(fetch, { objectApiName, recordId });
    if (!result.ok) throw new Error(String(result.error));
    nodeIds.push(String(result.nodeId));
  }
  return { nodeIds };
}
