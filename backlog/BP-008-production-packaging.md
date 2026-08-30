# BP-008: Observability (OpenTelemetry) gaps

- **Severity:** Medium
- **Status:** Partially mitigated (OTLP SDK wired; Node packaging closed)
- **Area:** `cmd/`, `internal/otel`, `internal/config`, `internal/httpapi`, `deploy/`
- **Design:** [outbound-otel-build-plan.md](../docs/architecture/outbound-otel-build-plan.md)
- **Remainder:** [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)

## Problem

Platform runtime is Go distroless (ADR-005 / BP-012). Structured JSON logs (`log/slog`) and request access lines ship; OpenTelemetry OTLP export is now available when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Why it matters

Production diagnostics for operators running Compose/Helm (or optional ECS) installs.

## Shipped

- `internal/otel` OTLP HTTP traces/metrics; no-op when endpoint unset
- Resource attrs: `PRODUCT_VERSION`, `CUSTOMER_ID`, `INSTALL_ID`, `INSTALL_ROLE`
- HTTP middleware spans + `trace_id`/`span_id` on access logs
- Worker job spans; outbound connector/http spans with redacted URLs
- Optional OTEL logs exporter: stdout JSON `log/slog` always remains; fan-out to OTLP only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set **and** `OTEL_LOGS_EXPORTER=otlp` (default **none**). Span context is attached on log records; `authorization` / token / ciphertext / cookie keys are dropped on the OTEL handler. Distroless image unchanged (no collector sidecar)
- Docs: ops, tech-stack, security, `.env.example`, Helm optional env

## Remaining

- Pair BP-033 queue-depth metrics when isolation budgets land. This tree has **no** worker job class/slot column on `jobs` (`agent_playbooks.job_class` is the BP-064 harness floor, not BP-033 isolation). Do not ship an undifferentiated `one.jobs.queue_depth` gauge that would be renamed per-class.

## Closed

- Node/`tsx` monorepo packaging — removed with Go-only purge

## Related

- Remainder design: [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)
- [outbound-otel-build-plan.md](../docs/architecture/outbound-otel-build-plan.md)
- [BP-014](./BP-014-agent-outbound-integrations.md)
- [docs/ops.md](../docs/ops.md) · [docs/tech-stack.md](../docs/tech-stack.md) · [docs/security.md](../docs/security.md)
