import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AppBridge } from "../App";
import { McpCatalogPanel, mcpUrl } from "./McpCatalogPanel";
import { upsertEnvironment } from "../session";

afterEach(() => cleanup());

describe("McpCatalogPanel", () => {
  it("lists GET /mcp/tools and shows the POST /mcp URL", async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      tools: [{ name: "query", description: "Run a Client query" }],
    });
    const bridge: AppBridge = {
      session: upsertEnvironment(null, {
        installId: "dev",
        installRole: "demo",
        baseUrl: "http://localhost:8080",
        token: "jwt",
      }),
      setSession: vi.fn().mockResolvedValue(undefined),
      fetch: fetchImpl,
    };
    render(<McpCatalogPanel bridge={bridge} />);
    expect(await screen.findByText("query")).toBeTruthy();
    expect(screen.getByTestId("mcp-url")).toHaveProperty("value", "http://localhost:8080/mcp");
    expect(fetchImpl).toHaveBeenCalledWith("/mcp/tools");
    expect(mcpUrl("http://localhost:8080/")).toBe("http://localhost:8080/mcp");
  });
});
