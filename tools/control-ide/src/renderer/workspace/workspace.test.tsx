import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentChatPane } from "./AgentChatPane";
import { AgentStreamDock } from "./AgentStreamDock";
import { ChangeStatusBar } from "./ChangeStatusBar";
import { ModeLauncher } from "./ModeLauncher";
import { ModeRail } from "./ModeRail";
import { ModeSubnav } from "./ModeSubnav";
import { ModeTitle } from "./ModeTitle";
import { TileCanvas } from "./TileCanvas";
import { WorkspaceCanvas } from "./WorkspaceCanvas";
import { WorkspaceToolRail } from "./WorkspaceToolRail";
import {
  SEED_AGENT_CHATS,
  agentGridClass,
  agentsForMode,
  cycleTileLayout,
  defaultChatLayout,
  MAX_WORKSPACE_TILES,
  normalizeWorkspaceOrder,
  selectAgentTiles,
  selectToolTiles,
} from "./types";
import { makeChatTile, makePanelTile, defaultSliceRatios, formatSliceRatios } from "./WorkspaceCanvas";
import { RAIL_FLYOUT_PX } from "./workspaceRailLayout";

afterEach(() => cleanup());

describe("MAX_WORKSPACE_TILES", () => {
  it("cap is always 2 regardless of tile kinds", () => {
    expect(MAX_WORKSPACE_TILES).toBe(2);
  });
});

describe("selectToolTiles", () => {
  it("adds a tool on the left when a chat is already open", () => {
    const chat = makeChatTile("agent-query");
    const next = selectToolTiles([chat], makePanelTile("query"));
    expect(next).toHaveLength(2);
    expect(next[0]?.kind).toBe("panel");
    expect(next[0] && next[0].kind === "panel" && next[0].panelId).toBe("query");
    expect(next[1]?.kind).toBe("chat");
    expect(next[1] && next[1].kind === "chat" && next[1].chatId).toBe("agent-query");
  });

  it("swaps the existing tool when another tool is selected", () => {
    const chat = makeChatTile("agent-query");
    const withQuery = selectToolTiles([chat], makePanelTile("query"));
    const swapped = selectToolTiles(withQuery, makePanelTile("monitor"));
    expect(swapped).toHaveLength(2);
    expect(swapped[0] && swapped[0].kind === "panel" && swapped[0].panelId).toBe("monitor");
    expect(swapped.some((t) => t.kind === "panel" && t.panelId === "query")).toBe(false);
    expect(swapped[1]?.kind).toBe("chat");
  });

  it("is a no-op when the same tool is already open", () => {
    const prev = [makePanelTile("objects")];
    expect(selectToolTiles(prev, makePanelTile("objects"))).toBe(prev);
  });
});

describe("selectAgentTiles", () => {
  it("keeps tool left and places chat on the right", () => {
    const tool = makePanelTile("objects");
    const next = selectAgentTiles([tool], makeChatTile("agent-meta"));
    expect(next).toHaveLength(2);
    expect(next[0]?.kind).toBe("panel");
    expect(next[1]?.kind).toBe("chat");
    expect(next.some((t) => t.kind === "chat" && t.chatId === "agent-meta")).toBe(true);
  });

  it("swaps the existing agent when another agent is selected", () => {
    const tool = makePanelTile("objects");
    const withMeta = selectAgentTiles([tool], makeChatTile("agent-meta"));
    const swapped = selectAgentTiles(withMeta, makeChatTile("agent-deploy"));
    expect(swapped).toHaveLength(2);
    expect(swapped.some((t) => t.kind === "chat" && t.chatId === "agent-deploy")).toBe(true);
    expect(swapped.some((t) => t.kind === "chat" && t.chatId === "agent-meta")).toBe(false);
    expect(swapped.some((t) => t.kind === "panel" && t.panelId === "objects")).toBe(true);
  });

  it("is a no-op when the same agent chat is already open", () => {
    const prev = [makeChatTile("agent-meta")];
    expect(selectAgentTiles(prev, makeChatTile("agent-meta"))).toBe(prev);
  });
});

describe("normalizeWorkspaceOrder", () => {
  it("places tools left and chats right even when reversed", () => {
    const chat = makeChatTile("agent-query");
    const tool = makePanelTile("query");
    const ordered = normalizeWorkspaceOrder([chat, tool]);
    expect(ordered[0]?.kind).toBe("panel");
    expect(ordered[1]?.kind).toBe("chat");
  });
});

