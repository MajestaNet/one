import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeContext } from "../ThemeContext";
import type { AppBridge } from "../App";
import { QueryPanel } from "./QueryPanel";
import { MonitorPanel, appendSyntheticMonitorLines } from "./MonitorPanel";
import { ExplorerPanel } from "./ExplorerPanel";
import {
  MONITOR_RING_MAX,
  appendMonitorLines,
  createMonitorRing,
  normalizeLogLine,
  trimMessage,
  windowLines,
} from "./monitorRing";
import {
  defaultQueryJson,
  fieldSuggestions,
  flattenRecordRow,
  objectSuggestions,
  opSuggestions,
  rankSuggestions,
  resultColumns,
} from "./queryAutocomplete";
import {
  buildEdgesFromDescribe,
  layoutObjectGraph,
  selectVisibleObjects,
} from "./objectGraph";
import {
  DescribeCache,
  describeCache,
  normalizeDescribeObject,
  normalizeGlobalObjects,
} from "./describeCache";

afterEach(() => {
  cleanup();
  describeCache.clear();
});

function bridge(fetchImpl: AppBridge["fetch"], session?: AppBridge["session"] | null): AppBridge {
  return {
    session:
      session === null
        ? null
        : (session ?? {
            activeInstallId: "inst-1",
            environments: [
              {
                installId: "inst-1",
                installRole: "prod",
                baseUrl: "http://localhost:8080",
                token: "jwt",
              },
            ],
            baseUrl: "http://localhost:8080",
            token: "jwt",
          }),
    setSession: vi.fn(),
    fetch: fetchImpl,
  };
}

describe("queryAutocomplete", () => {
  it("ranks field suggestions by prefix", () => {
    const ranked = rankSuggestions(
      fieldSuggestions([
        { apiName: "Name" },
        { apiName: "NamingConvention" },
        { apiName: "Industry" },
      ]),
      "nam",
    );
    expect(ranked.map((r) => r.label)).toEqual(["Name", "NamingConvention"]);
  });

  it("returns all suggestions when prefix empty and builds object/op helpers", () => {
    expect(rankSuggestions(objectSuggestions(["Account", "Contact"]), "")).toHaveLength(2);
    expect(opSuggestions().some((o) => o.label === "eq")).toBe(true);
    expect(defaultQueryJson("Contact")).toContain('"object": "Contact"');
  });

  it("orders result columns with Name/Id first and flattens nested rows", () => {
    expect(resultColumns([{ Industry: "Tech", id: "1", Name: "Acme" }])).toEqual([
      "Name",
      "id",
      "Industry",
    ]);
    expect(resultColumns([])).toEqual(["id"]);
    expect(
      flattenRecordRow({
        Name: "A",
        Owner: { id: "u1" },
        Tags: ["x", "y"],
      }),
    ).toEqual({
      Name: "A",
      Owner: JSON.stringify({ id: "u1" }),
      Tags: "[2]",
    });
  });
});

describe("describeCache", () => {
  it("caches global/object describes and invalidates by install", () => {
    const cache = new DescribeCache(60_000);
    cache.setGlobal("a", [{ apiName: "Account" }]);
    cache.setObject("a", "Account", { apiName: "Account", fields: [{ apiName: "Name" }] });
    expect(cache.getGlobal("a")?.[0].apiName).toBe("Account");
    expect(cache.getObject("a", "Account")?.fields?.[0].apiName).toBe("Name");
    cache.invalidateInstall("a");
    expect(cache.getGlobal("a")).toBeNull();
    expect(cache.getObject("a", "Account")).toBeNull();
  });

  it("expires stale entries and normalizes describe payloads", () => {
    const cache = new DescribeCache(1);
    cache.setGlobal("x", [{ apiName: "User" }]);
    cache.setObject("x", "User", { apiName: "User", fields: [] });
    // Force expiry by rewriting timestamps via short TTL wait
    const start = Date.now();
    while (Date.now() - start < 3) {
      /* spin */
    }
    expect(cache.getGlobal("x")).toBeNull();
    expect(cache.getObject("x", "User")).toBeNull();

    expect(normalizeGlobalObjects({ objects: [{ apiName: "  A  " }, { apiName: "" }] })).toEqual([
      { apiName: "A", label: undefined, pluralLabel: undefined, packageName: undefined },
    ]);
    expect(
      normalizeGlobalObjects({
        sobjects: [{ name: "Account", label: "Account", labelPlural: "Accounts" }],
      }),
    ).toEqual([
      {
        apiName: "Account",
        label: "Account",
        pluralLabel: "Accounts",
        packageName: undefined,
      },
    ]);
    expect(
      normalizeDescribeObject(
        { fields: [{ apiName: "Name", type: "string" }, { name: "" }] },
        "Account",
      ).fields,
    ).toEqual([{ apiName: "Name", fieldType: "string", referenceTo: null, relationshipName: null, length: null }]);
  });
});

