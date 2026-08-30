# Client Experience + telephony pairing guide

How to integrate vendor telephony (Twilio Voice, Vonage, etc.) with a customer-hosted **Client Experience** without weakening Majesta One security boundaries.

See [ADR-019](./adr/019-client-experience-oss-kits.md), [client-experience-security.md](./client-experience-security.md), and [BP-014](../backlog/BP-014-agent-outbound-integrations.md).

---

## Split responsibilities

```text
┌─────────────────────────────────────┐
│  Customer Experience (browser SPA)      │
│  Vendor WebRTC / softphone SDK UI     │
│  Majesta One JWT → /client/v1 only        │
└──────────────┬──────────────────────┘
               │ screen-pop: read Case/Contact via Client API
               ▼
┌─────────────────────────────────────┐
│  Majesta One install                    │
│  Client API (sobjects, query)         │
│  Connectors + egress allowlist        │
│  Async Deno automations (optional)    │
└──────────────┬──────────────────────┘
               │ REST webhooks / SMS / call control
               ▼
┌─────────────────────────────────────┐
│  Vendor API (Twilio, etc.)          │
│  Secrets in install secret refs     │
└─────────────────────────────────────┘
```

| Layer | Where | Pattern |
|---|---|---|
| Softphone UI | Experience app | Vendor browser SDK (customer-hosted bundle) |
| CRM screen-pop | Experience app | `@one/client` query/get under user JWT |
| Outbound REST (SMS, call initiate) | Install worker | `ctx.connector` in async automation ([ADR-014](./adr/014-customer-code-automations.md)) |
| Inbound webhooks | Install | Metadata webhook → customer relay or automation trigger |
| Secrets | Install DB | `POST /metadata/v1/secrets` + connector `secretRef` — never in SPA |

---

## Setup steps

### 1. Client Experience (browser)

1. Register a **public** Connected App with PKCE and `client` scope only.
2. Build the Experience with `@one/auth` + `@one/client`.
3. Embed vendor softphone SDK in your customer bundle.
4. On incoming call / screen-pop: `GET /client/v1/sobjects/{object}/{id}` or `POST /client/v1/query` for Case/Contact context.

### 2. Server-side connector (install)

1. `POST /metadata/v1/secrets` — store vendor API key as `enc:v1:…`.
2. `POST /metadata/v1/connectors` — `baseUrl` `https://api.twilio.com` (or vendor host), secret ref.
3. Add host to **egress allowlist** (Metadata install egress config).
4. Async automation: `await ctx.connector("Twilio").fetch("/2010-04-01/Accounts/.../Messages.json", { method: "POST", body: … })`.

### 3. Message / Case linkage (optional)

- Log call outcomes as **Message** or **Case** updates via Client API from automation or Experience.
- Channel adapter boundary for full CTI: [BP-024](./adr/030-install-agent-runtime.md) Phase C.

---

## Compliance notes

- **No npm in Deno guest** — automations use `ctx.connector` only ([ADR-014](./adr/014-customer-code-automations.md)).
- **Audit** — outbound calls via connector emit OTEL spans ([outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md)).
- **PCI/PII** — do not pass card data through Majesta One records unless metadata and PS grants explicitly allow; vendor SDK handles media paths.
- **Certification ladder** for partner connector packs is deferred ([BP-040](../backlog/BP-040-client-experience-oss-kits.md) Phase 6).

---

## Related

- [customer-connect.md](./customer-connect.md) · [automation-sdk.md](./automation-sdk.md)
- [customer-ide-ux.md](./customer-ide-ux.md) — channel boundary
