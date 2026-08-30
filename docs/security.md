# Majesta One security posture

Hardening notes for the API-first, dedicated install platform. This is operational guidance, not a threat model certification.

## AuthN model (current — ADR-006 / ADR-015 / BP-037)

| Principal | Mechanism | Notes |
|---|---|---|
| Users, agents, services | One-issued JWT (`Authorization: Bearer`) | Mint via `/auth/v1/token`, claim, password grant, social/OIDC exchange. `sub` = `users.id`. Interactive human grants also return an opaque refresh token ([BP-063](../backlog/BP-063-refresh-token-sessions.md)); family APIs still require the short-lived access JWT (~1h). |
| Day-0 first admin | `INSTALL_CLAIM_TOKEN` → `POST /auth/v1/install/claim` | Email + password SystemAdmin; works without Control IDE |
| Bootstrap / break-glass | API keys (`Bearer` or `x-api-key`) | Still accepted; may also be exchanged for a JWT via `/auth/v1/token` |
| Humans (customer SSO) | OIDC via install auth / exchange | Preferred steady-state; configure `PUT /metadata/v1/install/auth` |
| Humans (optional social) | Google/Apple/Slack broker when enabled | Not the product default |
| Edge | Platform TLS (+ optional WAF) | JWT validated in-app |

Admin privilege does **not** bypass API family scopes. See AuthZ model below and [ADR-006](./adr/006-jwt-auth.md) / [ADR-009](./adr/009-record-audit-authz-packaging.md).

### Password recovery and system alerts (no product mailer)

Majesta One does **not** send email (no SMTP / SES / SendGrid in product runtime — [BP-038](../backlog/BP-038-no-product-mailer-byo-alerts.md), ADR-011 §10). Prefer **customer SSO** for invites, MFA, and IdP-owned reset. When password login is enabled:

| Need | Mechanism |
|---|---|
| Lost password | Admin `POST /client/v1/principals/{id}/password` (or Control IDE Users → Set password) |
| Change own password | Authenticated `POST /client/v1/me/password` with current + new |
| System alerts to operators | Metadata webhooks on `install.claimed`, `principal.created`, `principal.password_changed` → customer SES/SendGrid/Slack |
| Package transactional contact | Automation `ctx.http` / `ctx.connector` (BP-014) |

No forgot-password-by-email, magic-link, or email OTP (ADR-015 / BP-037 non-goals).

Customer OIDC client secrets are stored with the same `enc:v1` at-rest envelope as
connector and webhook secrets. Installs upgraded from the earlier `plain:` format
remain readable for continuity and should rotate the secret with `PUT
/metadata/v1/install/auth` to rewrite it encrypted. New Google/Apple/Slack JIT users are
accepted only when the provider asserts a verified email.

## AuthN model (transitional extras)

| Principal | Mechanism | Notes |
|---|---|---|
| Users (legacy OIDC) | Optional OIDC JWT (`OIDC_*`, Cognito-compatible) | Mapped via `users.oidc_sub`; auto-assigns `StandardUser` role; **deprecated as product default** |
| Machines without JWT signing key | API keys only | Set `AUTH_JWT_SIGNING_KEY` to enable Token Service |

## AuthZ model

| Layer | Mechanism |
|---|---|
| API family | Scopes `client` \| `metadata` \| `deploy` \| `ops` from **Roles** (ADR-004 / ADR-009) |
| Packaging | Role (required) → scopes only; Permission sets → `user_permission_sets` → object/field |
| Object CRUD | Permission sets (`object_permissions`) — enforced today |
| Record visibility | `CreatedById` or optional `OwnerId` match, or `view_all` / admin |
| Record update/delete | Owner (or creator when OwnerId empty), or `modify_all` / admin — object `can_update`/`can_delete` alone is not enough |
| OwnerId assignment | `modify_all` / admin, or self-assign only |
| Field FLS | `field_permissions` enforced on Client create/read/update/query (**deny-by-default**; OR-union across assigned PSs). Permission-set `dataAccess` returns stored field rows |
| System permissions | Enforced via `permission_sets.system_permissions` (API caps + Control IDE `ide.*` mode/tool chrome) — see [customization-authz.md](./architecture/customization-authz.md) · [system-capabilities.md](./architecture/system-capabilities.md) |
| Metadata writes | System capability checks (`metadata.build`, `authz.manage`, …) — not blanket admin |
| Webhook secrets | Never returned on `GET /metadata/v1/webhooks`; one-time on admin create only |
| Outbox events | Scoped to visible records / acting principal; `data`/`patch` redacted for non-admin |
| Deploy peer baseUrl | Registered peer `baseUrl` rejects link-local/metadata hosts |

