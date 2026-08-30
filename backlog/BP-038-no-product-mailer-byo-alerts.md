# BP-038: No product mailer — system notification intents via BYO

- **Severity:** Medium
- **Status:** Partially mitigated
- **Area:** `internal/db`, `internal/httpapi`, `internal/worker`, `tools/control-ide`, `docs/`
- **Plan:** [system-alerts-byo-build-plan.md](../docs/architecture/system-alerts-byo-build-plan.md)
- **Related:** [BP-014](./BP-014-agent-outbound-integrations.md), [BP-024](../docs/adr/030-install-agent-runtime.md), [BP-037](./BP-037-install-claim-customer-sso.md), [ADR-011](../docs/adr/011-sales-service-managed-modules.md), [ADR-015](../docs/adr/015-idp-agnostic-social-login.md)

## Problem

Customers need admin/system alerts (claim, password change, principal create) and packages may want transactional contact. Building an in-kernel SMTP/SendGrid/SES mailer would fight ADR-011 channel boundaries, create deliverability/ops burden on every install, and conflict with dedicated install “no SaaS fleet mail plane.”

## Decision

**Majesta One does not ship a product mailer.** Email remains an identity attribute / field type. Auth recovery is admin/self password set in product (no inbox OTP). System events fan out through the existing outbox → webhook pipeline; packages/automations send via `ctx.http` / `ctx.connector` (BP-014).

## What shipped

1. Docs + ADR-011 note: no product SMTP; BYO transport
2. Admin `POST /client/v1/principals/{id}/password` + self `POST /client/v1/me/password`
3. Outbox event types: `install.claimed`, `principal.password_changed`, `principal.created`
4. Control IDE Users panel “Set password”
5. Customer-connect recipes for SES / SendGrid / Slack webhooks and connector send

## Remaining

- Optional package-declared custom notification intents
- Later: `security.session_revoked` and similar if needed
- CRM inbound/outbound email remains [BP-024](../docs/adr/030-install-agent-runtime.md) Phase C

## Explicit non-goals

- SMTP / SendGrid / SES / Mailgun SDK in `cmd/api` or `cmd/worker`
- Product-owned “from:” domain or deliverability stack
- Email MFA / magic-link / verification inbox
- Managed Emails CRM object
- Control IDE push as a substitute inbox