describe("defaultSliceRatios", () => {
  it("gives a lone tile the full board", () => {
    expect(defaultSliceRatios([makePanelTile("query")])).toEqual([1]);
    expect(defaultSliceRatios([makeChatTile("agent-meta")])).toEqual([1]);
  });

  it("defaults the agent to one-third when a tool is already open", () => {
    const ratios = defaultSliceRatios([makePanelTile("query"), makeChatTile("agent-meta")]);
    expect(ratios).toEqual([2 / 3, 1 / 3]);
    expect(formatSliceRatios(ratios)).toBe("0.667 0.333");
  });
});

describe("WorkspaceToolRail", () => {
  it("click-pins the catalog and notifies the workspace", async () => {
    const user = userEvent.setup();
    const onPinnedChange = vi.fn();
    render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query", summary: "Query records" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        onPinnedChange={onPinnedChange}
      />,
    );
    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(onPinnedChange).toHaveBeenCalledWith(true);
    expect(screen.getByTestId("workspace-tool-rail-flyout")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-pinned")).toBe("true");
    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(onPinnedChange).toHaveBeenCalledWith(false);
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-pinned")).toBe("false");
  });

  it("docks the catalog when forceExpanded even without hover", () => {
    render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query", summary: "Query records" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        forceExpanded
      />,
    );
    expect(screen.getByTestId("workspace-tool-rail-flyout")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-pinned")).toBe("false");
    expect(screen.getByTestId("tool-rail-query")).toBeTruthy();
  });

  it("keeps the same flyout width when hovering and when pinning", async () => {
    const user = userEvent.setup();
    render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
      />,
    );
    await user.hover(screen.getByTestId("workspace-tool-rail"));
    const hovered = await screen.findByTestId("workspace-tool-rail-flyout");
    expect(hovered.getAttribute("data-flyout-width")).toBe(String(RAIL_FLYOUT_PX));
    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("workspace-tool-rail-flyout").getAttribute("data-flyout-width")).toBe(
      String(RAIL_FLYOUT_PX),
    );
  });

  it("stays docked at the same width when forceExpanded while pin toggles", async () => {
    const user = userEvent.setup();
    const onPinnedChange = vi.fn();
    const { rerender } = render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        forceExpanded
        pinned={false}
        onPinnedChange={onPinnedChange}
      />,
    );
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("workspace-tool-rail-flyout").getAttribute("data-flyout-width")).toBe(
      String(RAIL_FLYOUT_PX),
    );
    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(onPinnedChange).toHaveBeenCalledWith(true);
    rerender(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        forceExpanded
        pinned
        onPinnedChange={onPinnedChange}
      />,
    );
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("workspace-tool-rail-flyout").getAttribute("data-flyout-width")).toBe(
      String(RAIL_FLYOUT_PX),
    );
  });

  it("unpins locally on Escape so the grid and rail stay in sync", async () => {
    const user = userEvent.setup();
    const onPinnedChange = vi.fn();
    render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        onPinnedChange={onPinnedChange}
      />,
    );
    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(screen.getByTestId("workspace-tool-rail").className).toMatch(/is-pinned/);
    await user.keyboard("{Escape}");
    expect(onPinnedChange).toHaveBeenCalledWith(false);
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-pinned")).toBe("false");
    expect(screen.getByTestId("workspace-tool-rail").className).not.toMatch(/is-pinned/);
  });

  it("collapses the hover overlay after selecting a tool", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <WorkspaceToolRail
        tools={[{ id: "query", label: "Query" }]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={onSelect}
      />,
    );
    await user.hover(screen.getByTestId("workspace-tool-rail"));
    expect(await screen.findByTestId("workspace-tool-rail-flyout")).toBeTruthy();
    await user.click(screen.getByTestId("tool-rail-query"));
    expect(onSelect).toHaveBeenCalledWith("query");
    expect(screen.queryByTestId("workspace-tool-rail-flyout")).toBeNull();
  });
});

