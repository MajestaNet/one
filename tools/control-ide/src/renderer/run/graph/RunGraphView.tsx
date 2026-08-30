import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  useNodesState,
  type Edge,
  type Node,
  type ReactFlowInstance,
  type Viewport,
} from "@xyflow/react";
import { useEffect, useMemo, useState } from "react";
import "@xyflow/react/dist/style.css";
import { useTheme } from "../../ThemeContext";
import { RunGraphNodeCard, type RunGraphFlowNodeData } from "./nodes/RunGraphNodeCard";
import type { RunGraphResolveResult } from "./types";
import type { RunGraphViewDocument } from "./lenses";
import {
  OPERATE_TOOL_DRAG_MIME,
  parseOperateToolDragPayload,
  type OperateToolDragPayload,
} from "../../workspace/operateToolDrag";
import { runGraphEdgeLabel } from "./labels";

const nodeTypes = { runGraphNode: RunGraphNodeCard };
const EMPTY_SELECTED_NODE_IDS: string[] = [];

export function RunGraphView({
  view,
  resolved,
  selectedNodeIds = EMPTY_SELECTED_NODE_IDS,
  selectedEdgeId,
  onSelectedNodeIdsChange,
  onSelectedEdgeIdChange,
  onConnectNodes,
  onNodePositionChange,
  onStartConnect,
  connectSourceId,
  pulseNodeId,
  toolLabels = {},
  onDropTool,
  viewport,
  onViewportChange,
}: {
  view: RunGraphViewDocument;
  resolved: Record<string, RunGraphResolveResult>;
  selectedNodeIds?: string[];
  selectedEdgeId?: string | null;
  onSelectedNodeIdsChange?: (nodeIds: string[]) => void;
  onSelectedEdgeIdChange?: (edgeId: string | null) => void;
  onConnectNodes?: (sourceId: string, targetId: string) => void;
  onNodePositionChange?: (nodeId: string, position: { x: number; y: number }) => void;
  onStartConnect?: (nodeId: string) => void;
  connectSourceId?: string | null;
  pulseNodeId?: string | null;
  toolLabels?: Record<string, string>;
  onDropTool?: (
    tool: OperateToolDragPayload,
    position: { x: number; y: number },
    targetNodeId?: string,
  ) => void;
  viewport?: Viewport;
  onViewportChange?: (viewport: Viewport) => void;
}) {
  const theme = useTheme();
  const [flow, setFlow] = useState<ReactFlowInstance<Node<RunGraphFlowNodeData>, Edge> | null>(null);
  const selected = useMemo(() => new Set(selectedNodeIds), [selectedNodeIds]);
  const mappedNodes: Node<RunGraphFlowNodeData>[] = useMemo(
    () =>
      view.nodes.map((graphNode, index) => ({
        id: graphNode.id,
        type: "runGraphNode",
        position: graphNode.layout ?? {
          x: 48 + (index % 4) * 250,
          y: 48 + Math.floor(index / 4) * 170,
        },
        style: graphNode.layout?.w ? { width: graphNode.layout.w } : undefined,
        className: [
          graphNode.id === connectSourceId ? "is-connect-source" : "",
          graphNode.id === pulseNodeId ? "is-pulsing" : "",
        ].filter(Boolean).join(" "),
        data: {
          graphNode,
          resolve: resolved[graphNode.id],
          toolLabel: toolLabels[graphNode.id],
          onStartConnect,
          connecting: graphNode.id === connectSourceId,
        },
        selected: selected.has(graphNode.id),
        draggable: true,
      })),
    [connectSourceId, onStartConnect, pulseNodeId, resolved, selected, toolLabels, view.nodes],
  );
  const [nodes, setNodes, onNodesChange] = useNodesState<Node<RunGraphFlowNodeData>>(mappedNodes);
  useEffect(() => {
    setNodes(mappedNodes);
  }, [mappedNodes, setNodes]);
  useEffect(() => {
    if (!flow || !pulseNodeId) return;
    void flow.fitView({
      nodes: [{ id: pulseNodeId }],
      duration: 320,
      padding: 0.5,
      maxZoom: 1.1,
    });
  }, [flow, pulseNodeId]);
  const edges: Edge[] = useMemo(
    () =>
      view.edges.map((edge) => ({
        id: edge.id,
        source: edge.from,
        target: edge.to,
        label: runGraphEdgeLabel(edge.kind),
        animated: edge.kind === "next" || edge.kind === "watches",
        className: `run-graph-edge run-graph-edge-${edge.kind}`,
        selected: edge.id === selectedEdgeId,
      })),
    [selectedEdgeId, view.edges],
  );

  return (
    <div
      className={`run-graph-flow${connectSourceId ? " is-connecting" : ""}`}
      data-testid="run-graph-view"
      tabIndex={0}
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        onSelectedNodeIdsChange?.([]);
        onSelectedEdgeIdChange?.(null);
      }}
    >
      <ReactFlow
        colorMode={theme}
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        nodeTypes={nodeTypes}
        fitView={!viewport}
        defaultViewport={viewport}
        minZoom={0.35}
        maxZoom={1.8}
        nodesDraggable
        nodesConnectable
        elementsSelectable
        onInit={setFlow}
        onNodeClick={(event, node) => {
          onSelectedEdgeIdChange?.(null);
          if (connectSourceId && connectSourceId !== node.id) {
            onConnectNodes?.(connectSourceId, node.id);
            return;
          }
          if (event.shiftKey || event.metaKey || event.ctrlKey) {
            const next = selectedNodeIds.filter((id) => id !== node.id);
            onSelectedNodeIdsChange?.(
              selectedNodeIds.includes(node.id) ? next : [...next, node.id],
            );
            return;
          }
          onSelectedNodeIdsChange?.([node.id]);
        }}
        onNodeDragStop={(_, node) => onNodePositionChange?.(node.id, node.position)}
        onConnect={(connection) => {
          if (connection.source && connection.target) onConnectNodes?.(connection.source, connection.target);
        }}
        onEdgeClick={(_, edge) => {
          onSelectedNodeIdsChange?.([]);
          onSelectedEdgeIdChange?.(edge.id);
        }}
        onPaneClick={() => {
          onSelectedNodeIdsChange?.([]);
          onSelectedEdgeIdChange?.(null);
        }}
        onDragOver={(event) => {
          if (!event.dataTransfer.types.includes(OPERATE_TOOL_DRAG_MIME)) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = "copy";
        }}
        onDrop={(event) => {
          const payload = parseOperateToolDragPayload(
            event.dataTransfer.getData(OPERATE_TOOL_DRAG_MIME),
          );
          if (!payload || !flow) return;
          event.preventDefault();
          const target = event.target instanceof Element
            ? event.target.closest<HTMLElement>(".react-flow__node")?.dataset.id
            : undefined;
          onDropTool?.(
            payload,
            flow.screenToFlowPosition({ x: event.clientX, y: event.clientY }),
            target,
          );
        }}
        onMoveEnd={(_, nextViewport) => onViewportChange?.(nextViewport)}
        panOnDrag={[1, 2]}
        zoomOnScroll={false}
        zoomOnPinch
        zoomOnDoubleClick={false}
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="var(--line)" gap={20} />
        <Controls showInteractive={false} />
        <MiniMap
          bgColor="var(--surface-elevated)"
          maskColor="color-mix(in srgb, var(--bg-deep) 72%, transparent)"
          nodeColor="var(--muted)"
          pannable={false}
          zoomable={false}
        />
      </ReactFlow>
    </div>
  );
}
