import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it, vi } from "vitest";
import {
  approveAgentRunStream,
  createAgentRun,
  createAgentRunStream,
  createInferenceTestChat,
  isTerminalRunStatus,
  pollAgentRun,
  summarizeRunOutput,
  type AgentRun,
} from "./runs";

describe("agents/runs", () => {
  it("documents the hosted MCP tool loop (BP-006)", () => {
    const src = readFileSync(resolve("src/renderer/agents/runs.ts"), "utf8");
    expect(src).not.toMatch(/there is no hosted tool loop/i);
    expect(src).toMatch(/BP-006/);
  });

  it("forwards approved: false on create so tool execution stays gated (CIDE-18)", async () => {
    const fetchFn = vi.fn().mockResolvedValue({ id: "r1", status: "awaiting_approval" } satisfies AgentRun);
    await createAgentRun(fetchFn, {
      goal: "rank accounts",
      playbookApiName: "QueryAssistant",
      dryRun: false,
      approved: false,
    });
    expect(fetchFn).toHaveBeenCalledWith(
      "/client/v1/agents/runs",
      expect.objectContaining({ method: "POST" }),
    );
    const body = JSON.parse((fetchFn.mock.calls[0][1] as { body: string }).body) as {
      approved?: boolean;
      dryRun?: boolean;
    };
    expect(body.approved).toBe(false);
    expect(body.dryRun).toBe(false);
  });

  it("detects terminal statuses", () => {
    expect(isTerminalRunStatus("completed")).toBe(true);
    expect(isTerminalRunStatus("failed")).toBe(true);
    expect(isTerminalRunStatus("queued")).toBe(false);
  });

  it("summarizes output and errors", () => {
    expect(summarizeRunOutput({ id: "1", status: "failed", error: "boom" })).toMatch(/boom/);
    expect(
      summarizeRunOutput({
        id: "1",
        status: "completed",
        output: { summary: "done", toolsPlanned: ["query"] },
      }),
    ).toBe("done");
    expect(summarizeRunOutput({ id: "1", status: "queued", goal: "hiu" })).toMatch(/still queued/i);
    expect(summarizeRunOutput({ id: "1", status: "queued", goal: "hiu" })).not.toMatch(/^Approved/);
  });

  it("polls until terminal", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce({ id: "r1", status: "queued" } satisfies AgentRun)
      .mockResolvedValueOnce({ id: "r1", status: "completed", output: { summary: "ok" } } satisfies AgentRun);
    const run = await pollAgentRun(fetchFn, "r1", { intervalMs: 1, maxAttempts: 5 });
    expect(run.status).toBe("completed");
    expect(fetchFn).toHaveBeenCalledTimes(2);
  });

  it("throws when a poll stays queued instead of treating it as a reply", async () => {
    const fetchFn = vi.fn().mockResolvedValue({ id: "r1", status: "queued" } satisfies AgentRun);
    await expect(pollAgentRun(fetchFn, "r1", { intervalMs: 1, maxAttempts: 2 })).rejects.toThrow(
      /timed out while queued/,
    );
  });

  it("streams SSE token deltas and returns the completed run", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: run\ndata: {"id":"r2","status":"running"}\n\n'));
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"Hello "}\n\n'));
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"world"}\n\n'));
        controller.enqueue(encoder.encode('event: done\ndata: {"id":"r2","status":"completed","output":{"summary":"Hello world"}}\n\n'));
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } }));
    vi.stubGlobal("fetch", fetchMock);
    const deltas: string[] = [];
    const run = await createAgentRunStream("https://one.example", "token", { goal: "hello" }, {
      onToken: ({ delta }) => { if (delta) deltas.push(delta); },
    });
    expect(deltas.join("")).toBe("Hello world");
    expect(run).toMatchObject({ id: "r2", status: "completed", output: { summary: "Hello world" } });
    vi.unstubAllGlobals();
  });

  it("continues a parked stream create via SSE approve instead of treating it as a reply", async () => {
    const encoder = new TextEncoder();
    const sse = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: run\ndata: {"id":"r3","status":"running"}\n\n'));
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"hello"}\n\n'));
        controller.enqueue(
          encoder.encode(
            'event: done\ndata: {"id":"r3","status":"completed","output":{"summary":"hello"}}\n\n',
          ),
        );
        controller.close();
      },
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "r3", status: "awaiting_approval", goal: "hi" }), {
          status: 202,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(sse, { status: 200, headers: { "Content-Type": "text/event-stream" } }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const deltas: string[] = [];
    const run = await createAgentRunStream("https://one.example", "token", { goal: "hi" }, {
      onToken: ({ delta }) => {
        if (delta) deltas.push(delta);
      },
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "https://one.example/client/v1/agents/runs/r3/approve",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "text/event-stream" }),
      }),
    );
    expect(deltas.join("")).toBe("hello");
    expect(run).toMatchObject({ id: "r3", status: "completed" });
    vi.unstubAllGlobals();
  });

  it("throws when a stream create parks and approve also fails to stream", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "r4", status: "awaiting_approval" }), {
            status: 202,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "r4", status: "queued" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "r5", status: "awaiting_approval" }), {
            status: 202,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "r5", status: "queued" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        ),
    );
    await expect(
      createAgentRunStream("https://one.example", "token", { goal: "hi" }, {}),
    ).rejects.toThrow(/Settings → Inference/);
    vi.unstubAllGlobals();
  });

  it("retries a parked stream as generation-only when SSE approve is unavailable", async () => {
    const encoder = new TextEncoder();
    const sse = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"hey"}\n\n'));
        controller.enqueue(
          encoder.encode('event: done\ndata: {"id":"r6","status":"completed"}\n\n'),
        );
        controller.close();
      },
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "r4", status: "awaiting_approval" }), {
          status: 202,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "r4", status: "queued" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(sse, { status: 200, headers: { "Content-Type": "text/event-stream" } }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const deltas: string[] = [];
    const run = await createAgentRunStream(
      "https://one.example",
      "token",
      { goal: "hi", approved: false },
      {
        onToken: ({ delta }) => {
          if (delta) deltas.push(delta);
        },
      },
    );
    const retryBody = JSON.parse((fetchMock.mock.calls[2][1] as { body: string }).body) as {
      approved?: boolean;
      stream?: boolean;
    };
    expect(retryBody.approved).toBe(true);
    expect(retryBody.stream).toBe(true);
    expect(deltas.join("")).toBe("hey");
    expect(run.status).toBe("completed");
    vi.unstubAllGlobals();
  });

  it("sends an inference test chat with approved:true so older APIs still generate", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"hi"}\n\n'));
        controller.enqueue(
          encoder.encode('event: done\ndata: {"id":"t1","status":"completed"}\n\n'),
        );
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await createInferenceTestChat("https://one.example", "token", "Say hi", {});
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as { body: string }).body) as {
      approved?: boolean;
      stream?: boolean;
      playbookApiName?: string;
    };
    expect(sent.approved).toBe(true);
    expect(sent.stream).toBe(true);
    expect(sent.playbookApiName).toBeUndefined();
    vi.unstubAllGlobals();
  });

  it("approves with SSE stream headers and token deltas", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: run\ndata: {"id":"r9","status":"running"}\n\n'));
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"ok"}\n\n'));
        controller.enqueue(
          encoder.encode(
            'event: done\ndata: {"id":"r9","status":"completed","output":{"summary":"ok"}}\n\n',
          ),
        );
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const deltas: string[] = [];
    const run = await approveAgentRunStream("https://one.example", "token", "r9", {
      onToken: ({ delta }) => {
        if (delta) deltas.push(delta);
      },
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "https://one.example/client/v1/agents/runs/r9/approve",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "text/event-stream" }),
      }),
    );
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as { body: string }).body) as { stream?: boolean };
    expect(sent.stream).toBe(true);
    expect(deltas.join("")).toBe("ok");
    expect(run).toMatchObject({ id: "r9", status: "completed", output: { summary: "ok" } });
    vi.unstubAllGlobals();
  });
});