describe("AgentChatPane", () => {
  it("sends trimmed text and clears the draft", async () => {
    const user = userEvent.setup();
    const onSend = vi.fn();
    render(<AgentChatPane chat={SEED_AGENT_CHATS[0]} onSend={onSend} />);
    const sendBtn = screen.getByRole("button", { name: /^Send$/i }) as HTMLButtonElement;
    expect(sendBtn.disabled).toBe(true);
    await user.type(screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i), "Rank my accounts");
    expect(sendBtn.disabled).toBe(false);
    await user.click(sendBtn);
    expect(onSend).toHaveBeenCalledWith("agent-query", "Rank my accounts", undefined);
    expect((screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i) as HTMLTextAreaElement).value).toBe("");
  });

  it("supports Enter to send from the single-line composer", async () => {
    const user = userEvent.setup();
    const onSend = vi.fn();
    render(<AgentChatPane chat={SEED_AGENT_CHATS[0]} onSend={onSend} />);
    await user.type(screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i), "hello{Enter}");
    expect(onSend).toHaveBeenCalledWith("agent-query", "hello", undefined);
  });

  it("does not render a pane-level close control", () => {
    render(<AgentChatPane chat={SEED_AGENT_CHATS[0]} onSend={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Close Query assistant/i })).toBeNull();
  });

  it("surfaces board handoff and approve from chat pane", async () => {
    const user = userEvent.setup();
    const onApprove = vi.fn();
    const chat = {
      ...SEED_AGENT_CHATS[0],
      messages: [
        ...SEED_AGENT_CHATS[0].messages,
        {
          id: "ap1",
          role: "approval" as const,
          body: "Approve before tools run",
          runId: "run-42",
          runStatus: "awaiting_approval",
          boardHandoff: {
            source: "approval_bundle" as const,
            objectApiName: "Account",
            recordIds: ["a1"],
            proposedMutations: [{ op: "update" as const, object: "Account", id: "a1", data: { Name: "Acme" } }],
          },
        },
      ],
    };
    render(
      <AgentChatPane
        chat={chat}
        onSend={vi.fn()}
        onApprove={onApprove}
      />,
    );
    expect(screen.getByTestId("chat-run-lifecycle").textContent).toMatch(/Awaiting approval/i);
    expect(screen.getByTestId("chat-handoff-block")).toBeTruthy();
    expect(screen.queryByTestId("crm-what-to-do")).toBeNull();
    expect(screen.queryByText(/Show matching records/i)).toBeNull();
    await user.click(screen.getByTestId("approve-run-inline"));
    expect(onApprove).toHaveBeenCalledWith("run-42");
  });

  it("shows typing lifecycle while busy", () => {
    render(
      <AgentChatPane chat={SEED_AGENT_CHATS[0]} onSend={vi.fn()} busy />,
    );
    expect(screen.getByTestId("chat-typing")).toBeTruthy();
    expect(screen.getByTestId("chat-run-lifecycle").textContent).toMatch(/Running/i);
  });

  it("keeps thinking inside the in-flight assistant bubble instead of a second row", () => {
    const chat = {
      ...SEED_AGENT_CHATS[0],
      messages: [
        ...SEED_AGENT_CHATS[0].messages,
        { id: "stream-1", role: "agent" as const, body: "", runStatus: "running" },
      ],
    };
    render(<AgentChatPane chat={chat} onSend={vi.fn()} busy />);
    expect(screen.getAllByTestId("chat-typing")).toHaveLength(1);
    expect(screen.getByTestId("stream-bubble-stream-1")).toBeTruthy();
  });

  it("does not show a thinking row once tokens are streaming", () => {
    const chat = {
      ...SEED_AGENT_CHATS[0],
      messages: [
        ...SEED_AGENT_CHATS[0].messages,
        { id: "stream-2", role: "agent" as const, body: "Partial reply", runStatus: "running" },
      ],
    };
    render(<AgentChatPane chat={chat} onSend={vi.fn()} busy />);
    expect(screen.queryByTestId("chat-typing")).toBeNull();
    expect(screen.getByText("Partial reply")).toBeTruthy();
  });

  it("pins a composer at the bottom of every agent chat", () => {
    render(<AgentChatPane chat={SEED_AGENT_CHATS[0]} onSend={vi.fn()} />);
    const pane = screen.getByTestId("agent-chat-pane-agent-query");
    const composer = screen.getByTestId("agent-chat-composer-agent-query");
    expect(pane.contains(composer)).toBe(true);
    expect(composer.className).toMatch(/agent-chat-pane-composer/);
    expect(screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i)).toBeTruthy();
    expect(screen.queryByText(/Bound ·/)).toBeNull();
  });
});