describe("monitorRing", () => {
  it("drops oldest lines beyond ring max", () => {
    let state = createMonitorRing();
    state = appendSyntheticMonitorLines(state, MONITOR_RING_MAX + 500, 1);
    expect(state.lines.length).toBe(MONITOR_RING_MAX);
    expect(state.dropped).toBe(500);
    expect(state.lines[0]?.seq).toBe(501);
    expect(state.maxSeq).toBe(MONITOR_RING_MAX + 500);
  });

  it("windows visible rows for virtualization", () => {
    const lines = Array.from({ length: 100 }, (_, i) => ({
      seq: i + 1,
      level: "info" as const,
      message: `m${i}`,
      at: "t",
    }));
    const win = windowLines(lines, 280, 140, 28);
    expect(win.start).toBeGreaterThanOrEqual(0);
    expect(win.end - win.start).toBeLessThanOrEqual(20);
    expect(win.offsetY).toBe(win.start * 28);
    expect(windowLines([], 0, 100, 28)).toEqual({ start: 0, end: 0, offsetY: 0 });
  });

  it("normalizes and trims log lines", () => {
    expect(trimMessage("abc", 2)).toBe("ab…");
    expect(normalizeLogLine(null, 1)).toBeNull();
    expect(normalizeLogLine({ seq: "bad" }, 1)).toBeNull();
    const line = normalizeLogLine(
      {
        Seq: 9,
        Level: "WARN",
        Message: "hello",
        CreatedAt: "t1",
        ExecutionRunId: "run-1",
      },
      1,
    );
    expect(line).toMatchObject({
      seq: 9,
      level: "warn",
      message: "hello",
      at: "t1",
      runId: "run-1",
    });
  });

  it("appendMonitorLines ignores duplicate seq and empty batches", () => {
    let state = createMonitorRing();
    expect(appendMonitorLines(state, [])).toBe(state);
    state = appendMonitorLines(state, [
      { seq: 1, level: "info", message: "a", at: "t" },
      { seq: 1, level: "info", message: "a", at: "t" },
    ]);
    expect(state.lines).toHaveLength(1);
  });
});

describe("objectGraph", () => {
  it("builds lookup edges and layouts with dagre", () => {
    const objects = [
      { apiName: "Account", label: "Account", packageName: "core" },
      { apiName: "Contact", label: "Contact", packageName: "core" },
    ];
    const describes = new Map([
      [
        "Contact",
        {
          apiName: "Contact",
          fields: [{ apiName: "AccountId", referenceTo: "Account", fieldType: "lookup" }],
        },
      ],
      ["Account", { apiName: "Account", fields: [{ apiName: "Name" }] }],
    ]);
    const edges = buildEdgesFromDescribe(
      "Contact",
      describes.get("Contact")!.fields!,
      new Set(["Account", "Contact"]),
    );
    expect(edges).toHaveLength(1);
    expect(edges[0].to).toBe("Account");
    const graph = layoutObjectGraph(objects, describes);
    expect(graph.nodes).toHaveLength(2);
    expect(graph.nodes.every((n) => n.enabled)).toBe(true);
    expect(graph.edges.length).toBeGreaterThanOrEqual(1);
    expect(graph.width).toBeGreaterThan(0);
  });

  it("filters and caps visible objects", () => {
    const many = Array.from({ length: 120 }, (_, i) => ({
      apiName: `Obj${i}`,
      label: `Obj ${i}`,
      packageName: i % 2 === 0 ? "core" : "sales",
    }));
    expect(selectVisibleObjects(many, { maxNodes: 80 })).toHaveLength(80);
    expect(selectVisibleObjects(many, { packages: ["sales"], maxNodes: 200 }).every((o) => o.packageName === "sales")).toBe(
      true,
    );
    expect(selectVisibleObjects(many, { search: "Obj1", maxNodes: 200 }).some((o) => o.apiName === "Obj1")).toBe(
      true,
    );
  });
});

