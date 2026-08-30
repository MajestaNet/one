#!/usr/bin/env node
/**
 * Stdio MCP entry — for local MCP hosts.
 *
 * Env:
 *   ONE_BASE_URL              https://<install>
 *   ONE_CLIENT_ID             principal credential id
 *   ONE_CLIENT_SECRET         secret
 *   ONE_ACCESS_TOKEN          optional static bearer (skips mint)
 *   ONE_PROXY_PRODUCT_TOOLS   set to "1" to proxy product /mcp tools
 */
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { clientFromEnv } from "./one-client.js";
import { buildOneMcpServer } from "./server.js";

async function main() {
  const client = clientFromEnv();
  const server = buildOneMcpServer({ client });
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