describe("ModeLauncher", () => {
  it("selects a mode from big tiles", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<ModeLauncher onSelect={onSelect} />);
    await user.click(screen.getByTestId("mode-launch-build"));
    expect(onSelect).toHaveBeenCalledWith("build");
  });

  it("includes Settings as the fourth tile in Operate/Build/Govern/Settings order", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<ModeLauncher onSelect={onSelect} />);
    const order = [
      "mode-launch-operate",
      "mode-launch-build",
      "mode-launch-govern",
      "mode-launch-settings",
    ];
    expect(screen.getAllByTestId(/mode-launch-/).map((el) => el.getAttribute("data-testid"))).toEqual(
      order,
    );
    expect(screen.getByTestId("brand-lockup")).toBeTruthy();
    expect(screen.getByRole("img", { name: /Majesta\.Net/i })).toBeTruthy();
    await user.click(screen.getByTestId("mode-launch-settings"));
    expect(onSelect).toHaveBeenCalledWith("settings");
  });

  it("hides Settings when allowAccount is false", () => {
    render(<ModeLauncher onSelect={vi.fn()} allowAccount={false} />);
    expect(screen.queryByTestId("mode-launch-settings")).toBeNull();
    expect(screen.getByTestId("mode-launch-govern")).toBeTruthy();
  });

  it("renders overlay dismiss", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(<ModeLauncher overlay onSelect={vi.fn()} onDismiss={onDismiss} />);
    expect(screen.getByTestId("mode-launcher-overlay")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Stay in current mode/i }));
    expect(onDismiss).toHaveBeenCalled();
  });
});

describe("ModeTitle", () => {
  it("toggles launcher on click", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<ModeTitle section="operate" launcherOpen={false} onToggleLauncher={onToggle} />);
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Operate/);
    await user.click(screen.getByTestId("mode-title"));
    expect(onToggle).toHaveBeenCalled();
  });

  it("shows Settings label for the settings section", () => {
    render(<ModeTitle section="settings" launcherOpen={false} onToggleLauncher={vi.fn()} />);
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Settings/);
  });
});

describe("ModeRail", () => {
  it("reports pressed mode without hint text and supports home", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    const onHome = vi.fn();
    render(<ModeRail mode="operate" onModeChange={onModeChange} onHome={onHome} />);
    const operate = screen.getByRole("button", { name: /^Operate$/i });
    expect(operate.getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: /^Build$/i }));
    expect(onModeChange).toHaveBeenCalledWith("build");
    await user.click(screen.getByTestId("mode-home"));
    expect(onHome).toHaveBeenCalled();
  });
});

describe("ModeSubnav", () => {
  it("lists build tools when operate tools are empty", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<ModeSubnav mode="build" active="objects" onSelect={onSelect} />);
    await user.click(screen.getByRole("button", { name: /^Objects$/i }));
    expect(onSelect).toHaveBeenCalledWith("objects");
  });
});

describe("TileCanvas", () => {
  it("selects and closes tiles", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(
      <TileCanvas
        openTiles={["client", "metadata"]}
        activeTile="client"
        onSelectTile={onSelect}
        onCloseTile={onClose}
      >
        <div>body</div>
      </TileCanvas>,
    );
    await user.click(screen.getByRole("tab", { name: "Metadata YAML" }));
    expect(onSelect).toHaveBeenCalledWith("metadata");
    await user.click(screen.getByRole("button", { name: /Close Metadata YAML/i }));
    expect(onClose).toHaveBeenCalledWith("metadata");
  });
});

