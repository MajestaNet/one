import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { useMemo } from "react";
import { ToolNodeView } from "../ToolNodeView";
import type { ActionChip, ToolDocument, ToolNode } from "../types";
import "@xyflow/react/dist/style.css";

type ToolFlowNodeData = {
  toolNode: ToolNode;
  selected: boolean;
  onSelect?: (nodeId: string) => void;
  onEnqueuePrompt?: (prompt: string, node: ToolNode) => void;
  onInvokeAutomation?: (chip: ActionChip, node: ToolNode) => void;
  busyChipKey?: string | null;
  onAddExcerptToChat?: (excerpt: import("../../workspace/contextExcerpt").ContextExcerpt) => void;
};

function ToolFlowNode({ data }: NodeProps<Node<ToolFlowNodeData>>) {
  return (
    <div
      className="run-tool-flow-node nodrag nopan nowheel"
      style={{ width: "100%", minWidth: 180 }}
      data-testid={`run-tool-flow-node-${data.toolNode.id}`}
    >
      <ToolNodeView
        node={data.toolNode}
        selected={data.selected}
        onSelect={() => data.onSelect?.(data.toolNode.id)}
        onEnqueuePrompt={data.onEnqueuePrompt}
        onInvokeAutomation={data.onInvokeAutomation}
        busyChipKey={data.busyChipKey}
        onAddExcerptToChat={data.onAddExcerptToChat}
      />
    </div>
  );
}

const nodeTypes = { toolNode: ToolFlowNode };

/** React Flow shell for spatial Tool layout (ADR-021 Phase 3 + P0 interaction guards). */
export function ToolSpatialView({
  document,
  selectedNodeIds = [],
  onSelectNode,
  onEnqueuePrompt,
  onInvokeAutomation,
  busyChipKey,
  onAddExcerptToChat,
}: {
  document: ToolDocument;
  selectedNodeIds?: string[];
  onSelectNode?: (nodeId: string) => void;
  onEnqueuePrompt?: (prompt: string, node: ToolNode) => void;
  onInvokeAutomation?: (chip: ActionChip, node: ToolNode) => void;
  busyChipKey?: string | null;
  onAddExcerptToChat?: (excerpt: import("../../workspace/contextExcerpt").ContextExcerpt) => void;
}) {
  const selectedKey = selectedNodeIds.join(",");
  const nodes: Node<ToolFlowNodeData>[] = useMemo(() => {
    const positions = document.layout.positions ?? {};
    const selected = new Set(selectedKey ? selectedKey.split(",") : []);
    return document.nodes.map((toolNode, index) => {
      const pos = positions[toolNode.id] ?? {
        x: 40 + (index % 3) * 260,
        y: 40 + Math.floor(index / 3) * 200,
      };
      return {
        id: toolNode.id,
        type: "toolNode",
        position: { x: pos.x, y: pos.y },
        style: pos.w ? { width: pos.w } : undefined,
        data: {
          toolNode,
          selected: selected.has(toolNode.id),
          onSelect: onSelectNode,
          onEnqueuePrompt,
          onInvokeAutomation,
          busyChipKey,
          onAddExcerptToChat,
        },
        draggable: false,
        selectable: true,
      };
    });
  }, [document.nodes, document.layout.positions, selectedKey, onSelectNode, onEnqueuePrompt, onInvokeAutomation, busyChipKey, onAddExcerptToChat]);

  return (
    <div className="run-tool-spatial" data-testid="canvas-spatial-viewport">
      <ReactFlow
        nodes={nodes}
        edges={[]}
        nodeTypes={nodeTypes}
        fitView
        minZoom={0.4}
        maxZoom={1.5}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        panOnDrag={[1, 2]}
        zoomOnScroll={false}
        zoomOnPinch
        zoomOnDoubleClick={false}
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        <MiniMap pannable={false} zoomable={false} />
      </ReactFlow>
    </div>
  );
}
