# BP-068: Control IDE Majesta brand visual restyle

- **Severity:** Medium
- **Status:** Open (plan landed; implementation not started)
- **Track:** Finish — **optional IDE demo client paint**, not frozen Electron product chrome
- **Area:** `tools/control-ide/**` (tokens, chrome marks, vendored brand assets, electron-builder icon)
- **Design:** [ide-brand-visual-build-plan.md](../docs/architecture/ide-brand-visual-build-plan.md)
- **Related:** [control-ide-design.md](../docs/control-ide-design.md) · [customer-ide-ux.md](../docs/customer-ide-ux.md) · [ADR-030](../docs/adr/030-install-agent-runtime.md) · Brand SoR: [MajestaNet/webpage](https://github.com/MajestaNet/webpage) · Frozen chrome: [ADR-030 freeze table](../docs/architecture/agent-runtime-build-plan.md#freeze-vs-finish)

## Problem

Majesta.Net now has an August 2026 identity (reference navy `#1B2E46`, luminous gold `#F6CF55`, globe symbol, `MAJESTA.NET` SVG lockup). Control IDE still presents as a generic graphite console with an invented teal accent (`#2ee6c5` / `#0d9f88`) and a **typed** “Majesta One Control” wordmark in the top bar, Sign in screen, and mode launcher.

There is no logo in chrome, no app icon in `electron-builder.yml`, and the Electron window flash color is hardcoded graphite. A naive “make light theme ivory” would fight the dense IDE (tables, Monaco, graph). Gold-as-body-text on white fails contrast (~1.5:1).

## Why it matters

The optional desktop client is often the first human surface reviewers see. Graphite/teal does not match majesta.net or the lockup already used on the public site. Restyling existing chrome does not make Electron the product, but it stops the demo from looking like a different company.

## Direction (locked)

Follow [ide-brand-visual-build-plan.md](../docs/architecture/ide-brand-visual-build-plan.md).

| WS | Outcome |
|---|---|
| 0 | Vendored SVG/PNG subset under `tools/control-ide/assets/brand/` + trademark notice (artwork is not Apache-2.0) |
| 1 | Dual-theme tokens: navy dark / **white** (not ivory) light; gold fills; `--accent-text` gold-on-dark / navy-on-light |
| 2 | Globe top-left; full lockup only on Sign in + mode launcher (≥ 180px) |
| 3 | Gold-on-navy app icon for electron-builder + renderer favicon |
| 4 | Vitest + both-theme contrast pass |
| 5 | Update `control-ide-design.md` / top-bar line in `customer-ide-ux.md` with the implementation |

**Do not:** ivory light canvas; type `MAJESTA.NET`; shrink the lockup into the top bar; add chrome; fetch artwork at runtime; relicense webpage art; touch Go / product images.

## Explicit non-goals

- Unfreezing license, update CDN, Operate-as-CRM, or four-tile IA
- Replacing IBM Plex with Josefin Sans / Inter
- Custom Monaco theme in v1
- Making Control IDE the GA path

## Verify

`cd tools/control-ide && npm test && npm run build`. From repo root: `make test-ide`. `npm run smoke:electron` if main/window icon changes. Do not run product `make ci` for this IDE-only restyle.

## Status notes

- **2026-09-03:** Plan and review landed. Implementation not started.