describe("WorkspaceCanvas", () => {
  it("shows empty board prompting to add a tool or agent", () => {
    render(
      <WorkspaceCanvas
        mode="operate"
        tiles={[]}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={vi.fn()}
        onOpenChat={vi.fn()}
        onSendInChat={vi.fn()}
        renderPanel={() => null}
      />,
    );
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
    expect(screen.getByText(/Add a tool or Agent/i)).toBeTruthy();
    expect(screen.getByText(/Open a tool from the left rail/i)).toBeTruthy();
    expect(screen.queryByTestId("reopen-primary-chat")).toBeNull();
  });

  it("fills the board with a single slice and shows a label instead of layout cycle", () => {
    render(
      <WorkspaceCanvas
        mode="operate"
        tiles={[{ id: "chat-agent-query", kind: "chat", chatId: "agent-query", ...defaultChatLayout() }]}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={vi.fn()}
        onOpenChat={vi.fn()}
        onSendInChat={vi.fn()}
        renderPanel={() => null}
      />,
    );
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("1");
    expect(screen.getByTestId("resize-chat-agent-query").textContent).toMatch(/Chat/i);
    expect(screen.queryByTestId("workspace-tools")).toBeNull();
  });

  it("shows resize handles for two slices", () => {
    const two = [
      { id: "panel-objects", kind: "panel" as const, panelId: "objects" as const, colSpan: 2 as const, rowSpan: 2 as const },
      { id: "chat-agent-meta", kind: "chat" as const, chatId: "agent-meta", ...defaultChatLayout() },
    ];
    render(
      <WorkspaceCanvas
        mode="build"
        tiles={two}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={vi.fn()}
        onOpenChat={vi.fn()}
        onSendInChat={vi.fn()}
        renderPanel={(id) => <div data-testid={`panel-${id}`}>{id}</div>}
      />,
    );
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("2");
    expect(screen.getByTestId("workspace-split-handle")).toBeTruthy();
    expect(screen.getByTestId("workspace-slices").className).toMatch(/count-2/);
    expect(screen.getByTestId("workspace-slices").getAttribute("data-ratios")).toBe("0.667 0.333");
    expect(screen.getByTestId("workspace-slices").getAttribute("style")).toMatch(/0\.666/);
  });

  it("does not resize on pointer move until the gutter is clicked", () => {
    const two = [
      { id: "panel-objects", kind: "panel" as const, panelId: "objects" as const, colSpan: 2 as const, rowSpan: 2 as const },
      { id: "chat-agent-meta", kind: "chat" as const, chatId: "agent-meta", ...defaultChatLayout() },
    ];
    render(
      <WorkspaceCanvas
        mode="build"
        tiles={two}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={vi.fn()}
        onOpenChat={vi.fn()}
        onSendInChat={vi.fn()}
        renderPanel={() => null}
      />,
    );
    const slices = screen.getByTestId("workspace-slices");
    const rectSpy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      width: 300,
      height: 100,
      top: 0,
      left: 0,
      right: 300,
      bottom: 100,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    } as DOMRect);
    try {
      const handle = screen.getByTestId("workspace-split-handle");
      fireEvent.pointerMove(handle, { buttons: 0, clientX: 40, pointerId: 1 });
      expect(slices.getAttribute("data-ratios")).toBe("0.667 0.333");
      expect(screen.getByTestId("workspace-board").getAttribute("data-resizing")).toBe("false");

      fireEvent.pointerDown(handle, {
        button: 0,
        buttons: 1,
        clientX: 200,
        pointerId: 1,
        isPrimary: true,
        pointerType: "mouse",
      });
      expect(screen.getByTestId("workspace-board").getAttribute("data-resizing")).toBe("true");
      fireEvent.pointerMove(handle, { buttons: 1, clientX: 90, pointerId: 1 });
      expect(screen.getByTestId("workspace-slices").getAttribute("data-ratios")).not.toBe("0.667 0.333");
      fireEvent.pointerUp(handle, { button: 0, pointerId: 1 });
      expect(screen.getByTestId("workspace-board").getAttribute("data-resizing")).toBe("false");
    } finally {
      rectSpy.mockRestore();
    }
  });

  it("gives the personal graph an immersive canvas without generic tile chrome", () => {
    const onTilesChange = vi.fn();
    render(
      <WorkspaceCanvas
        mode="operate"
        tiles={[{ id: "panel-runGraph", kind: "panel", panelId: "runGraph", colSpan: 2, rowSpan: 2 }]}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={onTilesChange}
        onOpenChat={vi.fn()}
        onSendInChat={vi.fn()}
        renderPanel={() => <div data-testid="graph-body">graph</div>}
      />,
    );
    expect(screen.queryByTestId("close-tile-panel-runGraph")).toBeNull();
    expect(screen.getByTestId("workspace-tile-panel-runGraph").className).toMatch(/is-immersive/);
    expect(onTilesChange).not.toHaveBeenCalled();
  });

  it("does not show a workspace-cap notification at the board bottom", () => {
    const onOpen = vi.fn();
    render(
      <WorkspaceCanvas
        mode="build"
        tiles={[
          { id: "panel-deploy", kind: "panel", panelId: "deploy", colSpan: 2, rowSpan: 2 },
          { id: "chat-agent-deploy", kind: "chat", chatId: "agent-deploy", ...defaultChatLayout() },
        ]}
        catalog={SEED_AGENT_CHATS}
        onTilesChange={vi.fn()}
        onOpenChat={onOpen}
        onSendInChat={vi.fn()}
        renderPanel={() => null}
      />,
    );
    expect(screen.queryByText(/Maximum of 2 workspace panels/i)).toBeNull();
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("2");
  });
});

