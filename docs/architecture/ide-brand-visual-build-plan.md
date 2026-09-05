# Control IDE — Majesta brand visual restyle

**Status:** Shipped (visual restyle of existing chrome; not a chrome expansion). Merged in [#25](https://github.com/MajestaNet/one/pull/25).  
**Backlog:** [BP-068](../../backlog/BP-068-ide-brand-visual.md) (mitigated)  
**Playbook:** [agent-control-ide.md](./agent-control-ide.md)  
**Domain agent:** `control-ide` (IDE-only). Do not edit `cmd/`, `internal/`, `migrations/`, or `deploy/`.  
**Source of artwork:** [MajestaNet/webpage](https://github.com/MajestaNet/webpage) (`public/brand/`, `src/lib/brand.ts`) — August 2026 identity.  
**Does not reopen:** frozen Control IDE chrome (license, update CDN, Operate-as-CRM, BoardHandoff, Hosting admin UI, four-tile IA, collection-node remainders). See [ADR-030 freeze table](./agent-runtime-build-plan.md#freeze-vs-finish).

Audit date: 2026-09-03 against `tools/control-ide` plus the public webpage brand tree. The review tables below are the **pre-implementation** snapshot; the restyle merged in [#25](https://github.com/MajestaNet/one/pull/25).

---

## Thesis

Control IDE still looks like a generic graphite/teal console. Majesta.Net now has a real identity: reference navy, luminous gold, a globe symbol, and a `MAJESTA.NET` lockup that must be **drawn from SVG**, not typed. This plan brings that identity into the optional desktop client without changing IA, adding panels, or pulling ivory into a data-dense light theme.

```text
webpage (brand SoR)          Control IDE (vendor client)
  navy / gold / white   →      dual theme tokens
  globe + lockup SVGs   →      top-left mark + auth/launcher
  gold-on-navy icon     →      Electron / installer icon
```

The product name in prose stays **Majesta One Control**. The artwork still says `MAJESTA.NET`. Those are not the same string.

---

## Review (current IDE vs brand)

### What the IDE ships today

| Surface | Today | Gap |
|---|---|---|
| Dual theme | `html[data-theme="dark\|light"]` in `styles.css`; persist `one.control.theme` | Tokens are graphite + **teal**, not navy + gold |
| Dark | `#0b0f14` canvas, cool ink, electric teal `#2ee6c5` | No navy field; teal is not a brand color |
| Light | Cool paper `#f4f6f8` + white panels, darkened teal `#0d9f88` | Paper is already closer to “console white” than ivory — keep that instinct |
| Top bar brand | Typed `Majesta One` + teal `Control` (`App.tsx` `.brand`) | Types a wordmark; no logo; teal “Control” is the only brand cue |
| Sign in / mode launcher | Same typed lockup at display size | These screens have width for the real lockup and do not use it |
| App / window icon | `electron-builder.yml` has **no** `icon`; `index.html` has no favicon | Packaged builds get the Electron default |
| Window chrome | `BrowserWindow({ backgroundColor: "#0b0f14" })` | Hardcoded graphite; ignores theme and navy |
| Accent usage | `--accent` is fill, text, border, focus, graph highlight (100+ rules) | A naive gold swap **fails contrast on light** (gold on white ≈ 1.5:1) |
| Typography | Self-hosted IBM Plex Sans / Serif / Mono | Website uses Josefin Sans + Inter. Keep Plex for the dense console. |
| Motion / density | Unchanged and in good shape | Out of scope |

Hardcoded hex besides tokens: `src/main/main.ts` window fill; a few `#000` shadows. Almost every interactive color already goes through `--accent`, so a token remap is the right lever — if we split “mark gold” from “readable accent text”.

### Brand SoR (webpage)

Canonical tokens from `src/lib/brand.ts`:

| Token | Hex | Website use |
|---|---|---|
| Navy | `#1B2E46` | Hero, footer, ~70% of the field; body ink on light |
| Gold | `#F6CF55` | Primary lockup, nav, focus; **not** body text on ivory or white |
| Ivory | `#F5F1E8` | Marketing panels (Projects / Notes) |
| Slate | `#697685` | Secondary text on ivory |
| White | `#FFFFFF` | Reversed marks |

Artwork under `public/brand/` (use the SVG; do not recreate the wordmark in a font):

| File | Role |
|---|---|
| `logo-gold.svg` | Primary digital signature (gold lockup) |
| `logo-navy.svg` | Navy lockup on light panels (site `BrandMark`) |
| `logo-white.svg` | White lockup on dark |
| `logo-two-colour.svg` | Navy wordmark + gold globe |
| `logo.svg` | `currentColor` lockup |
| `symbol.svg` / `symbol-gold.svg` / `symbol-navy.svg` / `symbol-white.svg` | Globe only — compact / favicon scale |
| `symbol/gold-{256,512,1024}.png` | Avatar / app-icon source |

Website rules that apply here:

- Do not type `MAJESTA.NET`. The lockup is supplied SVG.
- Full lockup stays **≥ 180px wide**. Below that, use the globe.
- Gold is the mark on navy. Navy (or white) carries body copy.
- In prose the parent name is Majesta.Net; the product name stays Majesta One Control.

The webpage repo is **proprietary**. This monorepo is Apache-2.0. Do **not** submodule webpage. Vendor a curated copy of the SVGs/PNGs with an explicit trademark notice (WS0).

### Contrast (why ivory and raw gold-as-text are wrong in this app)

| Pair | Approx contrast | Verdict in a dense IDE |
|---|---|---|
| Gold on navy | ~9.3:1 | Use for lockup, primary buttons, focus sparkle on dark |
| White on navy | ~14:1 | Dark-theme ink |
| Navy on white | ~14:1 | Light-theme ink and readable accent text |
| Gold on white | ~1.5:1 | **Fail** as text, links, hover labels |
| Slate on navy | ~3.0:1 | **Fail** as `--muted` on dark — lift it |
| Ivory as light canvas | — | Fine on a marketing scroll; muddy next to tables, Monaco, and graph nodes |

That last row is the reviewer’s agreement with the initial instinct: **light theme is white, not ivory.** Ivory reads as stationery. Control IDE is a console.

---

## Locked decisions

| Decision | Choice |
|---|---|
| Scope | Restyle existing chrome. No new tiles, docks, license, update CDN, or in-IDE agent host. |
| Dark canvas | **Reference navy family** (`#1B2E46` as `--bg` / `--nav`), with a slightly deeper navy for `--bg-deep` / `--panel` recession. Retire graphite `#0b0f14`. |
| Dark ink | **White**, not ivory. Ivory on navy is a landing-page move, not a 12px table. |
| Light canvas | **White panels** on a cool near-white board (`#F5F6F8` or current `#f4f6f8`). **Not** `#F5F1E8`. |
| Light ink | Navy (`#1B2E46`). Slate as `--muted` is OK on white. |
| Accent fill (both themes) | Gold (`#F6CF55`) with navy `--accent-ink` on primary buttons. Brand signature. |
| Accent **text** / borders that must read as type | **Gold on dark; navy on light.** New token `--accent-text` (do not point `--accent` at gold in light and hope). |
| Mark / sparkle | `--brand-gold` in both themes (logo, check-run pulse, graph selection glow). Never as paragraph color on light. |
| Top-left chrome | **Globe symbol** (~22–28px), not the full lockup. Optional typed product word **Control** beside it in IBM Plex. |
| Auth + mode launcher | Full lockup (**≥ 180px**): gold on dark, two-colour (or navy) on light. Product line “Control” under or beside, typed. |
| Wordmark | SVG only. Do not reconstruct `MAJESTA.NET` in IBM Plex / Josefin. |
| Fonts | Keep IBM Plex. Do not add Josefin or Inter for this restyle. |
| App icon | Gold globe on navy square, generated from `symbol/gold-1024.png`. |
| Asset copies | Vendored under `tools/control-ide/assets/brand/`. No runtime fetch of majesta.net. |
| License | Artwork remains Majesta.Net trademarked proprietary art sitting **in** an Apache-2.0 tree. NOTICE + `assets/brand/README.md` say the SVGs are not licensed as Apache-2.0. |
| Go / product plane | Untouched. |

### Token map (implementation target)

Introduce brand primitives, then map role tokens. Names can be bikeshed in the PR; the mapping is binding.

```css
:root {
  --brand-navy: #1b2e46;
  --brand-gold: #f6cf55;
  --brand-white: #ffffff;
  --brand-slate: #697685;
}
```

| Role | Dark | Light |
|---|---|---|
| `--bg` | `--brand-navy` | cool near-white (not ivory) |
| `--bg-deep` | navy mixed toward black (~`#152536`) | slightly cooler gray |
| `--panel` / `--surface-elevated` | lifted navy | `--brand-white` |
| `--nav` | `--brand-navy` | white / 2% navy mix |
| `--ink` | `--brand-white` | `--brand-navy` |
| `--muted` | lifted slate (mix white ~35%) | `--brand-slate` |
| `--line` | gold at ~12% on navy, or lifted navy edge | navy at ~12% |
| `--accent` (fills, focus ring paint) | `--brand-gold` | `--brand-gold` |
| `--accent-ink` | `--brand-navy` | `--brand-navy` |
| `--accent-text` | `--brand-gold` | `--brand-navy` |
| `--accent-dim` | darkened gold | navy at ~80% |
| `--check-run` | `--brand-gold` | `--brand-gold` |
| semantic `--warn` / `--danger` / `--success` / `--info` | keep current hues unless they clash on navy | keep |

`--accent-text` is the mechanical fix for “gold looks right on the landing page and wrong on a light secondary-button hover.” Every `color: var(--accent)` that is **text** (brand span leftover, `.btn-secondary:hover`, status labels) must move to `--accent-text`. Fills and 1px glows stay on `--accent`.

Body wash: replace the teal radial + `rgba(56, 120, 200, 0.07)` with a gold wash on navy (dark) and a faint navy wash on white (light). Keep it quiet — this is a console, not the hero.

Window `backgroundColor`: navy. Update on theme change when cheap (IPC or `nativeTheme`); otherwise navy is the correct flash-of-background for both, and light can paint over it.

---

## Chrome placement

```text
┌─ [globe] Control     [centered Mode title]     Env · theme · account ─┐
```

| Region | Mark | Why |
|---|---|---|
| Top bar, left | Globe + optional “Control” | Lockup would be < 180px in the 3-column top bar; website forbids shrinking it |
| Sign in | Full lockup + “Sign in” | First brand hit; gold on navy when dark, two-colour on white when light |
| Mode launcher | Full lockup + existing tagline | Same as Sign in; overlay uses panel surface so pick the light/dark lockup from `data-theme` |
| Boot skeleton | Globe only (or none) | Avoid layout jump; a11y label stays “Loading Majesta One Control” |
| Footer / status bar | No wordmark | Matches website footer (globe-only if we add anything; default is leave it) |
| Packaged app icon | Gold globe on navy | Matches site Apple touch icon; not the navy-on-transparent tab favicon |

Do not put Operate search in the top bar. Do not replace the centered mode title with a lockup.

`BrandMark` lives under `src/renderer/ui/` (or `icons/`) with `variant: "symbol" | "lockup"` and `theme` from `ThemeContext`. Tests assert `alt` / accessible name includes Majesta.Net and that the compact variant is in the top bar (`data-testid="brand"`).

---

## Workstreams

### WS0 — Vendor artwork + trademark fence

Copy a **curated** subset into `tools/control-ide/assets/brand/`:

- Lockups: `logo-gold.svg`, `logo-navy.svg`, `logo-white.svg`, `logo-two-colour.svg`, `logo.svg`
- Symbols: `symbol.svg`, `symbol-gold.svg`, `symbol-navy.svg`, `symbol-white.svg`
- Raster for icons: `symbol/gold-256.png`, `gold-512.png`, `gold-1024.png`

Add `assets/brand/README.md`: source commit of `MajestaNet/webpage`, file list, “do not type the wordmark”, trademark / not-Apache paragraph.

Update root `NOTICE` with a short “Majesta.Net name and artwork” clause so Apache-2.0 of the surrounding code is not mistaken for a logo license.

Do not copy ivory lockups, `og-image.png`, or site CSS/fonts.

### WS1 — Tokens

Edit `tools/control-ide/src/renderer/styles.css` `:root` / `html[data-theme="light"]` per the token map. Grep for `color: var(--accent)` and `border-color: var(--accent-dim)` used as type; retarget text to `--accent-text`.

Leave semantic success/warn/danger unless gold/navy makes them unreadable (then tweak, do not invent a second palette).

`src/main/main.ts` `backgroundColor` → `#1B2E46`.

### WS2 — Marks in chrome

Replace the three typed `Majesta One <span>Control</span>` sites:

- `App.tsx` top bar → compact `BrandMark`
- `AuthScreen.tsx` → lockup `BrandMark`
- `ModeLauncher.tsx` → lockup `BrandMark`

CSS: `.brand` / `.mode-launcher-brand` become flex rows (img + product word), not display serif with a teal child span.

Keep window title / `productName` as **Majesta One Control**.

### WS3 — App icon + favicon

- Add `tools/control-ide/assets/brand/app-icon.png` (gold globe on navy, ≥1024) and point `electron-builder.yml` `directories.icon` / per-platform `icon`.
- Add a renderer favicon (navy globe on transparent is fine for the Vite tab; packaged icon is the gold-on-navy tile).
- Unsigned CI AppImage smoke already runs; icon presence is enough. Signing remains frozen.

### WS4 — Tests + contrast pass

- Component tests: brand testid on top bar; lockup on auth and launcher; theme toggle still swaps `data-theme`.
- Theme unit tests: unchanged API; optionally assert `--accent-text` differs by theme via computed style if jsdom allows.
- Manual / screenshot pass on: Sign in, launcher, Operate graph, Build Object Manager, Govern users, Settings Environments — **both** themes. Hunt gold-as-text on white and navy-on-navy.
- `npm test` + `npm run build`. `npm run smoke:electron` if WS3 touches main/window icon load.

### WS5 — Design docs (same change set as implementation)

When WS1–WS2 land, update [control-ide-design.md](../control-ide-design.md) (look, dual-theme table, accent rule) and the top-bar line in [customer-ide-ux.md](../customer-ide-ux.md). Do not rewrite those docs ahead of the token PR beyond pointing at this plan.

---

## Explicit non-goals

- Ivory light theme, Josefin/Inter, or recreating the marketing hero inside Electron
- Typing or stroking a fake `MAJESTA.NET` in IBM Plex
- Full lockup in the 44px-tall top bar
- New chrome (about dialog, splash beyond boot skeleton, brand settings)
- Runtime dependency on `majesta.net` or the webpage git repo
- Relicensing webpage artwork as Apache-2.0
- Custom Monaco theme (keep `vs-dark` / `vs` until tokens settle)
- Go API, MCP, `one` CLI, or product images
- Unfreezing license / update CDN / Operate CRM

---

## Verify

```bash
cd tools/control-ide
npm ci
npm run lint
npm test
npm run build
```

From repo root: `make test-ide`. Do not run product `make ci` for this IDE-only restyle.

After WS3 (icons / `main.ts`): `npm run smoke:electron`.

---

## Sequence

1. WS0 (assets + NOTICE) can merge alone.
2. WS1 + WS2 together — tokens without marks still look like a teal-less console; marks without tokens put a gold globe on graphite.
3. WS3 can follow; default Electron icon is ugly but not blocking for token review.
4. WS4 gates merge. WS5 is the same PR as WS1–WS2 or immediately after.

---

## Related

- [control-ide-design.md](../control-ide-design.md) — token system (navy/gold after this plan)
- [customer-ide-ux.md](../customer-ide-ux.md) — top bar IA (geometry stays)
- [agent-control-ide.md](./agent-control-ide.md) — plane fence
- [ADR-030](../adr/030-install-agent-runtime.md) — optional client; restyle ≠ new capability
- [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md) — honesty of API calls; orthogonal to paint
- Webpage `DESIGN.md` / `README.md` § Brand assets — identity rules this plan inherits