describe("QueryPanel", () => {
  it("loads objects and runs a query into the results list", async () => {
    const user = userEvent.setup();
    const fetchImpl = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/client/v1/describe") {
        return { objects: [{ apiName: "Account", label: "Account" }] };
      }
      if (path.startsWith("/client/v1/describe/")) {
        return { apiName: "Account", fields: [{ apiName: "Name", fieldType: "string" }] };
      }
      if (path === "/client/v1/query") {
        expect(init?.method).toBe("POST");
        return { records: [{ id: "a1", Name: "Acme" }] };
      }
      throw new Error(`unexpected ${path}`);
    });
    render(
      <ThemeContext.Provider value="dark">
        <QueryPanel bridge={bridge(fetchImpl)} />
      </ThemeContext.Provider>,
    );
    await waitFor(() => expect(screen.getByTestId("query-object-select")).toBeTruthy());
    await user.click(screen.getByRole("button", { name: /^Run$/i }));
    await waitFor(() => expect(screen.getByText("Acme")).toBeTruthy());
    await user.click(screen.getByRole("button", { name: /^Clear$/i }));
    await waitFor(() => expect(screen.getByText(/Run a query to list matching records/i)).toBeTruthy());
  });

  it("shows connect empty state when disconnected", () => {
    render(
      <ThemeContext.Provider value="dark">
        <QueryPanel bridge={bridge(vi.fn(), null)} />
      </ThemeContext.Provider>,
    );
    expect(screen.getByText(/Connect to query/i)).toBeTruthy();
  });

  it("surfaces invalid JSON errors", async () => {
    const user = userEvent.setup();
    const fetchImpl = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") return { objects: [{ apiName: "Account" }] };
      return {};
    });
    const { container } = render(
      <ThemeContext.Provider value="dark">
        <QueryPanel bridge={bridge(fetchImpl)} />
      </ThemeContext.Provider>,
    );
    await waitFor(() => expect(screen.getByTestId("query-object-select")).toBeTruthy());
    // Monaco stub stores value; poke invalid JSON via editor host presence then Run after setValue through stub.
    const monaco = await import("monaco-editor");
    const ed = monaco.editor.create(container);
    ed.setValue("{not-json");
    await user.click(screen.getByRole("button", { name: /^Run$/i }));
    await waitFor(() => expect(screen.getByText(/Query JSON is invalid/i)).toBeTruthy());
  });

  it("asks an agent about query results", async () => {
    const user = userEvent.setup();
    const onAskAgent = vi.fn();
    const fetchImpl = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return { objects: [{ apiName: "Account", label: "Account" }] };
      }
      if (path.startsWith("/client/v1/describe/")) {
        return { apiName: "Account", fields: [{ apiName: "Name", fieldType: "string" }] };
      }
      if (path === "/client/v1/query") {
        return { records: [{ id: "a1", Name: "Acme" }] };
      }
      return {};
    });
    render(
      <ThemeContext.Provider value="dark">
        <QueryPanel bridge={bridge(fetchImpl)} onAskAgent={onAskAgent} />
      </ThemeContext.Provider>,
    );
    await waitFor(() => expect(screen.getByTestId("query-object-select")).toBeTruthy());
    await user.click(screen.getByRole("button", { name: /^Run$/i }));
    await waitFor(() => expect(screen.getByTestId("query-ask-agent")).toBeTruthy());
    await user.click(screen.getByTestId("query-ask-agent"));
    expect(onAskAgent).toHaveBeenCalledWith(expect.stringMatching(/Account.*a1/i));
  });
});