describe("WorkspaceToolRail", () => {
  it("shows a single Tools affordance and catalog cards with search", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(
      <WorkspaceToolRail
        tools={[
          { id: "objects", label: "Objects", summary: "Shape object metadata.", name: "objects" },
          { id: "packages", label: "Packages", summary: "Enable managed modules.", name: "packages" },
          { id: "agentSpecs", label: "Agents", summary: "Author AgentSpecs.", name: "agentSpecs" },
          { id: "repo", label: "Repo", summary: "Customer Git repo.", name: "repo" },
        ]}
        openToolIds={["objects"]}
        atCap={false}
        onSelectTool={onSelect}
        onCloseTool={onClose}
      />,
    );
    expect(screen.getByTestId("tool-rail-expand")).toBeTruthy();
    expect(screen.queryByTestId("tool-rail-icon-objects")).toBeNull();
    await user.hover(screen.getByTestId("workspace-tool-rail"));
    expect(await screen.findByTestId("tool-search")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-packages")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-agentSpecs")).toBeTruthy();
    expect(screen.getByText(/Shape object metadata/i)).toBeTruthy();
    expect(screen.getByText(/In workspace/i)).toBeTruthy();
    expect(screen.getByTestId("tool-rail-close-objects")).toBeTruthy();
    await user.click(screen.getByTestId("tool-rail-close-objects"));
    expect(onClose).toHaveBeenCalledWith("objects");
    await user.click(screen.getByTestId("tool-rail-agentSpecs"));
    expect(onSelect).toHaveBeenCalledWith("agentSpecs");
    fireEvent.pointerLeave(screen.getByTestId("workspace-tool-rail"));
    fireEvent.pointerEnter(screen.getByTestId("workspace-tool-rail"));

    const search = screen.getByLabelText(/Search tools by name/i);
    await user.clear(search);
    await user.type(search, "Packages");
    expect(screen.getByTestId("tool-rail-packages")).toBeTruthy();
    expect(screen.queryByTestId("tool-rail-agentSpecs")).toBeNull();
    await user.clear(search);
    await user.type(search, "zzzz-no-match");
    expect(screen.getByTestId("tool-rail-empty")).toBeTruthy();
  });

  it("shows a single title on catalog cards (no id kicker duplicate)", async () => {
    const user = userEvent.setup();
    render(
      <WorkspaceToolRail
        tools={[
          { id: "objects", label: "Objects", summary: "Shape object metadata.", name: "objects" },
          { id: "packages", label: "Packages", summary: "Enable managed modules." },
        ]}
        openToolIds={[]}
        atCap={false}
        onSelectTool={vi.fn()}
        forceExpanded
      />,
    );
    const objects = screen.getByTestId("tool-rail-objects");
    expect(objects.textContent?.match(/Objects/g)?.length).toBe(1);
    expect(objects.querySelector(".tool-catalog-kicker")).toBeNull();
    expect(screen.getByTestId("workspace-tool-rail-list").className).toMatch(/rail-scroll/);
    await user.hover(screen.getByTestId("workspace-tool-rail"));
  });
});

