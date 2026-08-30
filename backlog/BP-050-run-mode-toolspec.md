# BP-050: Control IDE Run mode + ToolSpec

- **Severity:** High
- **Status:** Mitigated
- **Area:** `tools/control-ide`; Metadata ToolSpec

## Outcome

Declarative in-IDE Tools via ToolSpec ([ADR-021](../docs/adr/021-run-mode-toolspec.md)). Historical Operate canvas (ADR-018) is not the product surface.

## Do not reopen

Do not expand ToolSpec chrome. Removing chrome Client routes / `ide.*` caps is [BP-065](./BP-065-ide-backend-coupling.md). Honest JWT consumption of shipped APIs is [BP-066](./BP-066-ide-demo-client-fidelity.md). End-user CRM is Client Experience ([BP-040](./BP-040-client-experience-oss-kits.md)).