describe("MonitorPanel", () => {
  it("shows BP-033 missing state when debug objects are absent", async () => {
    const fetchImpl = vi.fn(async () => {
      throw new Error("404");
    });
    render(<MonitorPanel bridge={bridge(fetchImpl)} />);
    await waitFor(() =>
      expect(screen.getByText(/Trace requires install debug objects/i)).toBeTruthy(),
    );
    expect(fetchImpl).toHaveBeenCalledWith("/metadata/v1/objects/ExecutionLogEntry");
    expect(fetchImpl.mock.calls.some(([path]) => String(path).includes("/client/v1/debug/"))).toBe(
      false,
    );
  });

  it("shows connect empty state when disconnected", () => {
    render(<MonitorPanel bridge={bridge(vi.fn(), null)} />);
    expect(screen.getByText(/Connect to monitor/i)).toBeTruthy();
  });

  it("arms, pauses, clears, and stops a TraceFlag via Client objects", async () => {
    const user = userEvent.setup();
    let flags: Array<Record<string, unknown>> = [];
    const fetchImpl = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/objects/ExecutionLogEntry") {
        return { apiName: "ExecutionLogEntry" };
      }
      if (path === "/client/v1/sobjects/TraceFlag" && init?.method === "POST") {
        flags = [
          {
            id: "tf1",
            TracedUserId: "u1",
            Level: "info",
            ExpiresAt: "soon",
            Active: true,
          },
        ];
        return { ok: true };
      }
      if (path.startsWith("/client/v1/sobjects/TraceFlag/") && init?.method === "DELETE") {
        flags = [];
        return { ok: true };
      }
      if (path === "/client/v1/query") {
        const body = JSON.parse(String(init?.body ?? "{}")) as { object?: string };
        if (body.object === "TraceFlag") {
          return { records: flags };
        }
        if (body.object === "ExecutionLogEntry") {
          return { records: [{ seq: 1, level: "info", message: "hello", at: "t" }] };
        }
        return { records: [{ id: "u1", Name: "Ada" }] };
      }
      throw new Error(`unexpected ${path}`);
    });
    render(<MonitorPanel bridge={bridge(fetchImpl)} />);
    await waitFor(() => expect(screen.getByTestId("monitor-user-select")).toBeTruthy());
    await user.selectOptions(screen.getByTestId("monitor-user-select"), "u1");
    await user.click(screen.getByRole("button", { name: /Start trace/i }));
    await waitFor(() => expect(screen.getByText("u1")).toBeTruthy());
    await user.click(screen.getByTestId("monitor-pause"));
    expect(screen.getByRole("button", { name: /Resume/i })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /Clear buffer/i }));
    await user.click(screen.getByRole("button", { name: /^Stop$/i }));
    await waitFor(() => expect(screen.getByText(/No active TraceFlags/i)).toBeTruthy());
    expect(fetchImpl.mock.calls.some(([path]) => String(path).includes("/client/v1/debug/"))).toBe(
      false,
    );
  });

  it("stays on the missing empty-state instead of calling unregistered debug routes", async () => {
    const fetchImpl = vi.fn(async (path: string) => {
      if (path.startsWith("/client/v1/debug/")) {
        return { flags: [{ id: "tf-debug", tracedUserId: "u-debug", level: "info", active: true }] };
      }
      throw new Error("404 ExecutionLogEntry");
    });
    render(<MonitorPanel bridge={bridge(fetchImpl)} />);
    await waitFor(() =>
      expect(screen.getByText(/Trace requires install debug objects/i)).toBeTruthy(),
    );
    expect(screen.queryByText("u-debug")).toBeNull();
    expect(fetchImpl.mock.calls.some(([path]) => String(path).includes("/client/v1/debug/"))).toBe(
      false,
    );
  });
});