Admin privilege does **not** bypass API family scopes. Effective AuthZ (permission sets, field perms, **IsAdmin**, Role scopes) is always loaded from the DB by `sub` when a user row exists — not trusted from JWT/OIDC claims alone. Transitional OIDC **ignores** IdP `admin` / `custom:one_admin` elevation. Principals must have ≥1 Role. See [ADR-006](./adr/006-jwt-auth.md) and [ADR-009](./adr/009-record-audit-authz-packaging.md).

## High-priority follow-ups

1. Finish BP-013/BP-037 remainders: Slack and generic OIDC exchange **shipped**; multi-IdP modeling and operator signing-key rotation remain
2. Identity directory productionization ([BP-017](../backlog/BP-017-identity-directory-productionization.md)): customer Role CRUD, role-on-create, freeze/unfreeze, SCIM-shaped user fields, multi-PS assign UX, `/scim/v2` adapter ([scim-provisioning.md](./architecture/scim-provisioning.md))
3. AuthZ kernel ([BP-003](../backlog/BP-003-enterprise-auth.md)) **mitigated** (FLS, capabilities, sharing). Customer-extendable User + SCIM/JIT provisioning ([BP-058](../backlog/BP-058-user-identity-extension.md)) **mitigated**.
4. Principal parity + hosted agent tool allowlists are mitigated ([BP-006](../backlog/BP-006-agent-guardrails.md)); continue approval-matrix and runtime-budget follow-ups
5. Deploy multi-env: repo→org only (peer push / inbound artifact promote removed)
6. OpenTelemetry OTLP for support SLOs ([BP-008](../backlog/BP-008-production-packaging.md); [outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md)) — redact secrets from spans **and OTEL log records** (`authorization`, token, ciphertext, cookie keys are dropped on the logs exporter)

## Recommended production settings

```bash
APP_ENV=production
DATABASE_URL=...
API_KEYS=...                         # bootstrap / break-glass only; each production key >=32 bytes
AUTH_JWT_SIGNING_KEY=...             # required for /auth/v1; >=32 bytes; not a dev placeholder
WEBHOOK_ENCRYPTION_KEY=...           # >=32 bytes; may omit only when JWT signing key is the fallback
INSTALL_CLAIM_TOKEN=...              # required for day-0 claim in production (BP-037)
AUTH_JWT_ISSUER=https://api.example/auth/v1   # optional; defaults from PLATFORM_PUBLIC_URL
PLATFORM_PUBLIC_URL=https://api.example
# Prefer customer SSO via Metadata install auth; OIDC_* optional env fallback
REQUEST_BODY_LIMIT=1mb
RATE_LIMIT_PER_MINUTE=600
ADMISSION_CLIENT_RPM_SHARE=0.7
AUTH_TOKEN_RATE_LIMIT_PER_MINUTE=30
DEPLOY_SYNC_MAX_FILES=50
DEPLOY_SYNC_MAX_BYTES=2097152
DEPLOY_QUEUE_MAX=8
JOB_SLOTS_DEPLOY=1
CUSTOMER_ID=...
INSTALL_ID=...
INSTALL_ROLE=prod
PRODUCT_VERSION=0.1.0
API_REVISION_CURRENT=1
API_REVISION_MIN=1
FEATURE_FLAGS=                       # enable only the product modules this install needs
# Optional legacy: DEPLOY_SHARE_SECRET / DEPLOY_PEER_MODE (not required; no inbound promote)
# EXPOSURE_RECONCILE / WAF_* — product default is Memory roller; AWS WAFv2 via community sdk/aws when opted in
# IDENTITY_SYNC=cognito — optional via sdk/aws; not required for Path A/B
```

## Edge / exposure defaults

- Default install exposure: `client` + `auth` **public**; `metadata` / `deploy` / `ops` **blocked**
- Customers may lock Client+Auth to `allowlist` + VPN/corp egress CIDRs via `PUT /metadata/v1/install/exposure` + apply (`govern.network`) — WAF updates when a cloud reconciler is wired; recommended when all IDE users are on a VPN
- `clientAccessMode`: `open` (default) | `registered_clients` — controls which Connected Apps may mint/use Client tokens (`azp` on Majesta One JWT). `ide_users` is rejected on write (stored leftovers are treated as `open`)
- `requireDeviceCert`: when true, Client calls need `X-One-Device-Id` for an enrolled device (`POST /client/v1/devices/enroll`)
- Connected App optional `allowedCidrs` merge into Client/Auth allowlists on apply (desired-state JSON unchanged)
- `metadata` / `deploy` / `ops` cannot be set to `public` (use `allowlist` or `blocked`)
- Community ECS reference requires ACM (`certificate_arn`) unless `allow_http=true` (dev only) — see [`sdk/aws`](../sdk/aws/README.md)
- Break-glass: `ADMIN_BREAKGLASS_CIDRS` merged into allowlists; env `API_KEYS` mint with `azp=one.bootstrap`

