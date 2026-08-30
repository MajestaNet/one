---
name: one-connect
description: Connect a coding agent to a Majesta One install via MCP and one. Use when pointing an MCP host at POST /mcp, minting a JWT, or pinning One-API-Revision.
---

# Connect to a Majesta One install

The install **is** the MCP server. Do not author a product MCP server in this customer repo.

## MCP (preferred)

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

Pin `One-API-Revision` from unauthenticated `GET /version` (`apiRevision.recommended` or `current`). The install must include `agents` in `FEATURE_FLAGS`.

Mint a JWT with `grant_type=client_credentials` for an `agent` or `service` principal, or use a short-lived user JWT.

## CLI (Ship + retrieve)

```bash
one auth login --base-url https://<install> --token "$ONE_JWT" --alias test
```

Credentials prefer the OS keychain and fall back to `~/.config/one/credentials.json` (mode `0600`). Force the file store with `ONE_CREDENTIAL_STORE=file`.

## Do not

- Use bootstrap `API_KEYS` for day-2 builder work.
- Call Metadata or Deploy from a browser Client Experience.
- Treat Control IDE as required. It is an optional JWT client of the same APIs.