describe("AgentStreamDock", () => {
  it("filters to mode agents and searches by name after hover", async () => {
    const user = userEvent.setup();
    render(
      <AgentStreamDock
        changeTitle="Test change"
        changeStatus="draft"
        mode="operate"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        agentsDropEnabled
      />,
    );
    expect(screen.queryByTestId("stream-agent-cards")).toBeNull();
    await user.hover(screen.getByTestId("agent-stream"));
    expect(await screen.findByTestId("agent-drag-agent-run")).toBeTruthy();
    expect(screen.queryByTestId("agent-drag-agent-query")).toBeNull();
    const search = screen.getByLabelText(/Search agents by name/i);
    await user.clear(search);
    await user.type(search, "RunCoach");
    expect(screen.getByTestId("agent-drag-agent-run")).toBeTruthy();
    await user.clear(search);
    await user.type(search, "zzzz-no-match");
    expect(screen.queryByTestId("agent-drag-agent-run")).toBeNull();
    expect(screen.getByTestId("stream-agents-empty")).toBeTruthy();
  });

  it("shows a single agent title on catalog cards", async () => {
    const user = userEvent.setup();
    render(
      <AgentStreamDock
        changeTitle="x"
        changeStatus="draft"
        mode="build"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        onAttachAgent={vi.fn()}
      />,
    );
    await user.hover(screen.getByTestId("agent-stream"));
    const card = await screen.findByTestId("agent-drag-agent-meta");
    expect(card.textContent?.match(/Metadata builder/gi)?.length).toBe(1);
    expect(card.querySelector(".agent-drag-name")).toBeNull();
    expect(screen.getByTestId("stream-agent-list").className).toMatch(/rail-scroll/);
  });

  it("does not show an orange Demo badge on seed agents", async () => {
    const user = userEvent.setup();
    render(
      <AgentStreamDock
        changeTitle="x"
        changeStatus="draft"
        mode="build"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        onAttachAgent={vi.fn()}
      />,
    );
    await user.hover(screen.getByTestId("agent-stream"));
    expect(await screen.findByTestId("agent-drag-agent-query")).toBeTruthy();
    expect(screen.queryByTestId("agent-badge-agent-query")).toBeNull();
    expect(screen.queryByText(/^Demo$/)).toBeNull();
    expect(screen.getByText("Query assistant")).toBeTruthy();
  });

  it("opens flyout from collapsed rail on pointer hover", async () => {
    render(
      <AgentStreamDock
        changeTitle="x"
        changeStatus="draft"
        mode="build"
        agentChats={[]}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
      />,
    );
    const rail = screen.getByTestId("agent-stream");
    expect(screen.queryByTestId("agent-stream-flyout")).toBeNull();
    expect(rail.className).not.toMatch(/\bcollapsed\b/);
    fireEvent.pointerEnter(rail);
    expect(await screen.findByTestId("agent-stream-flyout")).toBeTruthy();
    expect(rail.className).toMatch(/\bis-open\b/);
    expect(rail.className).not.toMatch(/\bcollapsed\b/);
    expect(screen.getByTestId("agent-stream-expand").textContent).toContain("Agents");
    expect(screen.queryByTestId("stream-messages")).toBeNull();
    expect(screen.queryByLabelText(/Message agent/i)).toBeNull();
  });

  it("shows Build inspect catalog without Connect box when flyout is open", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <AgentStreamDock
        changeTitle="Change"
        changeStatus="draft"
        mode="build"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={["agent-query"]}
        onOpenTile={vi.fn()}
        onAttachAgent={vi.fn()}
        onCloseAgent={onClose}
        catalogOnly
        connected={false}
        onGoConnect={vi.fn()}
      />,
    );
    await user.hover(screen.getByTestId("agent-stream"));
    expect(await screen.findByTestId("agent-drag-agent-query")).toBeTruthy();
    expect(screen.getByTestId("agent-drag-agent-query").getAttribute("draggable")).toBeNull();
    expect(screen.getByText(/In workspace/i)).toBeTruthy();
    expect(screen.getByTestId("agent-close-agent-query")).toBeTruthy();
    await user.click(screen.getByTestId("agent-close-agent-query"));
    expect(onClose).toHaveBeenCalledWith("agent-query");
    expect(screen.queryByText(/Connect to message agents/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /Open Connect/i })).toBeNull();
  });

  it("shows BYO empty CTA when connected with no Operate agents", async () => {
    const user = userEvent.setup();
    render(
      <AgentStreamDock
        changeTitle="Change"
        changeStatus="draft"
        mode="operate"
        agentChats={SEED_AGENT_CHATS.filter((c) => !c.modes.includes("operate"))}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        catalogOnly
        connected
        onGoConnect={vi.fn()}
      />,
    );
    await user.hover(screen.getByTestId("agent-stream"));
    expect(await screen.findByTestId("stream-agents-byo-empty-operate")).toBeTruthy();
    expect(screen.getByText(/No Operate agents on this install/i)).toBeTruthy();
  });

  it("click-pins the catalog and notifies the workspace", async () => {
    const user = userEvent.setup();
    const onPinnedChange = vi.fn();
    render(
      <AgentStreamDock
        changeTitle="Change"
        changeStatus="draft"
        mode="build"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        onPinnedChange={onPinnedChange}
      />,
    );
    await user.click(screen.getByTestId("agent-stream-expand"));
    expect(onPinnedChange).toHaveBeenCalledWith(true);
    expect(screen.getByTestId("agent-stream-flyout")).toBeTruthy();
    expect(screen.getByTestId("agent-stream").getAttribute("data-docked")).toBe("true");
    await user.click(screen.getByTestId("agent-stream-expand"));
    expect(onPinnedChange).toHaveBeenCalledWith(false);
    expect(screen.getByTestId("agent-stream").getAttribute("data-pinned")).toBe("false");
  });

  it("docks the catalog when forceExpanded even without hover", () => {
    render(
      <AgentStreamDock
        changeTitle="Change"
        changeStatus="draft"
        mode="build"
        agentChats={SEED_AGENT_CHATS}
        pinnedChatIds={[]}
        onOpenTile={vi.fn()}
        forceExpanded
      />,
    );
    expect(screen.getByTestId("agent-stream-flyout")).toBeTruthy();
    expect(screen.getByTestId("agent-stream").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("agent-drag-agent-query")).toBeTruthy();
  });
});

