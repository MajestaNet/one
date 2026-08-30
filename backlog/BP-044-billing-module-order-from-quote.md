# BP-044: Billing module — Order from accepted Quote

- **Severity:** High
- **Status:** Mitigated
- **Area:** `internal/seed`, `internal/packages`, `internal/actions`

## Outcome

Optional `billing` module (Order / OrderLine). Quote → Order is platform action `quote.accept` ([ADR-029](../docs/adr/029-platform-actions.md) / [BP-061](./BP-061-platform-actions.md)). Do not add a sibling `/acceptQuote` route.

## Do not reopen

Invoice and CPQ stay out of scope ([ADR-011](../docs/adr/011-sales-service-managed-modules.md) / [ADR-031](../docs/adr/031-billing-managed-module.md)).
