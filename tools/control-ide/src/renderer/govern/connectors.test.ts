import { describe, expect, it, vi } from "vitest";
import { CONNECTOR_CATALOG, installCatalogConnector } from "./connectors";

describe("connector catalog", () => {
  it("installs secret, egress, and connector metadata as one guided flow", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/metadata/v1/secrets" && !init) return { secrets: [] };
      if (path === "/metadata/v1/install/egress" && !init) return { allowlist: [] };
      if (path === "/metadata/v1/connectors" && init?.method === "POST") return JSON.parse(String(init.body));
      return {};
    });
    const created = await installCatalogConnector(fetchFn, {
      catalog: CONNECTOR_CATALOG[0],
      apiName: "slack_ops",
      label: "Slack operations",
      baseUrl: "https://slack.com/api",
      secret: "xoxb-secret",
    });
    expect(created.secretRef).toBe("connector.slack_ops");
    expect(fetchFn).toHaveBeenCalledWith(
      "/metadata/v1/install/egress",
      expect.objectContaining({ method: "POST" }),
    );
    expect(fetchFn).toHaveBeenCalledWith(
      "/metadata/v1/connectors",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("rejects insecure URLs before writing install state", async () => {
    const fetchFn = vi.fn();
    await expect(installCatalogConnector(fetchFn, {
      catalog: CONNECTOR_CATALOG[2],
      apiName: "legacy_api",
      label: "Legacy API",
      baseUrl: "http://legacy.example.com",
      secret: "secret",
    })).rejects.toThrow(/HTTPS/);
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("removes a newly-created connector when policy setup fails", async () => {
    const fetchFn = vi.fn(async (path: string, init?: RequestInit) => {
      if (!init && path === "/metadata/v1/connectors") return { connectors: [] };
      if (!init && path === "/metadata/v1/secrets") return { secrets: [] };
      if (!init && path === "/metadata/v1/install/egress") return { allowlist: [] };
      if (path === "/metadata/v1/connectors" && init?.method === "POST") return JSON.parse(String(init.body));
      if (path === "/metadata/v1/install/egress" && init?.method === "POST") throw new Error("policy write failed");
      if (path === "/metadata/v1/connectors/graph_ops" && init?.method === "DELETE") return {};
      return {};
    });
    await expect(installCatalogConnector(fetchFn, {
      catalog: CONNECTOR_CATALOG[1],
      apiName: "graph_ops",
      label: "Graph operations",
      baseUrl: "https://graph.microsoft.com/v1.0",
      clientId: "client-id",
      secret: "secret",
    })).rejects.toThrow(/policy write failed/);
    expect(fetchFn).toHaveBeenCalledWith(
      "/metadata/v1/connectors/graph_ops",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
