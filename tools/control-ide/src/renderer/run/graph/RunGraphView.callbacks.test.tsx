import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RunGraphView } from "./RunGraphView";
import type { RunGraphViewDocument } from "./lenses";

const flowSpies = vi.hoisted(() => ({
  fitView: vi.fn(),
  screenToFlowPosition: vi.fn(() => ({ x: 42, y: 84 })),
}));

vi.mock("@xyflow/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@xyflow/react")>();
  type FlowProps = {
    children?: ReactNode;
    colorMode?: string;
    onNodeClick?: (event: unknown, node: { id: string; position: { x: number; y: number } }) => void;
    onNodeDragStop?: (event: unknown, node: { id: string; position: { x: number; y: number } }) => void;
    onConnect?: (connection: { source: string | null; target: string | null }) => void;
    onEdgeClick?: (event: unknown, edge: { id: string }) => void;
    onPaneClick?: () => void;
    onMoveEnd?: (event: unknown, viewport: { x: number; y: number; zoom: number }) => void;
    onInit?: (instance: typeof flowSpies) => void;
    onDrop?: (event: unknown) => void;
  };

  return {
    ...actual,
    Background: () => null,
    Controls: () => null,
    MiniMap: () => null,
    ReactFlow: ({
      children,
      colorMode,
      onNodeClick,
      onNodeDragStop,
      onConnect,
      onEdgeClick,
      onPaneClick,
      onMoveEnd,
      onInit,
      onDrop,
    }: FlowProps) => (
      <div data-testid="mock-flow" data-color-mode={colorMode}>
        {children}
        <button onClick={() => onNodeClick?.({}, { id: "one", position: { x: 10, y: 20 } })}>Node</button>
        <button onClick={() => onNodeDragStop?.({}, { id: "one", position: { x: 30, y: 40 } })}>Drag</button>
        <button onClick={() => onConnect?.({ source: "one", target: "two" })}>Connect</button>
        <button onClick={() => onConnect?.({ source: "one", target: null })}>Incomplete connect</button>
        <button onClick={() => onEdgeClick?.({}, { id: "edge-one-two" })}>Edge</button>
        <button onClick={() => onPaneClick?.()}>Pane</button>
        <button onClick={() => onMoveEnd?.({}, { x: 5, y: 6, zoom: 0.75 })}>Move</button>
        <button onClick={() => onInit?.(flowSpies)}>Init flow</button>
        <button onClick={() => onDrop?.({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
          target: document.createElement("div"),
          dataTransfer: {
            getData: () => JSON.stringify({
              type: "operate-tool",
              railId: "tool:AccountBrief",
              label: "Account brief",
              toolSpecApiName: "AccountBrief",
            }),
          },
        })}>Drop Tool</button>
      </div>
    ),
    useNodesState: <T,>(initial: T[]) => [initial, vi.fn(), vi.fn()],
  };
});

const view: RunGraphViewDocument = {
  nodes: [
    { id: "one", kind: "insight", text: "One" },
    { id: "two", kind: "question", text: "Two" },
  ],
  edges: [{ id: "edge-one-two", from: "one", to: "two", kind: "related" }],
};

afterEach(() => {
  cleanup();
  flowSpies.fitView.mockClear();
  flowSpies.screenToFlowPosition.mockClear();
});

describe("RunGraphView interactions", () => {
  it("forwards selection, drag, connect, edge, pane, and viewport events", () => {
    const onSelectedNodeIdsChange = vi.fn();
    const onSelectedEdgeIdChange = vi.fn();
    const onConnectNodes = vi.fn();
    const onNodePositionChange = vi.fn();
    const onViewportChange = vi.fn();
    const props = {
      view,
      resolved: {},
      onSelectedNodeIdsChange,
      onSelectedEdgeIdChange,
      onConnectNodes,
      onNodePositionChange,
      onViewportChange,
    };
    const { rerender } = render(<RunGraphView {...props} />);

    fireEvent.click(screen.getByText("Node"));
    expect(onSelectedEdgeIdChange).toHaveBeenLastCalledWith(null);
    expect(onSelectedNodeIdsChange).toHaveBeenLastCalledWith(["one"]);

    rerender(<RunGraphView {...props} selectedNodeIds={["one"]} />);
    fireEvent.click(screen.getByText("Node"));
    expect(onSelectedNodeIdsChange).toHaveBeenLastCalledWith(["one"]);

    fireEvent.click(screen.getByText("Drag"));
    expect(onNodePositionChange).toHaveBeenCalledWith("one", { x: 30, y: 40 });

    fireEvent.click(screen.getByText("Connect"));
    fireEvent.click(screen.getByText("Incomplete connect"));
    expect(onConnectNodes).toHaveBeenCalledTimes(1);
    expect(onConnectNodes).toHaveBeenCalledWith("one", "two");

    fireEvent.click(screen.getByText("Edge"));
    expect(onSelectedNodeIdsChange).toHaveBeenLastCalledWith([]);
    expect(onSelectedEdgeIdChange).toHaveBeenLastCalledWith("edge-one-two");

    fireEvent.click(screen.getByText("Pane"));
    expect(onSelectedNodeIdsChange).toHaveBeenLastCalledWith([]);
    expect(onSelectedEdgeIdChange).toHaveBeenLastCalledWith(null);

    fireEvent.click(screen.getByText("Move"));
    expect(onViewportChange).toHaveBeenCalledWith({ x: 5, y: 6, zoom: 0.75 });
  });

  it("translates a Tool drop onto the canvas and brings pulsed nodes into view", async () => {
    const onDropTool = vi.fn();
    const { rerender } = render(
      <RunGraphView view={view} resolved={{}} onDropTool={onDropTool} />,
    );
    fireEvent.click(screen.getByText("Init flow"));
    fireEvent.click(screen.getByText("Drop Tool"));
    expect(onDropTool).toHaveBeenCalledWith(
      expect.objectContaining({ railId: "tool:AccountBrief", toolSpecApiName: "AccountBrief" }),
      { x: 42, y: 84 },
      undefined,
    );

    rerender(<RunGraphView view={view} resolved={{}} onDropTool={onDropTool} pulseNodeId="two" />);
    await waitFor(() => expect(flowSpies.fitView).toHaveBeenCalledWith(expect.objectContaining({
      nodes: [{ id: "two" }],
      duration: 320,
    })));
  });
});
