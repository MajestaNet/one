# BP-025: Control IDE ↔ API version compatibility (API revision pin + tested product window)

- **Severity:** High
- **Status:** Mitigated (ADR-025 implemented: discovery, middleware, `/r{N}/` alias, Connect/CLI/SDK pins, packaging `API_REVISION_*`. **Pin gaps closed:** datapack `OrgClient`, `@one/auth`, `tools/one-mcp`, MCP revision tests. Phase 4 per-revision adapters deferred until a real Client wire break; hosted-loop pin persist remains that Phase 4 blocker.)
- **Area:** `tools/control-ide`, `internal/httpapi` (`GET /version`, `GET /client/v1/me`, revision middleware), `internal/config` (`API_REVISION_*`); CLI `cmd/one`; later `sdk/client`
- **Design:** [ADR-025](../docs/adr/025-api-revision-versioning.md) · [ide-api-version-compatibility-build-plan.md](../docs/architecture/ide-api-version-compatibility-build-plan.md) · [remainder (slot 10)](../docs/architecture/agentic-remainders/10-bp-025-api-revision.md) · [control-ide-build.md](../docs/control-ide-build.md) · [control-ide-design.md](../docs/control-ide-design.md)

## Problem

Licensed Control IDE (and other clients) have no **pinnable wire contract** that survives install upgrades. Pairing an old IDE with a new OSS backend (or the reverse) yields silent protocol drift. An earlier draft gated Connect on `PRODUCT_VERSION` ± N minors; that solves a tooling test-matrix concern but **inverts** classic API-platform upgrades (new install blocks old clients) and overloads Ops/Deploy product semver as a wire SoR. Relying on Deploy-scoped `/environment` alone also fails for typical human Connect JWTs (no `deploy` scope).

## Why it matters

Commercial / DX promise (amended):

1. **Clients pin an API revision** (`One-API-Revision` / optional `/r{N}/`). The install advertises `[min, current]` and keeps those revisions working across product image rolls. Raising `min` forces migration with lead time.
2. **IDE/CLI declare a tested product window** (default latest **N = 2** minors) as a **soft** signal — warn when outside the matrix, do not hard-block Connect once revision negotiation exists.

Without both, Connect/release coupling and CI cannot enforce compatibility for IDE, `one` ([BP-048](./BP-048-one-cli.md)), or Experience kits ([BP-040](./BP-040-client-experience-oss-kits.md)). Treat this BP as a **production DX safety prerequisite** alongside runtime isolation ([BP-033](./BP-033-customer-runtime-isolation.md)).

## Scope

Detailed tech design + phases: **[ide-api-version-compatibility-build-plan.md](../docs/architecture/ide-api-version-compatibility-build-plan.md)** · locked axes: **[ADR-025](../docs/adr/025-api-revision-versioning.md)**.

1. **Contract** — Clients declare `preferredApiRevision` / `minApiRevision` + soft `targetProductVersion` / `supportedProductMinors` (default **N = 2**)
2. **Discovery** — unauth `GET /version` (+ `/client/v1/me`) returns `productVersion` **and** `apiRevision: {min,current}` (+ `httpApi` map); no Deploy scope required
3. **Pin transport** — `One-API-Revision` header on family calls; optional `/client/v1/r{N}/…` alias; **not** `/client/v12` replacing family major
4. **Handshake gate** — Connect/CLI **hard-block** on revision outside window; **warn** on product tested-against skew; override + loopback/dev exceptions
5. **Middleware** — enforce pin ∈ `[min,current]`; stash revision on request context; adapters when behavior diverges
6. **Release coupling** — revision changelog + `min` sunset policy; IDE/`control-ide-v*` ↔ preferred revision; product tags remain separate
7. **CI** — unit matrix; env-simulated / last-N live checks; SDK pin (Phase 3)
8. Optional later: `minControlIdeVersion`; first real per-revision Client adapter

## Explicit non-goals (now)

- Using `PRODUCT_VERSION` as the client pin or sole hard Connect SoR (superseded by ADR-025)
- License entitlement / seat activation (deferred; see [BP-015](../docs/adr/030-install-agent-runtime.md))
- Requiring AWS or a managed channel for version discovery
- Embedding the IDE in product images
- HTTP family URI `/v2` or flat `/v1` alias removal (ADR-004 separate track)
- Replacing document-schema versions (`one.runGraph/v1`, etc.)

## Related

- [ADR-025](../docs/adr/025-api-revision-versioning.md) — three-axis model + pin SoR
- [BP-015](../docs/adr/030-install-agent-runtime.md) — delivering IDE upgrades once incompatible
- [BP-022](./BP-022-client-access-ide-device.md) — who may connect (not which versions)
- [BP-040](./BP-040-client-experience-oss-kits.md) — `sdk/client` must pin revisions
- [BP-048](./BP-048-one-cli.md) — CLI productization (shares version/compat concerns)
- [BP-033](./BP-033-customer-runtime-isolation.md) — sibling production DX safety prerequisite
- [ADR-004](../docs/adr/004-three-api-families.md) · [ADR-012](../docs/adr/012-customer-repo-and-control-ide.md)
