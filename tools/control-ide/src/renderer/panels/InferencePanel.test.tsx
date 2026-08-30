import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { InferencePanel } from "./InferencePanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function bridge(fetchImpl?: AppBridge["fetch"]): AppBridge {
  return {
    session: {
      activeInstallId: "dev",
      environments: [],
      baseUrl: "http://api.test",
      token: "jwt",
    },
    setSession: vi.fn().mockResolvedValue(undefined),
    fetch: fetchImpl ?? vi.fn().mockResolvedValue({ activeSource: "byo", defaultProviderApiName: "ollama" }),
  };
}

describe("InferencePanel", () => {
  it("streams a Settings test chat without requiring Approve", async () => {
    const user = userEvent.setup();
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: run\ndata: {"id":"t1","status":"running"}\n\n'));
        controller.enqueue(encoder.encode('event: token\ndata: {"delta":"Hello from Ollama"}\n\n'));
        controller.enqueue(
          encoder.encode(
            'event: done\ndata: {"id":"t1","status":"completed","output":{"summary":"Hello from Ollama"}}\n\n',
          ),
        );
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    render(<InferencePanel bridge={bridge()} />);
    expect(await screen.findByTestId("inference-test-chat")).toBeTruthy();
    await user.click(screen.getByTestId("inference-test-send"));
    await waitFor(() =>
      expect(screen.getByTestId("inference-test-reply").textContent).toContain("Hello from Ollama"),
    );
    expect(screen.queryByTestId("inference-test-error")).toBeNull();
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as { body: string }).body) as {
      approved?: boolean;
      stream?: boolean;
      goal?: string;
    };
    expect(sent.approved).toBe(true);
    expect(sent.stream).toBe(true);
    expect(sent.goal).toMatch(/hello/i);
  });

  it("surfaces a parked-run error instead of an Approve bubble", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "parked", status: "awaiting_approval" }), {
            status: 202,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "parked", status: "queued" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "parked2", status: "awaiting_approval" }), {
            status: 202,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ id: "parked2", status: "queued" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        ),
    );
    render(<InferencePanel bridge={bridge()} />);
    await user.click(await screen.findByTestId("inference-test-send"));
    const err = await screen.findByTestId("inference-test-error");
    expect(err.textContent).toMatch(/Settings → Inference|parked/i);
  });
});