describe("ChangeStatusBar", () => {
  it("renders a compact single-row check strip without stacked groups", () => {
    const { container } = render(
      <ChangeStatusBar
        status="running"
        checks={[
          { id: "1", label: "CI / Control IDE", state: "running", duration: "1m" },
          { id: "2", label: "Agent plan", state: "passed" },
          { id: "3", label: "Policy gate", state: "pending" },
        ]}
      />,
    );
    expect(screen.getByText("Running checks")).toBeTruthy();
    expect(screen.getByText("Policy gate")).toBeTruthy();
    expect(screen.getByText(/1 pending/i)).toBeTruthy();
    expect(screen.getByTestId("change-checks").querySelectorAll(".change-check-chip")).toHaveLength(3);
    expect(container.querySelector(".check-group")).toBeNull();
    expect(container.querySelector(".check-group-title")).toBeNull();
    expect(screen.queryByRole("button", { name: /Mark as Ready/i })).toBeNull();
  });

  it("hides checks when showChecks is false and has no footer Settings control", () => {
    render(
      <ChangeStatusBar
        status="draft"
        checks={[{ id: "c1", label: "Policy gate", state: "pending" }]}
        showChecks={false}
      />,
    );
    expect(screen.getByText("Draft")).toBeTruthy();
    expect(screen.queryByTestId("change-checks")).toBeNull();
    expect(screen.queryByText("Policy gate")).toBeNull();
    expect(screen.queryByTestId("open-settings")).toBeNull();
    expect(screen.getByTestId("change-status-bar")).toBeTruthy();
  });

  it("shows Applied after a proposal commits", () => {
    render(<ChangeStatusBar status="applied" checks={[]} />);
    expect(screen.getByText("Applied")).toBeTruthy();
    expect(screen.getByText("Applied").className).toMatch(/tone-success/);
  });
});

describe("helpers", () => {
  it("filters agents by mode and query", () => {
    expect(agentsForMode(SEED_AGENT_CHATS, "build").map((a) => a.id)).toEqual([
      "agent-query",
      "agent-records",
      "agent-meta",
      "agent-deploy",
    ]);
    expect(agentsForMode(SEED_AGENT_CHATS, "operate").map((a) => a.id)).toEqual(["agent-run"]);
    expect(agentsForMode([{ ...SEED_AGENT_CHATS[0], modes: ["operate"] }], "build").map((a) => a.id)).toEqual([]);
    expect(agentsForMode([{ ...SEED_AGENT_CHATS[0], modes: ["operate"] }], "operate").map((a) => a.id)).toEqual([
      "agent-query",
    ]);
    expect(agentsForMode(SEED_AGENT_CHATS, "build", "deploy")[0]?.id).toBe("agent-deploy");
    expect(agentsForMode(SEED_AGENT_CHATS, "build", "QueryAssistant").map((a) => a.id)).toEqual([
      "agent-query",
    ]);
    expect(agentsForMode(SEED_AGENT_CHATS, "build", "RecordsAssistant").map((a) => a.id)).toEqual([
      "agent-records",
    ]);
    expect(agentsForMode(SEED_AGENT_CHATS, "build", "nope")).toEqual([]);
    expect(agentsForMode(SEED_AGENT_CHATS, "settings").map((a) => a.id)).toEqual(["agent-account"]);
  });

  it("cycles layout spans", () => {
    const tall = { id: "t", kind: "chat" as const, chatId: "x", colSpan: 1 as const, rowSpan: 2 as const };
    expect(cycleTileLayout(tall)).toMatchObject({ colSpan: 2, rowSpan: 1 });
  });

  it("maps counts to layout classes", () => {
    expect(agentGridClass(1)).toBe("agent-grid count-1");
    expect(agentGridClass(4)).toBe("agent-grid count-4");
  });
});
