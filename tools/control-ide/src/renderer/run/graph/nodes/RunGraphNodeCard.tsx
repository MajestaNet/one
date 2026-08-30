import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import type { RunGraphNode, RunGraphResolveResult } from "../types";

export type RunGraphFlowNodeData = Record<string, unknown> & {
  graphNode: RunGraphNode;
  resolve?: RunGraphResolveResult;
  toolLabel?: string;
  onStartConnect?: (nodeId: string) => void;
  connecting?: boolean;
};

function recordLabel(result: RunGraphResolveResult | undefined): string {
  if (!result) return "Loading record…";
  if (!result.ok) return result.code === "NOT_FOUND" ? "Record not found" : "Record unavailable";
  const record = result.record ?? {};
  if (typeof record.Name === "string" && record.Name) return record.Name;
  const person = [record.FirstName, record.LastName].filter((part) => typeof part === "string").join(" ");
  if (person) return person;
  for (const key of ["Subject", "Title", "Id"]) {
    if (typeof record[key] === "string" && record[key]) return String(record[key]);
  }
  return "Record";
}

export function runGraphNodeLabel(
  node: RunGraphNode,
  resolve?: RunGraphResolveResult,
  toolLabel?: string,
): { title: string; detail?: string } {
  switch (node.kind) {
    case "record":
      {
        const record = resolve?.ok ? resolve.record ?? {} : {};
        const glance = ["Email", "Phone", "AccountNumber", "StageName", "Status"]
          .map((key) => record[key])
          .find((value) => typeof value === "string" && value.trim());
      return {
        title: recordLabel(resolve),
        detail: resolve?.ok
          ? [node.ref?.objectApiName, glance].filter(Boolean).join(" · ")
          : resolve?.code ?? node.ref?.objectApiName,
      };
      }
    case "cluster":
      return { title: node.label || "Group" };
    case "tool":
      return {
        title: toolLabel || node.toolRef?.toolSpecApiName || node.toolRef?.workingToolId || "Tool",
        detail: "Open Tool",
      };
    case "insight":
      return { title: node.text || "Note" };
    case "question":
      return { title: node.text || "Question" };
    case "proposal":
      return { title: "Change review" };
    case "signal":
      return { title: "Live list" };
    case "person":
      return { title: node.label || "Person" };
    case "collection":
      return {
        title: node.label || node.ref?.objectApiName || "Object",
        detail: node.searchQ
          ? `${node.ref?.objectApiName ?? "Object"} · find “${node.searchQ}”`
          : node.bindingId
            ? `${node.ref?.objectApiName ?? "Object"} · saved list`
            : `${node.ref?.objectApiName ?? "Object"} · list`,
      };
  }
}

function kindLabel(node: RunGraphNode): string {
  switch (node.kind) {
    case "collection": return node.searchQ ? "Find" : node.bindingId ? "Saved list" : "List";
    case "record": return "Record";
    case "insight": return "Note";
    case "question": return "Question";
    case "tool": return "Tool";
    case "signal": return "Live signal";
    case "proposal": return "Proposal";
    case "person": return "Person";
    case "cluster": return "Group";
  }
}

export function RunGraphNodeCard({ data }: NodeProps<Node<RunGraphFlowNodeData>>) {
  const { graphNode, resolve, toolLabel, onStartConnect, connecting } = data;
  const label = runGraphNodeLabel(graphNode, resolve, toolLabel);
  const unavailable = graphNode.kind === "record" && resolve && !resolve.ok;
  return (
    <article
      className={`run-graph-node run-graph-node-${graphNode.kind} ${unavailable ? "is-unavailable" : ""}`}
      data-testid={`run-graph-node-${graphNode.id}`}
    >
      <Handle type="target" position={Position.Left} className="run-graph-handle" />
      <span className="run-graph-node-kind">{kindLabel(graphNode)}</span>
      <strong>{label.title}</strong>
      {label.detail ? <small>{label.detail}</small> : null}
      {graphNode.kind === "collection" ? <span className="run-graph-node-affordance">Open list</span> : null}
      {onStartConnect ? (
        <button
          type="button"
          className={`run-graph-start-link nodrag nopan${connecting ? " is-active" : ""}`}
          aria-label={`Connect ${label.title}`}
          title="Connect this node"
          onClick={(event) => {
            event.stopPropagation();
            onStartConnect(graphNode.id);
          }}
        >
          {connecting ? "Connecting…" : "Connect"}
        </button>
      ) : null}
      <Handle type="source" position={Position.Right} className="run-graph-handle" />
    </article>
  );
}
