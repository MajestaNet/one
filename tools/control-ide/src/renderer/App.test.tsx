import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { THEME_STORAGE_KEY } from "./theme";
import { PRIMARY_OPERATE_CHAT_ID } from "./workspace/types";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  Reflect.deleteProperty(window, "one");
  window.localStorage.removeItem(THEME_STORAGE_KEY);
  document.documentElement.removeAttribute("data-theme");
});

beforeEach(() => {
  window.localStorage.removeItem(THEME_STORAGE_KEY);
  mockRunGraphAPI();
});

function mockConnectedSession(overrides?: Record<string, unknown>) {
  window.one = {
    getSession: vi.fn().mockResolvedValue({
      baseUrl: "http://localhost:8080",
      token: "jwt",
      scopes: ["client", "metadata", "deploy"],
      activeInstallId: "local",
      environments: [
        {
          installId: "local",
          installRole: "local",
          baseUrl: "http://localhost:8080",
          token: "jwt",
          scopes: ["client", "metadata", "deploy"],
          displayName: "Ada Lovelace",
          email: "ada@example.com",
        },
      ],
      ...overrides,
    }),
    setSession: vi.fn().mockResolvedValue({ ok: true }),
    gitStatus: vi.fn(),
    listTree: vi.fn(),
    readText: vi.fn(),
    writeText: vi.fn(),
    getUpdateStatus: vi.fn().mockResolvedValue({
      state: "disabled",
      message: "Updates disabled until UPDATE_FEED_URL is configured (see ADR-030).",
    }),
  };
}

const DEMO_PLAYBOOKS = [
  { apiName: "QueryAssistant", label: "Query assistant", active: true, primarySection: "build" },
  { apiName: "RecordsAssistant", label: "Records assistant", active: true, primarySection: "build" },
  { apiName: "RunCoach", label: "Run coach", active: true, primarySection: "run", harnessId: "harness.run.coach" },
  { apiName: "MetadataBuilder", label: "Metadata builder", active: true, primarySection: "build" },
  { apiName: "DeployBot", label: "Deploy guide", active: true, primarySection: "build" },
  { apiName: "AdminSetup", label: "Admin setup", active: true, primarySection: "govern" },
  { apiName: "AccountGuide", label: "Account guide", active: true, primarySection: "settings" },
];

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function mockRunGraphAPI() {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.includes("/client/v1/me")) {
      return jsonResponse({
        scopes: ["client", "metadata", "deploy"],
        isAdmin: true,
        displayName: "Ada Lovelace",
        email: "ada@example.com",
      });
    }
    if (url.includes("/client/v1/agents/playbooks")) {
      return jsonResponse({ playbooks: DEMO_PLAYBOOKS });
    }
    if (url.endsWith("/client/v1/run-graphs/home")) {
      return new Response(
        JSON.stringify({
          id: "home",
          graphKey: "home",
          title: "My graph",
          revision: 1,
          document: {
            apiVersion: "one.runGraph/v1",
            id: "home",
            title: "My graph",
            nodes: [
              {
                id: "account-1",
                kind: "record",
                ref: { objectApiName: "Account", recordId: "00000000-0000-4000-8000-000000000001" },
              },
            ],
            edges: [],
          },
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      );
    }
    if (url.endsWith("/client/v1/run-graphs/resolve")) {
      return new Response(
        JSON.stringify({ nodes: [{ nodeId: "account-1", ok: false, code: "NOT_FOUND" }] }),
        { status: 200, headers: { "content-type": "application/json" } },
      );
    }
    return new Response(JSON.stringify({}), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  });
}

async function enterBuild(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByTestId("mode-launcher");
  await user.click(screen.getByTestId("mode-launch-build"));
  await screen.findByTestId("workspace-canvas");
}

async function openAgentFlyout() {
  const rail = screen.getByTestId("agent-stream");
  fireEvent.pointerLeave(rail);
  fireEvent.pointerEnter(rail);
  await screen.findByTestId("stream-agent-cards");
}

async function openToolFlyout() {
  const rail = screen.getByTestId("workspace-tool-rail");
  fireEvent.pointerLeave(rail);
  fireEvent.pointerEnter(rail);
  await screen.findByTestId("workspace-tool-rail-flyout");
}

