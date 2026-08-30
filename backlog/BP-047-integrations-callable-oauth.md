# BP-047: Integrations — Client-callable automations + outbound OAuth

- **Severity:** Medium
- **Status:** Partially mitigated (Phases 0–4 and IDE Govern connector catalog implemented; ExecutionRun projection remains)
- **Area:** `internal/httpapi`, `internal/worker`, `internal/automation`, `internal/db`, `internal/deploy`, `internal/connectoroauth`, `migrations/`
- **Design:** [integrations-build-plan.md](../docs/architecture/integrations-build-plan.md) · [ADR-014](../docs/adr/014-customer-code-automations.md) · [outbound-otel-build-plan.md](../docs/architecture/outbound-otel-build-plan.md)
- **Remainder:** [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)

## Problem

1. Integrations authenticate with Majesta One JWT / Connected Apps but can only **indirectly** trigger automations (via record writes). There is no Client invoke path, so integration authors cannot treat customer automations as callable platform functions under permission sets.
2. Outbound connectors support **static Bearer** secrets only. Customers that need OAuth (authorization code or client credentials) have no platform config for flow specs, consent, token storage, or refresh — pushing them toward sidecars or forbidden guest networking.

## Why it matters

Integrations are a primary product surface. Without Client invoke + platform OAuth, customers either over-privilege record writes as a trigger hack or leave Majesta One to implement OAuth themselves — both weaken AuthZ clarity and raise support burden.

## Plan

See [integrations-build-plan.md](../docs/architecture/integrations-build-plan.md) Phases 0–4.

| Phase | Outcome |
|---|---|
| 0 Docs | Plan + this BP + API-family / connect cross-links |
| 1 Client invoke | `GET/POST` automations runs under caller AuthZ |
| 2 Connector auth model | `auth_type` + `oauth_flow`; token/state tables |
| 3 OAuth runtime | `/auth/v1` authorize/callback; host refresh |
| 4 Deploy + DX | Promote defs/refs; recipes; re-consent; Govern catalog UX |

## Related

- Remainder design: [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)
- [BP-009](./BP-009-no-in-kernel-language.md) — Manual Run remainder owned here
- [BP-014](./BP-014-agent-outbound-integrations.md) — static connectors shipped; OAuth extension here
- [BP-033](./BP-033-customer-runtime-isolation.md) — future `ExecutionRun` for invoke status
- [BP-024](../docs/adr/030-install-agent-runtime.md) — Govern connector registry UI shipped; channel send contract remains
- [BP-040](./BP-040-client-experience-oss-kits.md) — Connected Apps / public PKCE callers of invoke

## Explicit non-goals

- Per-user OAuth; BYO LLM; sync outbound; Deno `fetch`; inbound provider webhooks
