# BP-055: Run personal graph

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/rungraph`, Client run-graphs

## Outcome

Principal-scoped reference-only graph; hydrate-on-read; never store record payloads as SoR ([ADR-023](../docs/adr/023-run-personal-graph.md)).

## Do not reopen

Graph chrome expansion is frozen. Hosted `graph.*` execution stays on [BP-006](./BP-006-agent-guardrails.md). Coupling cleanup: [BP-065](./BP-065-ide-backend-coupling.md).
