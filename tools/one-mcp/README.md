# Majesta One MCP scaffold (vendor plane)

TypeScript MCP server customers (or integrators) can run **outside** the Majesta One product image. It uses the official [`@modelcontextprotocol/sdk`](https://www.npmjs.com/package/@modelcontextprotocol/sdk) over **stdio**, authenticates with Majesta One JWT (`client_credentials` or a static bearer), and calls Client APIs — the same AuthZ model as Path B/C in [docs/customer-connect.md](../../docs/customer-connect.md).

**This is not the product MCP gateway.** The install already exposes Streamable HTTP at `POST /mcp` (Go). Use this scaffold when:

- The host only speaks **stdio** (many desktop MCP clients).
- You need **custom vertical tools** that wrap Client/Metadata HTTP.
- You want one stdio entrypoint that can optionally **proxy** product gateway tools (`ONE_PROXY_PRODUCT_TOOLS=1`).

Do **not** confuse with Deno guest automations ([ADR-014](../../docs/adr/014-customer-code-automations.md)) — those cannot import npm and run inside the worker.

**Prefer the install MCP** (`POST /mcp`) when the host supports Streamable HTTP. This scaffold is stdio fallback + custom vertical tools only. Builder recipes: [docs/builder-connect.md](../../docs/builder-connect.md).

Never copy this tree into `deploy/Dockerfile`.

## Setup

```bash
cd tools/one-mcp
npm install
npm run build
```

## Environment

| Variable | Required | Purpose |
|---|---|---|
| `ONE_BASE_URL` | yes | Install origin, e.g. `https://one.example.com` |
| `ONE_CLIENT_ID` | unless token | Credential id from `POST /client/v1/principals/{id}/credentials` |
| `ONE_CLIENT_SECRET` | unless token | Credential secret |
| `ONE_ACCESS_TOKEN` | optional | Skip mint; use an existing Majesta One JWT |
| `ONE_API_REVISION` | optional | `One-API-Revision` pin on token mint, family HTTP, and `POST /mcp` (default `1`) |
| `ONE_PROXY_PRODUCT_TOOLS` | optional | `1` to register `product_tools_list` / `product_tools_call` |

Create an `agent` (or `service`) principal with Client (and Metadata if needed) scopes — see [customer-connect.md](../../docs/customer-connect.md).

## Local MCP hosts (stdio)

```json
{
  "mcpServers": {
    "one": {
      "command": "npx",
      "args": ["tsx", "src/stdio.ts"],
      "cwd": "/absolute/path/to/tools/one-mcp",
      "env": {
        "ONE_BASE_URL": "https://<install>",
        "ONE_CLIENT_ID": "<credential_id>",
        "ONE_CLIENT_SECRET": "<secret>",
        "ONE_API_REVISION": "1",
        "ONE_PROXY_PRODUCT_TOOLS": "1"
      }
    }
  }
}
```

Or after `npm run build`:

```json
{
  "command": "node",
  "args": ["dist/stdio.js"],
  "cwd": "/absolute/path/to/tools/one-mcp"
}
```

## Prefer remote product MCP when the client supports it

```json
{
  "mcpServers": {
    "one": {
      "url": "https://<install>/mcp",
      "headers": {
        "Authorization": "Bearer <one_jwt>",
        "One-API-Revision": "1"
      }
    }
  }
}
```

## Custom tools

Edit [`src/server.ts`](./src/server.ts): add `server.tool(...)` handlers that call `client.request(...)` against `/client/v1` or `/metadata/v1`. Keep secrets in env; do not bake AuthZ into the scaffold.

## Related

- [docs/builder-connect.md](../../docs/builder-connect.md) — builder MCP + CLI (prefer `/mcp`)
- [docs/customer-connect.md](../../docs/customer-connect.md)
- [docs/customer-agents.md](../../docs/customer-agents.md)
- [ADR-010](../../docs/adr/010-customer-agentic-platform.md)
- [ADR-030](../../docs/adr/030-install-agent-runtime.md)
- [BP-006](../../backlog/BP-006-agent-guardrails.md)
