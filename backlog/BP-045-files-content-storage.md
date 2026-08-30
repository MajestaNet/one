# BP-045: Files / content storage

- **Severity:** Medium
- **Status:** Open
- **Area:** `internal/dataengine`, `internal/metadata`, `internal/httpapi`, `migrations/`, `deploy/`
- **Design:** remainder: [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md) · [ADR-017](../docs/adr/017-canonical-field-types.md), [ADR-013](../docs/adr/013-high-volume-flexible-storage.md)
- **Identified:** Headless 360 backlog review (2026-07) — explicit omit in ADR-011 with no product path

## Problem

Typical headless CRM stacks expose **Files / ContentDocument / attachments** and often Knowledge binaries. Majesta One explicitly omits Files, Emails, and Knowledge ([ADR-011](../docs/adr/011-sales-service-managed-modules.md) §9). Canonical field types defer **file/blob** ([BP-036](./BP-036-canonical-field-types.md) / ADR-017).

Integrations for Case evidence, Quote PDFs, Account logos, and compliance archives therefore have **no supported product surface** — only BYO object storage URLs in text fields or external systems.

## Why it matters

- Common blocker for Service (Case attachments) and Sales (quote documents) headless UX
- Client Experience portals ([BP-040](./BP-040-client-experience-oss-kits.md)) cannot securely upload/download without a governed API
- ADR-013 notes object storage “later” for large bodies — never backlog’d

## Scope (target)

1. **ADR:** content model — metadata object(s) vs kernel `content_blobs` table; link to parent record (lookup or polymorphic)
2. **Storage backend:** install-local or BYO S3-compatible / DO Spaces — **no Majesta One SaaS CDN**; dedicated install blob prefix per install
3. **Client API:** upload (presigned or streaming), download, list by parent record, delete; virus scan hook optional/deferred
4. **AuthZ:** object + record-parent permission checks ([BP-003](./BP-003-enterprise-auth.md))
5. **Metadata:** optional `file` field type or ContentLink pattern — amend ADR-017 when ready
6. **Size limits:** align with [BP-033](./BP-033-customer-runtime-isolation.md) admission budgets

## Depends on / pairs with

- [BP-003](./BP-003-enterprise-auth.md) — who can read/write attachments
- [BP-033](./BP-033-customer-runtime-isolation.md) — upload size / rate limits
- [BP-040](./BP-040-client-experience-oss-kits.md) — browser upload from Experience apps
- [BP-044](./BP-044-billing-module-order-from-quote.md) — quote PDF generation remains non-goal in sales ADR; storage enables customer-generated docs

## Explicit non-goals

- Third-party files/content API clone
- In-kernel Knowledge articles ([ADR-011](../docs/adr/011-sales-service-managed-modules.md) omit)
- Product-wide CDN or multi-tenant blob fleet
- Email MIME storage (see [BP-038](./BP-038-no-product-mailer-byo-alerts.md), [BP-024](../docs/adr/030-install-agent-runtime.md))
- Full DAM / versioning / legal hold in v1

## Related

- Remainder (slot 11): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)
- [sales-service-data-model.md](../docs/architecture/sales-service-data-model.md) — quote-document generation non-goal
- [security.md](../docs/security.md)