describe("App", () => {
  it("shows the auth screen when not connected — no tiles or workspace actions", async () => {
    render(<App />);
    expect(await screen.findByTestId("auth-screen")).toBeTruthy();
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Sign in/);
    expect(screen.getByText(/unlock Operate, Build, Govern, and Settings/)).toBeTruthy();
    expect(screen.getByTestId("connect-section")).toBeTruthy();
    expect(screen.getByTestId("connect-sign-in")).toBeTruthy();
    expect(screen.queryByTestId("mode-launcher")).toBeNull();
    expect(screen.queryByTestId("mode-launch-operate")).toBeNull();
    expect(screen.queryByTestId("mode-launch-settings")).toBeNull();
    expect(screen.queryByTestId("open-settings")).toBeNull();
    expect(screen.queryByTestId("workspace-tool-rail")).toBeNull();
    expect(screen.queryByTestId("agent-stream")).toBeNull();
    expect(screen.getByTestId("session-chip").textContent).toMatch(/Not connected/);
    expect(screen.queryByTestId("operate-global-search")).toBeNull();
  });

  it("shows four mode tiles after connect, including Settings (no footer Settings)", async () => {
    mockConnectedSession();
    render(<App />);
    expect(await screen.findByTestId("mode-launcher")).toBeTruthy();
    expect(screen.queryByTestId("auth-screen")).toBeNull();
    const order = [
      "mode-launch-operate",
      "mode-launch-build",
      "mode-launch-govern",
      "mode-launch-settings",
    ];
    for (const id of order) {
      expect(screen.getByTestId(id)).toBeTruthy();
    }
    const cards = screen.getAllByTestId(/mode-launch-/);
    expect(cards.map((el) => el.getAttribute("data-testid"))).toEqual(order);
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Choose a mode/);
    expect(screen.queryByTestId("open-settings")).toBeNull();
    expect(screen.queryByTestId("mode-subnav")).toBeNull();
    expect(screen.queryByTestId("mode-rail")).toBeNull();
  });

  it("enters Operate at the personal graph home and keeps the graph search in its command bar", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-operate"));
    await screen.findByTestId("workspace-canvas");
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Operate/);
    expect(await screen.findByTestId("operate-global-search")).toBeTruthy();
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();
    expect(await screen.findByTestId("run-graph-view")).toBeTruthy();
    expect(await screen.findByText("Record not found")).toBeTruthy();
    expect(screen.getByLabelText("Graph view")).toBeTruthy();
    expect(screen.getByTestId("operate-global-search").closest(".run-graph-command")).toBeTruthy();
    expect(screen.queryByTestId("workspace-empty")).toBeNull();
    expect(screen.getByTestId("workspace-tool-rail")).toBeTruthy();
    await openToolFlyout();
    expect(await screen.findByTestId("tool-rail-runGraph")).toBeTruthy();
    expect(await screen.findByTestId("tool-rail-objectHome")).toBeTruthy();
    expect(screen.queryByTestId("tool-rail-runDemo")).toBeNull();
    await user.click(screen.getByTestId("tool-rail-objectHome"));
    expect(await screen.findByTestId("run-object-home-panel")).toBeTruthy();
    expect(screen.queryByTestId("run-graph-home")).toBeNull();
    await openAgentFlyout();
    expect(await screen.findByTestId("agent-drag-agent-run")).toBeTruthy();
    expect(screen.queryByTestId("agent-drag-agent-query")).toBeNull();
  });

  it("keeps My graph immersive without redundant tile chrome", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();
    expect(screen.queryByTestId("close-tile-panel-runGraph")).toBeNull();
    expect(screen.getByTestId("workspace-tile-panel-runGraph").className).toMatch(/is-immersive/);
    expect(screen.getByTestId("run-graph-home")).toBeTruthy();
  });

  it("reopens My graph and focuses its command bar with Ctrl/Cmd+K", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();

    await openToolFlyout();
    await user.click(screen.getByRole("button", { name: "Remove My graph from workspace" }));
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();

    await user.keyboard("{Control>}k{/Control}");
    const search = await screen.findByTestId("operate-global-search");
    await waitFor(() => expect(document.activeElement).toBe(search));
    expect(screen.getByTestId("run-graph-home")).toBeTruthy();
  });

  it("closes the Settings default tool and leaves an empty workspace", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-settings"));
    expect(screen.getByTestId("account-settings-panel")).toBeTruthy();
    await user.click(screen.getByTestId("close-tile-panel-account"));
    expect(screen.queryByTestId("account-settings-panel")).toBeNull();
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
  });

  it("enters Build with inspect tools on an empty workspace", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Build/);
    expect(screen.queryByTestId("operate-global-search")).toBeNull();
    expect(screen.getByTestId("workspace-canvas")).toBeTruthy();
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
    expect(screen.queryByTestId(`agent-chat-pane-${PRIMARY_OPERATE_CHAT_ID}`)).toBeNull();
    expect(screen.getByTestId("workspace-tool-rail")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("agent-stream").getAttribute("data-docked")).toBe("true");
    expect(screen.getByTestId("tool-rail-query")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-monitor")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-explorer")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-automations")).toBeTruthy();
    expect(screen.getByTestId("stream-agent-cards")).toBeTruthy();
    expect(await screen.findByTestId("agent-drag-agent-query")).toBeTruthy();
    expect(screen.getByTestId("agent-drag-agent-records")).toBeTruthy();
    expect(screen.queryByTestId("agent-drag-agent-run")).toBeNull();
    expect(screen.getByTestId("agent-search")).toBeTruthy();
  });

  it("does not keep seed agent stubs when connected playbooks are empty", async () => {
    mockConnectedSession();
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes("/client/v1/me")) {
        return jsonResponse({
          scopes: ["client", "metadata", "deploy"],
          isAdmin: true,
        });
      }
      if (url.includes("/client/v1/agents/playbooks")) {
        return jsonResponse({ playbooks: [] });
      }
      if (url.endsWith("/client/v1/run-graphs/home")) {
        return jsonResponse({
          id: "home",
          graphKey: "home",
          title: "My graph",
          revision: 1,
          document: { apiVersion: "one.runGraph/v1", id: "home", title: "My graph", nodes: [], edges: [] },
        });
      }
      return jsonResponse({});
    });
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openAgentFlyout();
    await waitFor(() => {
      expect(screen.queryByTestId("agent-drag-agent-query")).toBeNull();
      expect(screen.queryByTestId("agent-drag-agent-records")).toBeNull();
      expect(screen.queryByTestId("agent-drag-agent-run")).toBeNull();
    });
  });

  it("keeps the 3-column workspace class when Operate graph is open and the agent rail hovers", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();
    const chrome = screen.getByTestId("workspace-chrome");
    expect(chrome.className.split(/\s+/)).toEqual(["workspace", "workspace-single"]);
    expect(chrome.getAttribute("data-tools-docked")).toBe("false");
    expect(chrome.getAttribute("data-agents-docked")).toBe("false");
    const agents = screen.getByTestId("agent-stream");
    expect(agents.className).not.toMatch(/\bcollapsed\b/);
    await user.hover(agents);
    expect(await screen.findByTestId("agent-stream-flyout")).toBeTruthy();
    expect(chrome.className.split(/\s+/)).toEqual(["workspace", "workspace-single"]);
    expect(agents.className).toMatch(/\bis-open\b/);
    expect(agents.className).not.toMatch(/\bcollapsed\b/);
  });

  it("retracts both rails after selecting a tool on an empty Build workspace", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("true");
    await user.click(screen.getByTestId("tool-rail-query"));
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail").getAttribute("data-docked")).toBe("false");
    expect(screen.getByTestId("agent-stream").getAttribute("data-docked")).toBe("false");
    expect(screen.queryByTestId("workspace-tool-rail-flyout")).toBeNull();
    expect(screen.queryByTestId("agent-stream-flyout")).toBeNull();
  });

  it("pins only the tools rail after retract and keeps the flyout width", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    const emptyToolsFlyout = screen.getByTestId("workspace-tool-rail-flyout");
    const emptyAgentsFlyout = screen.getByTestId("agent-stream-flyout");
    expect(emptyToolsFlyout.getAttribute("data-flyout-width")).toBe("240");
    expect(emptyAgentsFlyout.getAttribute("data-flyout-width")).toBe("240");
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-tools-docked")).toBe("true");
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-agents-docked")).toBe("true");

    await user.click(screen.getByTestId("tool-rail-query"));
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-tools-docked")).toBe("false");
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-agents-docked")).toBe("false");

    await user.click(screen.getByTestId("tool-rail-expand"));
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-tools-docked")).toBe("true");
    expect(screen.getByTestId("workspace-chrome").getAttribute("data-agents-docked")).toBe("false");
    expect(screen.getByTestId("workspace-tool-rail-flyout").getAttribute("data-flyout-width")).toBe("240");
    expect(screen.queryByTestId("agent-stream-flyout")).toBeNull();
  });

  it("loads session from Electron bridge when present and skips auth", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await waitFor(() => expect(screen.getByTestId("session-chip").textContent).toMatch(/Ada Lovelace/));
    expect(window.one.getSession).toHaveBeenCalled();
    expect(screen.queryByTestId("auth-screen")).toBeNull();
  });

  it("returns to sign in when a stored JWT is rejected", async () => {
    mockConnectedSession();
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => JSON.stringify({ error: "expired" }),
    } as Response);
    render(<App />);
    expect(await screen.findByTestId("auth-screen")).toBeTruthy();
    expect(window.one.setSession).toHaveBeenCalledWith(null);
    expect(screen.getByTestId("session-chip").textContent).toMatch(/Not connected/);
  });

  it("stays signed in when a stored JWT is rejected but refresh succeeds", async () => {
    const expiresAt = Date.now() + 3_600_000;
    mockConnectedSession({
      refreshToken: "rt-old",
      accessExpiresAt: expiresAt,
      environments: [
        {
          installId: "local",
          installRole: "local",
          baseUrl: "http://localhost:8080",
          token: "jwt",
          refreshToken: "rt-old",
          accessExpiresAt: expiresAt,
          displayName: "Ada Lovelace",
          email: "ada@example.com",
        },
      ],
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.includes("/auth/v1/token")) {
        return new Response(
          JSON.stringify({
            access_token: "jwt-new",
            refresh_token: "rt-new",
            expires_in: 3600,
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        );
      }
      const auth = new Headers(init?.headers).get("Authorization");
      if (url.includes("/client/v1/me") && auth === "Bearer jwt") {
        return new Response(JSON.stringify({ error: "expired" }), { status: 401 });
      }
      if (url.includes("/client/v1/me")) {
        return new Response(
          JSON.stringify({
            displayName: "Ada Lovelace",
            email: "ada@example.com",
            principalType: "user",
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        );
      }
      return new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
    });
    render(<App />);
    expect(await screen.findByTestId("mode-launcher")).toBeTruthy();
    expect(screen.queryByTestId("auth-screen")).toBeNull();
    await waitFor(() => {
      const payloads = vi.mocked(window.one.setSession).mock.calls.map((c) => c[0]);
      expect(payloads.some((s) => s && typeof s === "object" && s.token === "jwt-new")).toBe(true);
    });
    expect(window.one.setSession).not.toHaveBeenCalledWith(null);
    expect(screen.getByTestId("session-chip").textContent).toMatch(/Ada Lovelace/);
  });

  it("opens mode launcher overlay from centered mode title and switches mode", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await user.click(screen.getByTestId("mode-title"));
    expect(await screen.findByTestId("mode-launcher-overlay")).toBeTruthy();
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Operate/);
    await openAgentFlyout();
    expect(await screen.findByTestId("agent-drag-agent-run")).toBeTruthy();
    expect(screen.queryByTestId("agent-drag-agent-query")).toBeNull();
    expect(screen.queryByTestId("agent-drag-agent-deploy")).toBeNull();
  });

  it("opens Settings from the launcher tile (no footer Settings)", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    expect(screen.queryByTestId("open-settings")).toBeNull();
    await user.click(screen.getByTestId("mode-launch-settings"));
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Settings/);
    expect(screen.getByTestId("settings-workspace")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail")).toBeTruthy();
    expect(screen.getByTestId("account-settings-panel")).toBeTruthy();
    expect(screen.getByRole("heading", { name: /Account settings/i })).toBeTruthy();
    expect(screen.getByTestId("agent-stream")).toBeTruthy();
    await openToolFlyout();
    expect(await screen.findByTestId("tool-rail-hosting")).toBeTruthy();
    expect(await screen.findByTestId("tool-rail-env")).toBeTruthy();
    await user.click(screen.getByTestId("tool-rail-hosting"));
    expect(screen.getByTestId("hosting-panel")).toBeTruthy();
  });

  it("opens each Operate agent as its own chat card (1:1)", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-query"));
    expect(screen.getByTestId("agent-chat-pane-agent-query")).toBeTruthy();
    expect(screen.getByTestId("agent-chat-bound").textContent).toMatch(/1:1 chat/);
    expect(screen.getByTestId("agent-chat-pane-agent-query").textContent).toMatch(/QueryAssistant/);
    expect(screen.queryByTestId("agent-chat-attachments")).toBeNull();
    expect(screen.queryByTestId(`agent-chat-pane-${PRIMARY_OPERATE_CHAT_ID}`)).toBeNull();
  });

  it("swaps to another agent's own chat when a different agent is clicked", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-query"));
    expect(screen.getByTestId("agent-chat-pane-agent-query")).toBeTruthy();

    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-records"));
    expect(screen.getByTestId("agent-chat-pane-agent-records")).toBeTruthy();
    expect(screen.getByTestId("agent-chat-bound").textContent).toMatch(/1:1 chat/);
    expect(screen.getByTestId("agent-chat-pane-agent-records").textContent).toMatch(/RecordsAssistant/);
    // Still a single agent interaction on the board — distinct chat card.
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("1");
    expect(screen.queryByTestId("agent-chat-pane-agent-query")).toBeNull();
    expect(screen.queryByTestId(`agent-chat-pane-${PRIMARY_OPERATE_CHAT_ID}`)).toBeNull();
  });

  it("swaps the open tool when another tool is clicked", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-query"));
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("1");
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();

    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-monitor"));
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("1");
    expect(screen.getByTestId("workspace-tile-panel-monitor")).toBeTruthy();
    expect(screen.queryByTestId("workspace-tile-panel-query")).toBeNull();
    // Tool-only board — no default Operate chat.
    expect(screen.queryByTestId(`agent-chat-pane-${PRIMARY_OPERATE_CHAT_ID}`)).toBeNull();
  });

  it("opens a Build agent as its own chat tile instead of multi-attach", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-query"));
    expect(screen.getByTestId("agent-chat-bound").textContent).toMatch(/1:1 chat/);
    expect(screen.getByTestId("agent-chat-pane-agent-query").textContent).toMatch(/QueryAssistant/);

    // Switch to Build and use MetadataBuilder — should open a separate tile, not attach chips.
    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-build"));
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-meta"));
    expect(screen.getByTestId("agent-chat-pane-agent-meta")).toBeTruthy();
    expect(screen.queryByTestId("agent-chat-attachments")).toBeNull();
  });

  it("opens an agent from the catalog as its own chat tile", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    expect(screen.getByTestId("workspace-empty")).toBeTruthy();
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-query"));

    expect(screen.getByTestId("agent-chat-pane-agent-query")).toBeTruthy();
    const tile = screen.getByTestId("workspace-tile-chat-agent-query");
    expect(tile.className).toMatch(/is-solo/);
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("1");
    expect(screen.getByTestId("resize-chat-agent-query").textContent).toMatch(/Chat/i);
  });

  it("opens an agent beside a tool at one-third of the board", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-query"));
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();
    await openAgentFlyout();
    await user.click(await screen.findByTestId("agent-use-agent-query"));
    expect(screen.getByTestId("workspace-slices").getAttribute("data-count")).toBe("2");
    expect(screen.getByTestId("workspace-slices").getAttribute("data-ratios")).toBe("0.667 0.333");
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();
    expect(screen.getByTestId("agent-chat-pane-agent-query")).toBeTruthy();
  });

  it("preserves each mode's last workspace selection when switching tiles", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-query"));
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();

    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();
    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-objectHome"));
    expect(await screen.findByTestId("run-object-home-panel")).toBeTruthy();

    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-build"));
    expect(screen.getByTestId("workspace-tile-panel-query")).toBeTruthy();
    expect(screen.queryByTestId("run-object-home-panel")).toBeNull();

    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-object-home-panel")).toBeTruthy();
    expect(screen.queryByTestId("workspace-tile-panel-query")).toBeNull();
  });

  it("preserves Settings tool selection across mode round-trips", async () => {
    mockConnectedSession();
    mockRunGraphAPI();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-settings"));
    await openToolFlyout();
    await user.click(await screen.findByTestId("tool-rail-hosting"));
    expect(screen.getByTestId("hosting-panel")).toBeTruthy();

    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-operate"));
    expect(await screen.findByTestId("run-graph-home")).toBeTruthy();

    await user.click(screen.getByTestId("mode-title"));
    await user.click(screen.getByTestId("mode-launch-settings"));
    expect(screen.getByTestId("hosting-panel")).toBeTruthy();
  });

  it("filters agents by search", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await openAgentFlyout();
    await user.type(screen.getByLabelText(/Search agents/i), "Query");
    expect(await screen.findByTestId("agent-drag-agent-query")).toBeTruthy();
  });

  it("toggles theme and persists preference", async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(THEME_STORAGE_KEY, "dark");
    render(<App />);
    await screen.findByTestId("theme-toggle");
    expect(document.documentElement.dataset.theme).toBe("dark");
    await user.click(screen.getByTestId("theme-toggle"));
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
  });

  it("renders the change status footer bar and reflects Needs review", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    expect(screen.getByTestId("change-status-bar")).toBeTruthy();
    expect(screen.getByText("Draft")).toBeTruthy();
    expect(screen.queryByTestId("change-checks")).toBeNull();
    expect(screen.queryByText(/Checks /)).toBeNull();
    expect(screen.queryByText("Policy gate")).toBeNull();
    expect(screen.queryByRole("button", { name: /Mark as Ready/i })).toBeNull();
  });

  it("opens Connect via session chip when authenticated", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await user.click(screen.getByTestId("session-chip"));
    expect(screen.getByTestId("active-mode").textContent).toMatch(/Settings/);
    expect(screen.getByText(/Add or refresh credentials for an install/i)).toBeTruthy();
    expect(screen.getByTestId("connect-section")).toBeTruthy();
    expect(screen.getByTestId("workspace-tool-rail")).toBeTruthy();
    await openToolFlyout();
    expect(await screen.findByTestId("tool-rail-env")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-account")).toBeTruthy();
  });

  it("does not list Environments on the Govern rail", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await screen.findByTestId("mode-launcher");
    await user.click(screen.getByTestId("mode-launch-govern"));
    await screen.findByTestId("workspace-canvas");
    expect(screen.getByTestId("tool-rail-users")).toBeTruthy();
    expect(screen.getByTestId("tool-rail-permissions")).toBeTruthy();
    expect(screen.queryByTestId("tool-rail-env")).toBeNull();
  });

  it("returns to auth screen after clearing the session", async () => {
    mockConnectedSession();
    const user = userEvent.setup();
    render(<App />);
    await enterBuild(user);
    await user.click(screen.getByTestId("session-chip"));
    await screen.findByTestId("connect-section");
    await user.click(screen.getByRole("button", { name: /Clear session/i }));
    expect(await screen.findByTestId("auth-screen")).toBeTruthy();
    expect(screen.queryByTestId("mode-launcher")).toBeNull();
    expect(screen.queryByTestId("workspace-canvas")).toBeNull();
  });

  it("sends from an Operate agent chat and can reopen after close", async () => {
    mockConnectedSession();
    const runPayload = {
      id: "run-1",
      status: "completed",
      output: { summary: "Live run: ranked your accounts." },
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes("/client/v1/me")) {
          return jsonResponse({
            scopes: ["client", "metadata", "deploy"],
            isAdmin: true,
            displayName: "Ada Lovelace",
          });
        }
        if (url.includes("/client/v1/agents/playbooks")) {
          return jsonResponse({ playbooks: DEMO_PLAYBOOKS });
        }
        if (url.includes("/client/v1/agents/runs/") && url.includes("run-1")) {
          return new Response(JSON.stringify(runPayload), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          });
        }
        if (url.includes("/client/v1/agents/runs")) {
          return new Response(JSON.stringify(runPayload), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          });
        }
        return new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }),
    );
    const user = userEvent.setup();
    try {
      render(<App />);
      await enterBuild(user);
      await openAgentFlyout();
      await user.click(await screen.findByTestId("agent-use-agent-query"));
      const pane = screen.getByTestId("agent-chat-pane-agent-query");
      await user.type(screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i), "Plan my day");
      await user.click(within(pane).getByRole("button", { name: /^Send$/i }));
      expect(await screen.findByText(/Live run: ranked your accounts/i)).toBeTruthy();
      expect(screen.queryByText(/Connect to stream live/i)).toBeNull();
      await user.click(screen.getByTestId("close-tile-chat-agent-query"));
      expect(screen.queryByTestId("agent-chat-pane-agent-query")).toBeNull();
      expect(screen.getByTestId("workspace-empty")).toBeTruthy();
      expect(screen.getByText(/Add a tool or Agent/i)).toBeTruthy();
      // Re-open by choosing the agent again from the catalog.
      await openAgentFlyout();
      await user.click(await screen.findByTestId("agent-use-agent-query"));
      expect(screen.getByTestId("agent-chat-pane-agent-query")).toBeTruthy();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("pins an Operate handoff to the personal graph and focuses Run", async () => {
    mockConnectedSession();
    let graph = {
      apiVersion: "one.runGraph/v1",
      id: "home",
      title: "My graph",
      nodes: [] as Array<Record<string, unknown>>,
      edges: [] as Array<Record<string, unknown>>,
    };
    let revision = 1;
    const runPayload = {
      id: "run-handoff",
      status: "completed",
      output: {
        summary: "Top account",
        boardHandoff: {
          objectApiName: "Account",
          recordIds: ["00000000-0000-4000-8000-000000000111"],
        },
      },
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/client/v1/me")) {
        return jsonResponse({
          scopes: ["client", "metadata", "deploy"],
          isAdmin: true,
          displayName: "Ada Lovelace",
        });
      }
      if (url.includes("/client/v1/agents/playbooks")) {
        return jsonResponse({ playbooks: DEMO_PLAYBOOKS });
      }
      if (url.endsWith("/client/v1/agents/conversations") && init?.method === "POST") {
        return new Response(JSON.stringify({ id: "conversation-1" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      if (url.endsWith("/client/v1/agents/runs") && init?.method === "POST") {
        return new Response(JSON.stringify(runPayload), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      if (url.endsWith("/client/v1/run-graphs/home")) {
        if (init?.method === "PUT") {
          graph = JSON.parse(String(init.body)) as typeof graph;
          revision += 1;
        }
        return new Response(JSON.stringify({
          id: "home",
          graphKey: "home",
          title: "My graph",
          revision,
          document: graph,
        }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      if (url.endsWith("/client/v1/run-graphs/resolve")) {
        return new Response(JSON.stringify({ nodes: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      return new Response(JSON.stringify({ conversations: [], tools: [], items: [], messages: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }));
    const user = userEvent.setup();
    try {
      render(<App />);
      await enterBuild(user);
      await openAgentFlyout();
      await user.click(await screen.findByTestId("agent-use-agent-query"));
      const pane = screen.getByTestId("agent-chat-pane-agent-query");
      await user.type(screen.getByPlaceholderText(/Send follow-up to QueryAssistant/i), "Find my top account");
      await user.click(within(pane).getByRole("button", { name: /^Send$/i }));
      await user.click(await screen.findByTestId("chat-pin-to-graph"));

      expect(await screen.findByTestId("run-graph-home")).toBeTruthy();
      expect(screen.getByTestId("active-mode").textContent).toMatch(/Operate/);
      expect(graph.nodes).toEqual([
        expect.objectContaining({
          kind: "record",
          ref: {
            objectApiName: "Account",
            recordId: "00000000-0000-4000-8000-000000000111",
          },
        }),
      ]);
      expect(JSON.stringify(graph)).not.toMatch(/Top account|summary|fields|rows/);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
