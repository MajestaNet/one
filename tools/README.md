# Vendor tools

Helpers used by MajestaNet engineers and CI. **Nothing in this directory is copied into product images.**

| Path | Role |
|---|---|
| [`control-ide/`](./control-ide/) | Control IDE (Electron; Apache-2.0) |
| [`automation-sdk/`](./automation-sdk/) | Type stubs for Deno guest automations (not an npm runtime package) |
| [`one-mcp/`](./one-mcp/) | Optional TypeScript MCP scaffold (`@modelcontextprotocol/sdk`) for stdio / custom vertical tools against a Majesta One install |
| `one-docs/` | Public docs publisher (Astro Starlight) — **not scaffolded yet**; plan [public-docs-site-build-plan.md](../docs/architecture/public-docs-site-build-plan.md) |

Put CLIs, codegen wrappers, and one-off migration aids here. Customer customer customizations do **not** belong here — see [docs/customer-customizations.md](../docs/customer-customizations.md). Connect paths: [docs/customer-connect.md](../docs/customer-connect.md).

Mac developers: [docs/local-development-mac.md](../docs/local-development-mac.md).
