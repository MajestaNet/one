# BP-064: Install as agent runtime

- **Severity:** High
- **Status:** Mitigated — Phases 0–5 landed
- **Area:** `internal/agentharness`, `internal/mcp`, `internal/httpapi`, `internal/worker`, `cmd/one`

## Outcome

Job-class harness (`query|customize|ship|govern|operate|skill`) with `primarySection` as alias; builder MCP catalog; hosted loop via [BP-006](./BP-006-agent-guardrails.md); CLI templates + keychain; builder skills docs.

## Do not reopen

Remainders are **not** this item: neutralize IDE coupling ([BP-065](./BP-065-ide-backend-coupling.md)), CLI Ship ([BP-048](./BP-048-one-cli.md)), skill invoke ([BP-014](./BP-014-agent-outbound-integrations.md)), inference ([BP-052](./BP-052-customer-inference.md)). Do not add Control IDE chrome.

## Related

- [ADR-030](../docs/adr/030-install-agent-runtime.md) · [agent-runtime-build-plan.md](../docs/architecture/agent-runtime-build-plan.md) · [agentic-remainders/](../docs/architecture/agentic-remainders/README.md)
