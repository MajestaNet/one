/**
 * Build an McpServer that talks to a Majesta One install via JWT.
 * Product tools can be proxied to POST /mcp; custom vertical tools call Client HTTP.
 */
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { OneClient } from "./one-client.js";

export type BuildServerOptions = {
  client: OneClient;
  /** When true, register tools that forward to the product MCP gateway. */
  proxyProductTools?: boolean;
};

function textResult(data: unknown) {
  return {
    content: [{ type: "text" as const, text: typeof data === "string" ? data : JSON.stringify(data, null, 2) }],
  };
}

export function buildOneMcpServer(opts: BuildServerOptions): McpServer {
  const { client } = opts;
  const proxy = opts.proxyProductTools ?? process.env.ONE_PROXY_PRODUCT_TOOLS === "1";

  const server = new McpServer({
    name: "one-mcp",
    version: "0.1.0",
  });

  // --- Custom vertical example: Client HTTP (always registered) ---

  server.tool(
    "one_describe_object",
    "Describe an object via Client GET /client/v1/describe/{object}",
    { object: z.string().describe("Object API name, e.g. Account") },
    async ({ object }) => {
      const data = await client.request("GET", `/client/v1/describe/${encodeURIComponent(object)}`);
      return textResult(data);
    },
  );

  server.tool(
    "one_query",
    "Run a Client query via POST /client/v1/query",
    {
      object: z.string(),
      filters: z.array(z.unknown()).optional(),
      sort: z.array(z.unknown()).optional(),
      limit: z.number().int().positive().optional(),
      cursor: z.string().optional(),
    },
    async (args) => {
      const data = await client.request("POST", "/client/v1/query", args);
      return textResult(data);
    },
  );

  server.tool(
    "one_search",
    "Cross-object find via POST /client/v1/search",
    {
      q: z.string().describe("Name, email, phone, or other searchable needle"),
      objects: z.array(z.string()).optional(),
      limit: z.number().int().positive().optional(),
    },
    async (args) => {
      const data = await client.request("POST", "/client/v1/search", args);
      return textResult(data);
    },
  );

  server.tool(
    "one_get_record",
    "Get a record via Client GET /client/v1/sobjects/{object}/{id}",
    {
      object: z.string(),
      id: z.string(),
    },
    async ({ object, id }) => {
      const data = await client.request(
        "GET",
        `/client/v1/sobjects/${encodeURIComponent(object)}/${encodeURIComponent(id)}`,
      );
      return textResult(data);
    },
  );

  server.tool(
    "one_create_record",
    "Create a record via Client POST /client/v1/sobjects/{object}",
    {
      object: z.string(),
      data: z.record(z.unknown()),
    },
    async ({ object, data }) => {
      const created = await client.request("POST", `/client/v1/sobjects/${encodeURIComponent(object)}`, { data });
      return textResult(created);
    },
  );

  /**
   * Example custom vertical tool — replace with customer-specific glue.
   * Still AuthZ'd by the Majesta One principal used for client_credentials.
   */
  server.tool(
    "one_ping_install",
    "Health check: mint a token and list Client describe (global)",
    {},
    async () => {
      const data = await client.request("GET", "/client/v1/describe");
      return textResult({ ok: true, describe: data });
    },
  );

  // --- Optional proxy to product MCP gateway tools ---

  if (proxy) {
    server.tool(
      "product_tools_list",
      "List tools from the install product MCP gateway (POST /mcp tools/list)",
      {},
      async () => {
        const result = await client.mcpRpc("tools/list", {});
        return textResult(result);
      },
    );

    server.tool(
      "product_tools_call",
      "Call a product MCP gateway tool by name (POST /mcp tools/call)",
      {
        name: z.string(),
        arguments: z.record(z.unknown()).optional(),
      },
      async ({ name, arguments: args }) => {
        const result = await client.mcpRpc("tools/call", {
          name,
          arguments: args ?? {},
        });
        return textResult(result);
      },
    );
  }

  return server;
}