describe("ExplorerPanel", () => {
  it("renders graph nodes from describe metadata", async () => {
    const fetchImpl = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return {
          objects: [
            { apiName: "Account", label: "Account", packageName: "core" },
            { apiName: "Contact", label: "Contact", packageName: "core" },
          ],
        };
      }
      if (path.includes("Contact")) {
        return {
          apiName: "Contact",
          fields: [{ apiName: "AccountId", referenceTo: "Account", fieldType: "lookup" }],
        };
      }
      return { apiName: "Account", fields: [{ apiName: "Name" }] };
    });
    render(<ExplorerPanel bridge={bridge(fetchImpl)} />);
    await waitFor(() => expect(screen.getByTestId("explorer-node-Account")).toBeTruthy());
    expect(screen.getByTestId("explorer-node-Contact")).toBeTruthy();
  });

  it("shows connect empty state when disconnected", () => {
    render(<ExplorerPanel bridge={bridge(vi.fn(), null)} />);
    expect(screen.getByText(/Connect to explore/i)).toBeTruthy();
  });

  it("lists not-enabled catalog packages and visualizes their objects", async () => {
    const user = userEvent.setup();
    const onOpenInQuery = vi.fn();
    const fetchImpl = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return {
          objects: [{ apiName: "Account", label: "Account", packageName: "core" }],
        };
      }
      if (path === "/metadata/v1/packages") {
        return {
          packages: [
            {
              name: "core",
              enabled: true,
              objectApiNames: ["Account"],
              objects: [{ apiName: "Account", label: "Account", fieldCount: 4, fields: [] }],
            },
            {
              name: "sales",
              enabled: false,
              objectApiNames: ["Opportunity"],
              objects: [
                {
                  apiName: "Opportunity",
                  label: "Opportunity",
                  fieldCount: 14,
                  fields: [{ apiName: "AccountId", fieldType: "lookup", referenceTo: "Account" }],
                },
              ],
            },
          ],
        };
      }
      if (path.includes("/describe/")) {
        return { apiName: "Account", fields: [{ apiName: "Name" }] };
      }
      throw new Error(`unexpected ${path}`);
    });
    render(<ExplorerPanel bridge={bridge(fetchImpl)} onOpenInQuery={onOpenInQuery} />);
    await waitFor(() => expect(screen.getByTestId("explorer-node-Account")).toBeTruthy());
    expect(screen.queryByTestId("explorer-node-Opportunity")).toBeNull();
    const select = screen.getByTestId("explorer-package") as HTMLSelectElement;
    expect([...select.options].map((o) => o.text)).toEqual(
      expect.arrayContaining(["Enabled", "All packages", "sales (not enabled)"]),
    );
    await user.selectOptions(select, "sales");
    await waitFor(() => expect(screen.getByTestId("explorer-node-Opportunity")).toBeTruthy());
    expect(screen.getByTestId("explorer-node-Opportunity").getAttribute("class")).toMatch(/is-catalog/);
    expect(screen.queryByTestId("explorer-node-Account")).toBeNull();
    fireEvent.mouseEnter(screen.getByTestId("explorer-node-Opportunity"));
    await waitFor(() => expect(screen.getByTestId("explorer-node-not-enabled")).toBeTruthy());
    expect(screen.queryByRole("button", { name: /Open in Query/i })).toBeNull();
    await user.selectOptions(select, "*");
    await waitFor(() => expect(screen.getByTestId("explorer-node-Account")).toBeTruthy());
    expect(screen.getByTestId("explorer-node-Opportunity")).toBeTruthy();
  });

  it("filters by search, zooms, and opens query from hover", async () => {
    const user = userEvent.setup();
    const onOpenInQuery = vi.fn();
    const fetchImpl = vi.fn(async (path: string) => {
      if (path === "/client/v1/describe") {
        return {
          objects: [
            { apiName: "Account", label: "Account", packageName: "core" },
            { apiName: "Contact", label: "Contact", packageName: "sales" },
          ],
        };
      }
      if (path.includes("Contact")) {
        return {
          apiName: "Contact",
          fields: [{ apiName: "AccountId", referenceTo: "Account", fieldType: "lookup" }],
        };
      }
      return { apiName: "Account", fields: [{ apiName: "Name" }] };
    });
    render(<ExplorerPanel bridge={bridge(fetchImpl)} onOpenInQuery={onOpenInQuery} />);
    await waitFor(() => expect(screen.getByTestId("explorer-node-Account")).toBeTruthy());
    await user.type(screen.getByTestId("explorer-search"), "Contact");
    await waitFor(() => expect(screen.queryByTestId("explorer-node-Account")).toBeNull());
    expect(screen.getByTestId("explorer-node-Contact")).toBeTruthy();
    await user.clear(screen.getByTestId("explorer-search"));
    await waitFor(() => expect(screen.getByTestId("explorer-node-Account")).toBeTruthy());
    await user.selectOptions(screen.getByTestId("explorer-package"), "sales");
    await waitFor(() => expect(screen.queryByTestId("explorer-node-Account")).toBeNull());
    fireEvent.mouseEnter(screen.getByTestId("explorer-node-Contact"));
    await waitFor(() => expect(screen.getByTestId("explorer-node-popover")).toBeTruthy());
    await user.click(screen.getByRole("button", { name: /Open in Query/i }));
    expect(onOpenInQuery).toHaveBeenCalledWith("Contact");
    await user.click(screen.getByRole("button", { name: /Zoom in/i }));
    await user.click(screen.getByRole("button", { name: /Zoom out/i }));
    await user.click(screen.getByRole("button", { name: /^Fit$/i }));
  });
});