## Webhooks

- Delivery URLs must be `https://` and must not resolve to loopback/private/link-local/metadata
- Shared secrets are encrypted at rest (`enc:v1:…`) when `AUTH_JWT_SIGNING_KEY` / `WEBHOOK_ENCRYPTION_KEY` is set
- Worker delivery disables HTTP redirects

## Package uploads and OIDC egress

- Customer zip/tar package extraction is capped independently of compressed HTTP size: 128 MiB expanded and 10,000 files.
- OIDC discovery/JWKS requires HTTPS (HTTP loopback only), exact issuer, bounded documents, and no redirects. A configured HTTPS issuer may still resolve to a private address; restrict API egress at the network layer until an explicit private-IdP/SSRF policy is added.

## Outbound connectors (automations)

- Async Deno automations call external HTTPS only via Go host RPC (`ctx.http` / `ctx.connector`) — guest stays deny-net ([BP-014](../backlog/BP-014-agent-outbound-integrations.md))
- Install egress allowlist + connector `base_url`; same SSRF rules as webhooks; redirects disabled
- Provider keys live in `install_secrets` (`enc:v1`); Metadata returns `hasSecret` only; Deploy/Git carry refs, not ciphertext
- Sync automations must not perform outbound I/O
- AgentSpec `allowedSkills` grants named automations; run-as + PS `canRun` still apply when skills execute

## IP / distribution protection

The entire repository is **Apache-2.0**, including Control IDE. Community `sdk/` is Apache-2.0 and **not** product GA. Technical controls raise the bar; they are **not** DRM.

| Surface | Posture |
|---|---|
| Go `api` / `worker` images | Distroless + `-trimpath` + `-ldflags="-s -w"`. **No** default `garble`/obfuscator — operators pull images; obfuscation adds CI cost without stopping determined reverse engineering. |
| Kernel `migrations/` | Shipped readable in-image (boot migrate + support). Product schema, not vendor secrets. |
| Community AWS TF / CFN | Optional under [`sdk/aws/deploy/`](../sdk/aws/deploy/); customer-visible by design. Keep agent docs/backlog out of listing assets. |
| Control IDE | Desktop installers only; asar + production sourcemaps off. No JS obfuscator — logic stays on the Go API; signing + private update channel remain frozen ([ADR-030](./adr/030-install-agent-runtime.md)). |
| Vendor/agent plane + `sdk/` | `docs/`, `backlog/`, `.cursor/`, `AGENTS.md`, `tools/` (except published IDE installers), `scripts/`, `sdk/` never enter product images — enforced by `.dockerignore`, `assert-product-boundary.sh`, and `assert-image-contents.sh`. |
| Secrets | Never commit; store in install secrets. Gitleaks on PR + release. |

**Install shape:** Path A App Platform or Path B Compose/Helm ([self-host.md](./self-host.md)); Control IDE on a **separate** private channel; community AWS materials under `sdk/aws/` are optional Path B extensions — **not** a managed subscription GA.

## Related

- [Client Experience security guide](./client-experience-security.md) (customer browser apps — ADR-019 / BP-040)
- [ADR-006: Majesta One JWT auth](./adr/006-jwt-auth.md)
- [ADR-009: Record audit + AuthZ packaging](./adr/009-record-audit-authz-packaging.md)
- [Community managed-channel notes](../sdk/aws/docs/managed-channel.md) (non-GA)
- [Community managed-channel security](../sdk/aws/docs/managed-channel-security.md) (non-GA)
- [BP-013 Majesta One JWT + unified principals](../backlog/BP-013-jwt-unified-principals.md)
- [BP-003 Enterprise AuthZ (mitigated)](../backlog/BP-003-enterprise-auth.md)
- [BP-058 User identity extension (mitigated)](../backlog/BP-058-user-identity-extension.md)
- [BP-011 Container Marketplace](../backlog/BP-011-container-marketplace-fargate.md)
- [Community AWS Fargate notes](../sdk/aws/docs/aws-fargate.md)
- [ADR-004: Three API families](./adr/004-three-api-families.md)
- [Monorepo / plane boundary](./monorepo.md)
- [Community AWS Marketplace notes](../sdk/aws/docs/marketplace.md)
