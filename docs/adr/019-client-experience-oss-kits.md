# ADR-019: Client Experience and OSS client kits

## Status

Accepted (plan locked; implementation phased — see [client-experience-build-plan.md](../architecture/client-experience-build-plan.md))

## Context

Majesta One is API-first with three commercial families (Client, Metadata, Deployment) plus Ops ([ADR-004](./004-three-api-families.md)). Control IDE is the licensed surface for Build / Ship / Govern / Operate / **Run**, including declarative **Tools** in Run mode ([ADR-021](./021-run-mode-toolspec.md); document model from [ADR-018](./018-crm-canvas-document.md)). In-IDE Tools do **not** solve every end-user client need (browser list views, branded portals, telephony softphones, mobile shells).

Customers and SIs need a **first-class, open-source path** to build customer-hosted client apps that call the install under the same AuthZ model as users — without forking Majesta One product source, loading arbitrary code into the Electron renderer, or minting Metadata/Deploy scopes from a browser.

An earlier exploration rejected an OSS browser Experience host **as the in-IDE Tool/canvas surface** ([ADR-018](./018-crm-canvas-document.md)): that risks a second IDE inside the product shell. This ADR **reopens** OSS client kits and customer-hosted Experiences as a **separate Client-API track** — the **end-user** surface. Control IDE is optional/frozen ([ADR-030](./030-install-agent-runtime.md)); Experiences do **not** become an admin console and do **not** call Metadata/Deploy from the browser.

## Decision

### 1. Two client surfaces, one AuthZ model

| Surface | Who builds UI | Typical caller | API families (default) |
|---|---|---|---|
| **Control IDE** (licensed) | Vendor (`tools/control-ide`) | Admins, builders, operators | Client + Metadata + Deploy (+ Ops) per JWT scopes |
| **Client Experience** (OSS kits + customer apps) | Customer or SI on customer infra | End users in browser/mobile | **`/auth/v1` + `/client/v1` only** |

Effective AuthZ for both paths remains install-local: Majesta One JWT + Roles + permission sets ([ADR-006](./006-jwt-auth.md), [ADR-009](./009-record-audit-authz-packaging.md)).

### 2. Run Tools are the in-IDE strategy; Experience is the browser strategy

- Run Tools = declarative ToolSpecs inside optional Control IDE ([ADR-021](./021-run-mode-toolspec.md)).
- Experience = customer-hosted browser/mobile apps under this ADR.
- Experience does **not** replace Run Tools; Run Tools do **not** replace Experience portals.

### 3. OSS client kits live under `sdk/client/`

- **License:** Apache-2.0 (same as product plane).
- **In product image?** **No** — excluded from `deploy/Dockerfile` / `.dockerignore` like other `sdk/` trees.
- **Planned packages:** `@one/auth` (PKCE / token exchange / refresh), `@one/client` (typed Client API fetch), optional `@one/react` (hooks/provider only — no Metadata/Deploy helpers).
- **Distinct from** community cloud helpers under `sdk/aws`, `sdk/azure`, `sdk/gcp`.

Kits are the **encouraged** path for customer client apps. They document and default to the secure scope fence; they do not grant capabilities beyond what Connected Apps and permission sets allow.

### 4. Secure-by-default Connected Apps for Experiences

- Register each Experience as a **Connected App** (`/client/v1/integrations`) with PKCE for public/browser clients.
- **Default scopes for Experience apps:** `client` only (plus `/auth/v1` for login).
- **Metadata / Deploy / Ops scopes** are reserved for confidential clients: Control IDE (`one.controlIde`), CI service principals, Deploy bots — not browser SPAs.
- Platform enforcement of this default is phased ([BP-040](../../backlog/BP-040-client-experience-oss-kits.md) Phase 3); documentation and kit defaults apply from Phase 0.

### 5. Install and configure — not code injection into the product

Customers **configure** Experiences on an install; they do **not** load arbitrary UI bundles into the Go runtime or Control IDE renderer.

1. Admin registers Connected App (redirect URIs, allowed origins).
2. Optional: Experience package metadata in `one/v1` (home URL, `connectedAppApiName`, CSP hints) — promoted via Deploy ([client-experience-build-plan.md](../architecture/client-experience-build-plan.md) Phase 4).
3. Customer hosts the built SPA on **their** infra (CDN, App Platform static site, etc.).
4. Customer CI builds and deploys Experience **code**; Majesta One promotes **config** only.

### 6. No second IDE; forks are unsupported

- OSS kits must **not** document or encourage Metadata/Deploy calls from browser apps.
- Forking Control IDE or building an alternate admin console is **unsupported**; security guidance argues for Client-API-only defaults.
- Control IDE remains the only supported surface for Deploy/Metadata authoring.

### 7. Third-party integration SDKs (telephony, maps, etc.)

| Layer | Pattern |
|---|---|
| Browser UI (WebRTC softphone, vendor widget) | Customer Experience app; vendor SDK in customer-hosted bundle |
| Server I/O (REST webhooks, SMS, call control) | Install **connectors** + secret refs + egress allowlist ([BP-014](../../backlog/BP-014-agent-outbound-integrations.md)) |
| Domain logic | Async Deno automations via `ctx.connector` ([ADR-014](./014-customer-code-automations.md)); no npm in guest |

Compliance is **capability-based**: narrow Client scopes, no ambient Metadata authority, auditable outbound via connectors, secrets never in the browser.

### 8. Explicit non-goals (this ADR)

- OSS Experience host as **Run/Operate Tool surface** or Control IDE replacement
- Customer Electron/React/iframe plugins in Control IDE (ADR-012 stands)
- Embedding customer webpack bundles in product images or Electron
- Global marketplace that injects arbitrary code into `cmd/` / `internal/`
- npm / third-party imports inside Deno guest automations (ADR-014 stands)
- Product-hosted `/x` same-origin SPA router in the Go API

## Consequences

- Implementation plan: [client-experience-build-plan.md](../architecture/client-experience-build-plan.md)
- Risk tracking: [BP-040](../../backlog/BP-040-client-experience-oss-kits.md)
- [customer-connect.md](../customer-connect.md) gains Path A split: Control IDE vs Client Experience
- [sdk/README.md](../../sdk/README.md) and [monorepo.md](../monorepo.md) document `sdk/client/` plane
- ADR-018 / ADR-021 cover **in-IDE Tools**; Client Experience is the scalable browser path
- ADR-012 Electron plugin ban unchanged; OSS apps **outside** the IDE are encouraged here

## Related

- [ADR-004](./004-three-api-families.md) · [ADR-005](./005-go-runtime.md) · [ADR-006](./006-jwt-auth.md) · [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-014](./014-customer-code-automations.md) · [ADR-018](./018-crm-canvas-document.md) · [ADR-021](./021-run-mode-toolspec.md)
- [customer-connect.md](../customer-connect.md) · [customer-customizations.md](../customer-customizations.md) · [security.md](../security.md)
- [BP-013](../../backlog/BP-013-jwt-unified-principals.md) · [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) · [BP-022](../../backlog/BP-022-client-access-ide-device.md) · [BP-039](./018-crm-canvas-document.md) · [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) · [BP-050](../../backlog/BP-050-run-mode-toolspec.md)
