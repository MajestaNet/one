import { cleanup, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeContext } from "../../ThemeContext";
import { RunGraphView } from "./RunGraphView";
import type { RunGraphViewDocument } from "./lenses";

vi.mock("@xyflow/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@xyflow/react")>();
  return {
    ...actual,
    Background: () => null,
    Controls: () => null,
    MiniMap: () => null,
    ReactFlow: ({ children, colorMode }: { children?: ReactNode; colorMode?: string }) => (
      <div data-testid="mock-flow" data-color-mode={colorMode}>
        {children}
      </div>
    ),
    useNodesState: <T,>(initial: T[]) => [initial, vi.fn(), vi.fn()],
  };
});

const view: RunGraphViewDocument = {
  nodes: [],
  edges: [],
};

afterEach(() => {
  cleanup();
});

describe("RunGraphView", () => {
  it("passes the active IDE theme to React Flow", async () => {
    const { rerender } = render(
      <ThemeContext.Provider value="dark">
        <RunGraphView view={view} resolved={{}} />
      </ThemeContext.Provider>,
    );

    expect(screen.getByTestId("mock-flow").getAttribute("data-color-mode")).toBe("dark");

    rerender(
      <ThemeContext.Provider value="light">
        <RunGraphView view={view} resolved={{}} />
      </ThemeContext.Provider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("mock-flow").getAttribute("data-color-mode")).toBe("light");
    });
  });
});
