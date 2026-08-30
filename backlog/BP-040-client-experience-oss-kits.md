# BP-040: Client Experience + OSS client kits

- **Severity:** High
- **Status:** Partially mitigated (Phases 1–6 landed; remainder **R1** kit wire + tests landed 2026-08-23; partner certification deferred)
- **Track:** Finish (end-user DX). Do not add Control IDE Govern Experiences chrome ([ADR-030](../docs/adr/030-install-agent-runtime.md)).
- **Area:** `sdk/client/` + docs; later `internal/httpapi` Connected Apps defaults; `internal/customerrepo` Experience metadata; optional Control IDE Govern Experiences list
- **Design:** [client-experience-build-plan.md](../docs/architecture/client-experience-build-plan.md) · [ADR-019](../docs/adr/019-client-experience-oss-kits.md) · [ADR-012](../docs/adr/012-customer-repo-and-control-ide.md)
- **Remainder (Finish):** [05-bp-040-client-experience.md](../docs/architecture/agentic-remainders/05-bp-040-client-experience.md) — **R1 landed:** `@one/client` query `{ object, select }` + `/sobjects` + `describe` + `OneAPIError` + `probeVersion`; `@one/auth` sends `One-API-Revision` and rejects metadata/deploy/ops/admin on authorize; kit unit tests (`tsc` + `node --test`). Still open: R2 refresh/revoke/exchange, R3 Experience HTTP tests + template YAML, `@one/react`, partner certification. No new Control IDE chrome.

## Problem

CRM Canvas ([ADR-018](../docs/adr/018-crm-canvas-document.md), [BP-039](../docs/adr/018-crm-canvas-document.md)) scales Operate working sets inside optional Control IDE, but enterprises need **browser and mobile client apps** (list views, portals, telephony UIs) at scale. Without a first-class OSS path, teams either fork an admin IDE, embed unsafe UI in Electron, or call Metadata/Deploy from browser apps with excessive authority.

## Why it matters

- Unblocks enterprise end-user Experiences without weakening Control IDE trust boundary (ADR-012).
- Open-source kits (`sdk/client/`) are auditable and community-extensible while AuthZ stays install-local.
- Secure-by-default Connected Apps (`client` scope only for public clients) reduce the dominant CRM risk: over-scoped browser tokens.

## Direction (locked)

Per **ADR-019**:

1. **Canvas** = Operate tool only; not the sole client strategy.
2. **Client Experience** = customer- or SI-hosted apps using OSS kits + Connected Apps.
3. **API fence:** `/auth/v1` + `/client/v1` only by default for Experiences.
4. **Configure via Deploy** (Experience metadata + Connected App registration); **code** via customer CI on customer infra.
5. **Integrations:** browser vendor SDKs in Experience; server I/O via connectors ([BP-014](./BP-014-agent-outbound-integrations.md)).

## Implementation phases

See [client-experience-build-plan.md](../docs/architecture/client-experience-build-plan.md).

| Phase | Status | Outcome |
|---|---|---|
| 0 | Done | ADR-019, build plan, BP-040, doc amendments |
| 1 | Done | [client-experience-security.md](../docs/client-experience-security.md) |
| 2 | Done | `sdk/client/auth` + `sdk/client/client` scaffold; **R1 (2026-08-23):** live query/sobjects wire + unit tests |
| 3 | Done | Public Connected Apps `client`-only defaults (`internal/integration/scopes.go`) |
| 4 | Done | Experience metadata migration, pack/apply, Metadata API, IDE Govern list |
| 5 | Done | `sdk/client/examples/list-view/` sample |
| 6 | Done | [client-experience-telephony.md](../docs/client-experience-telephony.md); certification deferred |

## Depends on / pairs with

- [BP-013](./BP-013-jwt-unified-principals.md) — Majesta One JWT, Connected Apps
- [BP-022](./BP-022-client-access-ide-device.md) — IDE PKCE; contrast with Experience PKCE defaults
- [BP-014](./BP-014-agent-outbound-integrations.md) — connectors for server-side integration I/O
- [BP-039](../docs/adr/018-crm-canvas-document.md) — Canvas remains Operate-only; not Experience replacement

## Explicit non-goals

- OSS Experience as Operate canvas or Control IDE replacement
- Customer Electron/React plugins in Control IDE
- Product-hosted `/x` SPA router or embedding customer webpack in Go image
- Global code-injection marketplace into `cmd/` / `internal/`
- npm inside Deno guest ([ADR-014](../docs/adr/014-customer-code-automations.md))

## Related

- [customer-connect.md](../docs/customer-connect.md) · [customer-customizations.md](../docs/customer-customizations.md) · [security.md](../docs/security.md)
- [sdk/README.md](../sdk/README.md) · [monorepo.md](../docs/monorepo.md)
- [ADR-018](../docs/adr/018-crm-canvas-document.md) · [ADR-019](../docs/adr/019-client-experience-oss-kits.md)
