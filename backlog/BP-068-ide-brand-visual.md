# BP-068: Control IDE Majesta brand visual restyle

- **Severity:** Medium
- **Status:** Mitigated — WS0–WS5 landed in [#25](https://github.com/MajestaNet/one/pull/25)
- **Track:** Keep — optional IDE demo client paint (not frozen Electron product chrome)
- **Area:** `tools/control-ide/**` (tokens, chrome marks, vendored brand assets, electron-builder icon)
- **Design:** [ide-brand-visual-build-plan.md](../docs/architecture/ide-brand-visual-build-plan.md) (shipped)
- **Related:** [control-ide-design.md](../docs/control-ide-design.md) · [customer-ide-ux.md](../docs/customer-ide-ux.md) · [ADR-030](../docs/adr/030-install-agent-runtime.md) · Brand SoR: [MajestaNet/webpage](https://github.com/MajestaNet/webpage) · Frozen chrome: [ADR-030 freeze table](../docs/architecture/agent-runtime-build-plan.md#freeze-vs-finish)

## Outcome

Control IDE chrome uses Majesta navy / gold / white tokens, a sourced globe in the top bar, the full `MAJESTA.NET` SVG lockup on Sign in and the mode launcher, and a gold-on-navy app icon. Graphite/teal and typed wordmarks are gone.

| WS | Landed |
|---|---|
| 0 | Vendored SVG/PNG subset under `tools/control-ide/assets/brand/` + trademark notice (artwork is not Apache-2.0) |
| 1 | Dual-theme tokens: navy dark / **white** (not ivory) light; gold fills; `--accent-text` gold-on-dark / navy-on-light |
| 2 | Globe top-left; full lockup only on Sign in + mode launcher (≥ 180px) |
| 3 | Gold-on-navy app icon for electron-builder + renderer favicon |
| 4 | Vitest + both-theme contrast pass |
| 5 | `control-ide-design.md` / top-bar line in `customer-ide-ux.md` updated |

## Do not reopen

Do not retoken back to teal, type `MAJESTA.NET`, shrink the lockup into the top bar, add chrome, fetch artwork at runtime, relicense webpage art, or touch Go / product images. Custom Monaco theme was an explicit non-goal. Demo-client honesty remains [BP-066](./BP-066-ide-demo-client-fidelity.md).

## Status notes

- **2026-09-03:** Plan landed ([#24](https://github.com/MajestaNet/one/pull/24)).
- **2026-09-03:** Implementation merged ([#25](https://github.com/MajestaNet/one/pull/25)) — vendored artwork, navy/gold tokens, BrandMark chrome, app icon. `npm test` (661), `npm run lint` (no new errors), `npm run build`, `npm run smoke:electron` (10/10).
- **2026-09-05:** Status aligned to Mitigated (the implementation PR had left this item “Open (implementation PR)”).
