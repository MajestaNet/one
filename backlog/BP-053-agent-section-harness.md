# BP-053: Agent section harness

- **Severity:** High
- **Status:** Mitigated — **Keep** (compat)
- **Area:** `internal/agentharness`

## Outcome

Product harness floors + required `primarySection` with aliases (`run→operate`, `ship→build`). Runtime `Apply` on stream + worker.

## Do not reopen

Job-class source of record is [BP-064](./BP-064-install-agent-runtime.md) / [ADR-030](../docs/adr/030-install-agent-runtime.md). Do not add create-wizard chrome. Customers may not drop below the floor. Design: [agent-section-harness-build-plan.md](../docs/architecture/agent-section-harness-build-plan.md).
