# System alerts without a product mailer (BP-038)

Build plan for admin/system contact **without** an in-kernel email service. Locked recommendation: customers bring their own transport via webhooks and connectors.

## Locked decisions

1. **No Majesta One mailer** — no SMTP credentials, no SES/SendGrid SDK in product runtime (ADR-011 / BP-024 channel boundary).
2. **Auth recovery without inbox** — admin set password + authenticated self-change; SSO IdP owns invites/MFA/reset for steady-state humans.
3. **Fan-out = existing outbox → webhooks** — closed event set listed below.
4. **Package alerts** — Deno `ctx.http` / `ctx.connector` (BP-014), not `sendEmail()`.
5. **CRM email** — Message ingest + approve-gated send via connector (BP-024 Phase C); out of this plan.

## Event types

| `event_type` | When | Payload (JSON) |
|---|---|---|
| `install.claimed` | Successful `POST /auth/v1/install/claim` | `userId`, `email`, `claimed: true` |
| `principal.created` | Client/SCIM/JIT principal create | `userId`, `email`, `principalType`, `source` |
| `principal.password_changed` | Admin or self password set | `userId`, `actorId`, `source`: `admin` \| `self` |

Subscribe with Metadata webhooks (`eventTypes` includes the type or `*`). Worker delivery is unchanged (HTTPS, SSRF checks, `webhook_deliveries` ledger).

## API

| Method | Path | AuthZ | Body |
|---|---|---|---|
| `POST` | `/client/v1/principals/{id}/password` | `identity.users` (Client scope) | `{ "password": "…" }` (≥10 chars) |
| `POST` | `/client/v1/me/password` | Authenticated JWT/key with Client scope | `{ "currentPassword": "…", "newPassword": "…" }` |

Revokes prior active `password` credentials, stores bcrypt hash, audits, enqueues `principal.password_changed`.

## Phases

| Phase | Status |
|---|---|
| 0 — BP-038 + ADR/docs | This change set |
| 1 — Password set/rotate API + IDE | This change set |
| 2 — Outbox system intents | This change set |
| 3 — BYO recipes (SES/SendGrid/Slack + connector) | This change set |

## Acceptance

- Docs state no product mailer; tech-stack has no mail provider.
- Admin can set a user password without email; self-change requires current password.
- Listed events appear in `outbox_events` and deliver when a matching webhook is configured.
- Recipes in [customer-connect.md](../customer-connect.md) show webhook → provider mapping.

## Non-goals

Email OTP, magic-link, forgot-password-by-email, product “from” domain, managed Emails object.
