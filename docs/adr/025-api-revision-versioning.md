# ADR-025: API revision versioning (three-axis model)

## Status

Accepted (design locked; implementation via [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md) / [ide-api-version-compatibility-build-plan.md](../architecture/ide-api-version-compatibility-build-plan.md)).

## Context

Majesta One already has several version-like strings that agents and operators conflate:

| Concept | Example | True role |
|---|---|---|
| HTTP family major | `/client/v1` | ADR-004 commercial family wire; rare `/v2` is a breaking-family event |
| Product version | `PRODUCT_VERSION=0.14.0` | Image / Ops roll / Deploy bundle ranges |
| Document schemas | `one.runGraph/v1` | Payload shapes inside a family |
| IDE / CLI build | `control-ide-v0.4.0` | Desktop/tooling release |

[BP-025](../../backlog/BP-025-ide-api-version-compatibility.md) originally proposed gating Control IDE Connect on **product** semver (“latest N minors”). That solves a narrow **tooling test-matrix** problem, but it is the wrong SoR for **wire stability**:

1. Product minors bump for kernel, Ops, managed packages, and packaging — not only Client wire changes.
2. “Install too new → block IDE” **inverts** the classic API-platform upgrade model (new platform keeps old clients working until sunset).
3. Experience kits (`sdk/client`), agents, and iPaaS get no pinnable contract — only Connect/CLI would gate.
4. ADR-004 leaves a cliff between eternal `/v1` additive change and rare `/v2`; there is no graduated deprecation for wire behavior.

Operators and optional IDE need the familiar property: **clients pin a revision**; the install may already support a newer revision; upgrades must not break pinned clients; clients migrate later under a published sunset.

## Decision

### 1. Three axes (do not conflate)

| Axis | Identifier | Who sets / pins | Changes when |
|---|---|---|---|
| **A. Product version** | `PRODUCT_VERSION` (`X.Y.Z`) | Install operator / Ops roll | Image ships; Deploy bundle ranges; peer provision |
| **B. HTTP family major** | `/client/v1`, `/metadata/v1`, … | ADR-004 | Deliberate breaking-family event only |
| **C. API revision** | Monotonic integer `N` (e.g. `12`, `14`) | **Clients pin**; install advertises `min`/`current` | Wire-incompatible or negotiated behavior change inside family majors |

Document schemas (`one.*/vN`) remain a **fourth, payload-local** axis and are unchanged by this ADR.

### 2. API revision is the client pin SoR

- Every product image advertises:

  ```json
  {
    "productVersion": "0.14.0",
    "apiRevision": { "current": 14, "min": 12 },
    "httpApi": {
      "client": "v1",
      "metadata": "v1",
      "deploy": "v1",
      "ops": "v1",
      "auth": "v1"
    }
  }
  ```

  on unauthenticated `GET /version` (and echoed where clients already call `/client/v1/me` / Deploy environment).

- Clients (Control IDE, `one`, `sdk/client`, agents, integrations) **pin** an integer revision for the session/org config.
- The install **supports** all revisions in `[min, current]` concurrently.
- Ops may roll `productVersion` forward without forcing clients to move their pin, as long as the pinned revision remains ≥ `min`.
- Raising `min` is the **forced-migration** lever (with release-note lead time). Clients below `min` are rejected with a stable error code and upgrade CTA.

### 3. How clients express the pin (prefer header)

**Preferred:** keep ADR-004 family paths stable; send the pin on every family request:

```http
GET /client/v1/sobjects/Account
Authorization: Bearer …
One-API-Revision: 12
```

**Allowed alternate (same semantics):** nest under the family major without changing family ownership:

```http
GET /client/v1/r12/sobjects/Account
```

Do **not** invent `/client/v12` as a replacement for `/client/v1` — that conflates axis B with axis C and explodes route ownership across families.

Default when the header/`/rN` segment is omitted: treat as `apiRevision.current` for that image (documented; SDKs should still send an explicit pin).

Unknown or below-`min` revision → `400` / `426` with code `API_REVISION_UNSUPPORTED` (exact status locked in the build plan; prefer `400` with machine code for v1 consistency unless HTTP semantics demand otherwise).

### 4. Product version remains tooling / Ops SoR — soft for Connect

- `PRODUCT_VERSION` stays authoritative for Ops upgrades, Deploy `productVersionRange`, and peer cloud provision.
- Control IDE / CLI may **warn** when the install’s product minor is outside the client’s *tested-against* window (`targetProductVersion` + `supportedProductMinors`).
- Connect must **not** hard-block solely because `productVersion` is newer/older than that window, once API revision negotiation exists.
- Hard block (with break-glass override) applies when:
  - pinned `apiRevision` is outside install `[min, current]`, or
  - revision is missing/unparsable in production (policy in build plan), or
  - (optional Phase 4) install advertises `minControlIdeVersion` and the IDE build is older.

### 5. Mapping product releases → revision bumps

| Change type | Bump |
|---|---|
| Additive field / new optional endpoint / new capability bit | Usually **no** revision bump (stay on `current`; document in capabilities) |
| Behavior change that breaks a pinned client’s expectations | **Bump `current`**; keep old revision adapter until sunset |
| Removing a revision adapter | Raise **`min`** (forced migration) |
| Entire family rewrite incompatible with `/v1` | New **family major** (`/client/v2`) per ADR-004 — orthogonal to revision integers |

`current` may track product minor for early Majesta One (e.g. product `0.14.x` → revision `14`) as a **convenience**, but the integer is an independent SoR — never parse `PRODUCT_VERSION` as the pin.

### 6. Scope of multi-revision support

v1 of this ADR requires revision awareness on **Client** family first (highest external surface: IDE Operate/Run, Experience kits, agents, iPaaS). Metadata / Deploy / Ops should advertise the same `apiRevision` window and accept the header for consistency; deep per-revision adapters on those families land as needed when breaking changes appear.

## Consequences

- BP-025 implementation must deliver discovery, pin persistence, middleware, and Connect/CLI gates around **API revision**, with product-version handshake demoted to soft “tested-against.”
- Go handlers gain a small revision context (middleware) and, over time, adapters or branched behavior keyed by `N` — not a second product tree.
- Release docs must publish revision changelog + sunset (`min` raise) alongside product tags.
- ADR-004 family majors remain rare; revision integers absorb most graduated breakage.
- Flat `/v1` alias deprecation remains a separate ADR-004 track.

## Non-goals

- Multi-tenant SaaS router or shared version fleet across customers
- GraphQL versioning
- Versioning customer *metadata shapes* via URL (schema is Metadata-driven; clients use describe)
- Embedding Control IDE in product images
- Replacing document-schema `apiVersion` strings (`one.runGraph/v1`, etc.)

## Related

- [ADR-004](./004-three-api-families.md) — family majors
- [ADR-007](./007-platform-ops-upgrades.md) — product image rolls
- [ADR-012](./012-customer-repo-and-control-ide.md) — Control IDE as client
- [BP-025](../../backlog/BP-025-ide-api-version-compatibility.md) — implementation backlog
- [ide-api-version-compatibility-build-plan.md](../architecture/ide-api-version-compatibility-build-plan.md) — phases + wire details
- [BP-040](../../backlog/BP-040-client-experience-oss-kits.md) / [ADR-019](./019-client-experience-oss-kits.md) — `sdk/client` must pin revisions
- [BP-048](../../backlog/BP-048-one-cli.md) — CLI parity
