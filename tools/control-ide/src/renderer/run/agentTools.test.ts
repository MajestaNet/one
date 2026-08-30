import { afterEach, describe, expect, it, vi } from "vitest";
import { demoAccountsTool } from "./fixtures";
import {
  applyToolEffectsFromRunOutput,
  executeToolBridge,
  toolCreate,
  toolGet,
  toolList,
  toolRerun,
  toolSaveAsSpec,
  toolUpdate,
} from "./agentTools";
import { processRunToolEffects } from "./runToolEffects";
import { loadToolStore } from "./store";
import { TOOL_DOCUMENT_API_VERSION } from "./types";

afterEach(() => {
  localStorage.clear();
});

describe("tool agent bridge", () => {
  it("tool.create validates and persists", () => {
    const created = toolCreate({ document: demoAccountsTool() });
    expect(created.ok).toBe(true);
    if (!created.ok) return;
    expect(created.toolId).toBe("demo-top-accounts");
    const store = loadToolStore();
    expect(store.documents.some((d) => d.id === "demo-top-accounts")).toBe(true);
  });

  it("rejects malicious unknown node kinds at the bridge", async () => {
    const result = await executeToolBridge("tool.create", {
      document: {
        apiVersion: TOOL_DOCUMENT_API_VERSION,
        id: "evil",
        title: "Evil",
        layout: { mode: "sections" },
        nodes: [{ id: "x", kind: "rawHtml", props: {} }],
      },
    }, {});
    expect((result as { ok: boolean }).ok).toBe(false);
    expect(loadToolStore().documents).toHaveLength(0);
  });

  it("tool.update patches nodes", () => {
    toolCreate({ document: demoAccountsTool() });
    const updated = toolUpdate({
      toolId: "demo-top-accounts",
      patch: {
        title: "Renamed",
        nodes: [
          {
            id: "note",
            kind: "markdownNote",
            props: { text: "Updated by agent" },
          },
        ],
      },
    });
    expect(updated.ok).toBe(true);
    if (!updated.ok) return;
    const got = toolGet({ toolId: "demo-top-accounts" });
    expect(got.ok).toBe(true);
    if (!got.ok) return;
    expect(got.document.title).toBe("Renamed");
    expect(got.document.nodes.find((n) => n.id === "note")?.props.text).toBe("Updated by agent");
  });

  it("tool.update merges dataBindings by id", () => {
    toolCreate({ document: demoAccountsTool() });
    const updated = toolUpdate({
      toolId: "demo-top-accounts",
      patch: {
        dataBindings: [
          {
            id: "bind-accounts",
            objectApiName: "Account",
            query: { limit: 5, sort: [{ field: "AnnualRevenue", direction: "desc" }] },
          },
        ],
      },
    });
    expect(updated.ok).toBe(true);
    if (!updated.ok) return;
    expect(updated.document.dataBindings?.[0]?.query?.limit).toBe(5);
  });

  it("tool.list returns saved documents", () => {
    toolCreate({ document: demoAccountsTool() });
    const list = toolList();
    expect(list.ok).toBe(true);
    expect(list.tools).toHaveLength(1);
  });

  it("tool.rerun refreshes binding-backed nodes", async () => {
    toolCreate({ document: demoAccountsTool() });
    const fetch = vi.fn(async () => ({
      records: [{ id: "acc-9", Name: "Fresh Co" }],
    }));
    const rerun = await toolRerun({ toolId: "demo-top-accounts" }, { fetch });
    expect(rerun.ok).toBe(true);
    if (!rerun.ok) return;
    expect(fetch).toHaveBeenCalled();
    const stat = rerun.document.nodes.find((n) => n.id === "stat-open");
    expect(stat?.props.value).toBe(1);
  });

  it("tool.saveAsSpec posts Metadata ToolSpec without baked record rows", async () => {
    toolCreate({ document: demoAccountsTool() });
    const fetch = vi.fn(async () => ({ apiName: "Demo_Tool" }));
    const saved = await toolSaveAsSpec(
      { toolId: "demo-top-accounts", apiName: "Demo_Tool", label: "Demo Tool" },
      { fetch },
    );
    expect(saved.ok).toBe(true);
    if (!saved.ok) return;
    expect(fetch).toHaveBeenCalledWith(
      "/metadata/v1/tools",
      expect.objectContaining({ method: "POST" }),
    );
    const body = JSON.parse(String((fetch.mock.calls[0][1] as { body?: string }).body));
    const table = body.nodes.find((n: { id: string }) => n.id === "table");
    expect(table.props.rows).toBeUndefined();
    expect(table.props.columns).toBeTruthy();
    const got = toolGet({ toolId: "demo-top-accounts" });
    expect(got.ok).toBe(true);
    if (!got.ok) return;
    expect(got.document.toolSpecApiName).toBe("Demo_Tool");
  });

  it("applyToolEffectsFromRunOutput handles toolDocument and tool calls", async () => {
    const doc = { ...demoAccountsTool(), id: "from-run" };
    const effects = await applyToolEffectsFromRunOutput(
      {
        toolDocument: doc,
        toolCalls: [{ tool: "tool.update", input: { toolId: "from-run", title: "From agent" } }],
      },
      {},
    );
    expect(effects.toolId).toBe("from-run");
    expect(effects.toolTitle).toBe("From agent");
    const got = toolGet({ toolId: "from-run" });
    expect(got.ok).toBe(true);
    if (!got.ok) return;
    expect(got.document.title).toBe("From agent");
  });
});

describe("processRunToolEffects", () => {
  it("synthesizes a Tool when the goal asks for an interactive tool", async () => {
    const effects = await processRunToolEffects(
      {
        id: "run-tool-1",
        status: "completed",
        goal: "Show my top Accounts as an interactive tool with stats and a table",
        output: { summary: "Built account view", toolsPlanned: ["query"] },
      },
      { mode: "operate" },
    );
    expect(effects.toolId).toBe("run-run-tool-1");
    expect(effects.enrichedOutput?.toolHandoff).toMatchObject({
      toolId: "run-run-tool-1",
    });
  });
});
