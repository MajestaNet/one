# Control IDE — desktop build & distribution plan

Concrete plan to ship **Majesta One Control** (`tools/control-ide`) as installable desktop apps only (Mac / Linux / Windows). Aligns with [ADR-012](./adr/012-customer-repo-and-control-ide.md): client-only Electron shell; never in product images; no hosted browser product surface.

## Goals

| Goal | Approach |
|---|---|
| Reach Mac, Linux, Windows | One Electron app + `electron-builder` |
| Protect IP (no browser product) | Private installers only; Vite browser mode is vendor-local |
| Easy to maintain | Keep current stack; path-filtered CI; shared release script |
| Stay out of product images | Vendor plane under `tools/`; Dockerfile allowlist unchanged |

## Artifact matrix

| OS | Builder target | Primary artifact | Signing (later) |
|---|---|---|---|
| macOS | `dmg` + `zip` | `Majesta One Control-x.y.z.dmg` | Apple Developer ID + notarize |
| Windows | `nsis` | `Majesta One Control Setup x.y.z.exe` | Authenticode |
| Linux | `AppImage` | `One-Control-x.y.z.AppImage` | Optional GPG later |

Config lives in [`tools/control-ide/electron-builder.yml`](../tools/control-ide/electron-builder.yml).

## Local commands

```bash
cd tools/control-ide
npm ci
npm test                 # Vitest (unit + component)
npm run build            # renderer + main/preload
npm run dist:mac         # requires macOS host for signed builds
npm run dist:win         # Windows host preferred for NSIS
npm run dist:linux       # AppImage on Linux CI
npm run dist             # current OS defaults from electron-builder.yml
```

`npm run dev` (Vite in a browser) is **vendor UI iteration only**. Customer-facing filesystem/git IPC requires Electron (`npm run electron:dev` or a packaged build).

## CI (path-filtered)

[`.github/workflows/ci.yml`](../.github/workflows/ci.yml):

1. **changes** job — `dorny/paths-filter` detects `tools/control-ide/**` vs platform paths. On `pull_request` it lists files via the GitHub API, so the workflow grants `pull-requests: read` (plus `contents: read`).
2. **control-ide** job — runs only when Control IDE paths change: `npm ci`, `npm test`, `npm run build`, Linux AppImage smoke (`dist:linux`).
3. **go** (platform) job — runs only when non-IDE product paths change. **IDE-only PRs do not run `go test`.**

When both areas change in one PR, both jobs run.

## Release track (separate from product `v*`)

Product tags (`vX.Y.Z`) publish api/worker/migrate only — see [release-cicd.md](./release-cicd.md).

Control IDE uses a **separate** tag namespace so desktop packaging never blocks platform releases:

| Step | Detail |
|---|---|
| Version | `tools/control-ide/package.json` `version` |
| Tag | `control-ide-vX.Y.Z` |
| Workflow | `.github/workflows/control-ide-release.yml` (matrix: ubuntu → AppImage, windows → NSIS, macos → dmg/zip) |
| Publish | Private GitHub Release assets **or** private S3/CloudFront — not a public web app |
| Product boundary | Assert still runs; IDE never enters `deploy/Dockerfile` |

Signing secrets (Apple / Windows) are wired in a later milestone; unsigned CI artifacts are fine for internal smoke until then.

## IP / packaging hardening checklist

- [x] Desktop-only distribution plan (this doc)
- [x] asar packaging via electron-builder defaults
- [x] Strip renderer/main/preload source maps in production builds (`vite.config.ts` + `scripts/assert-ide-artifacts.sh`)
- [ ] Code-sign Mac + Windows; notarize Mac
- [ ] Private download channel (no public SPA host)
- [x] Keep secrets and AuthZ on the Go install (IDE stores JWT via `safeStorage` only)

Electron cannot fully hide JS; real protection is **no public web deploy** + **logic stays on the API**. Do **not** rely on `javascript-obfuscator` — asar unpack recovers JS. Signing + private CDN remain frozen ([ADR-030](./adr/030-install-agent-runtime.md)).

## Milestone sequence

1. **Done** — Vitest coverage, path-filtered CI, Linux AppImage smoke, `control-ide-v*` release workflow (unsigned private Release).
2. **Now** — Own CSS design tokens ([control-ide-design.md](./control-ide-design.md)); navy/gold restyle is [BP-068](../backlog/BP-068-ide-brand-visual.md); `electron-updater` scaffold gated on `UPDATE_FEED_URL` (no live feed in CI).
3. **Frozen (ADR-030)** — Private update CDN; Mac notarization + Windows Authenticode; customer download portal; distribution E2E; local/file-based CI update-feed smoke.
4. **Later** — Staged rollouts / dual channels once the private feed exists.

Visual + update architecture: [control-ide-design.md](./control-ide-design.md). Update CDN / signing remain frozen ([ADR-030](./adr/030-install-agent-runtime.md)).

IDE ↔ backend compatibility (API revision pin + soft `PRODUCT_VERSION` tested-against window): [ADR-025](./adr/025-api-revision-versioning.md) · [ide-api-version-compatibility-build-plan.md](./architecture/ide-api-version-compatibility-build-plan.md) ([BP-025](../backlog/BP-025-ide-api-version-compatibility.md)).

## Non-goals

- Hosting Control IDE as a browser SaaS
- Embedding the IDE in ECS/Marketplace images
- Replacing Electron with Tauri / an editor fork for v1
- Running the Go platform suite on IDE-only changes
- Live private-CDN update verification (frozen under ADR-030)
