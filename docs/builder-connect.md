# Builder connect

How coding agents talk to a Majesta One **install**. Control IDE is optional and not required ([ADR-030](./adr/030-install-agent-runtime.md)).

Full design: [architecture/agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md). Connect protocol: [customer-connect.md](./customer-connect.md). Customer AgentSpecs: [customer-agents.md](./customer-agents.md). Hosted `/client/v1/agents/runs` (in-product agents, not an external MCP host) is a **different path**: [hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md).

`one project init` writes customer-owned `AGENTS.md` plus `skills/{connect,query,customize,ship,govern,skill}/SKILL.md` into the customer repo. Copy or edit those after init — they are not vendor `.cursor/` in the product image.

## 1. Install and claim

Path A or Path B per [self-host.md](./self-host.md). Claim the install (`POST /auth/v1/install/claim`) or use break-glass `API_KEYS`. You do **not** need Control IDE.

## 2. Create an agent or service principal

```http
POST /client/v1/principals
```

Use `principalType=agent` (or `service` for CI). Assign Roles (`client`, `metadata`, `deploy` as needed) and permission sets. Issue a credential; mint a JWT with `grant_type=client_credentials` ([customer-connect.md](./customer-connect.md) Path B).

Pin `One-API-Revision` from unauthenticated `GET /version` (`apiRevision.recommended`, an alias of `current`) ([ADR-025](./adr/025-api-revision-versioning.md)). Send that header on MCP and family HTTP.

## 3. Point an MCP host at the install MCP

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

`FEATURE_FLAGS` must include `agents`. Catalog is Client describe/query/CRUD plus agent runs, plus builder tools (`org_validate` / `org_deploy` / `pack` / `org_retrieve`, `invoke_skill` / `invoke_action`, Metadata upserts, `install_version`). Full table: [customer-connect.md](./customer-connect.md#product-mcp-tools-adapter-over-family-http).

MCP invents no capabilities. Missing family scope or capability is `401`/`403`. Ops **mutate** is out of MCP v1.

Stdio fallback: [`tools/one-mcp`](../tools/one-mcp/README.md) — prefer remote `/mcp` when the host supports Streamable HTTP.

## 4. Job-class harnesses

AgentSpecs bind a **job class**. `primarySection` is a compatibility alias for existing YAML. Creating an AgentSpec accepts `jobClass` XOR `primarySection` and fills the other. Customers may widen tools/skills within AuthZ; they **cannot PATCH away the floor**.

| Job class | Harness id | Floor (illustrative) |
|---|---|---|
| `query` | `harness.query.read` | `sobjects.read`, `query`, `search` |
| `customize` | `harness.customize.metadata` | describe + Metadata reads; writes if the customer widens |
| `ship` | `harness.ship.release` | read-heavy; deploy verbs only with `deploy` scope |
| `govern` | `harness.govern.admin` | identity / PS / install policy |
| `operate` | `harness.operate.mutate` | record mutate + platform actions when opted in |
| `skill` | `harness.skill.invoke` | `skills.invoke` ∩ `allowedSkills` ∩ PS `canRun` |

Hosted `/agents/runs` executes a **v1 subset** of MCP names (Client read + gated write + `invoke_skill` / `invoke_action`). Metadata upserts and Deploy `org_*` stay on this MCP / family HTTP path.

## 5. Ship with the CLI (same SoR as MCP)

Install `one` from the product `v*` GitHub Release (`one` is linux/amd64; Mac/Windows use `one-darwin-*` / `one-windows-amd64.exe`) or run `go run ./cmd/one` from a product checkout.

```bash
one project init -dir . --customer-id acme
one auth login --base-url https://<install> --token "$ONE_JWT" --alias test
one org validate -dir .
one org deploy -dir .
```

`org validate` / `org deploy` are equivalent to MCP `org_validate` / `org_deploy` — same Deploy engine, same customer-test gate. Do not peer-promote.

`auth login` stores the credential in the OS keychain when available, otherwise `~/.config/one/credentials.json` (mode `0600`). Override with `ONE_CREDENTIAL_STORE=file|keychain|auto`.

Customer YAML lives in the **customer** Git repo (`one/v1`), never in the Majesta One product tree.

## 6. Rules agents must not break

- AuthZ is install-local. `401`/`403` means the principal lacks grants — do not bypass with bootstrap keys in day-2 work.
- Do not drop harness floors; do not mutate `ownership=managed` metadata.
- Client Experience / browser apps stay `/auth/v1` + `/client/v1` only ([ADR-019](./adr/019-client-experience-oss-kits.md)).
- Do not put customer customizations into the product monorepo ([customer-customizations.md](./customer-customizations.md)).
