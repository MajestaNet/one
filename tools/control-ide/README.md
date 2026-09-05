# Majesta One Control IDE

**Version `0.1.0` (alpha).** Optional JWT client. Not a CRM product and not required to install or run agents. **Frozen chrome ([ADR-030](../../docs/adr/030-install-agent-runtime.md)).** New graphs, license, update CDN, and in-IDE agent hosts stay out of scope. **Demo-client honesty** (kill stubs, consume the hosted `/agents/runs` loop, wire shipped family APIs the panels already claim) is [BP-066](../../backlog/BP-066-ide-demo-client-fidelity.md) / [ide-demo-client-uplift-build-plan.md](../../docs/architecture/ide-demo-client-uplift-build-plan.md). Builder DX: [builder-connect.md](../../docs/builder-connect.md).

Client-only Electron + React + Monaco shell for the Majesta One **Customer IDE** experience ([ADR-012](../../docs/adr/012-customer-repo-and-control-ide.md), [customer-ide-ux.md](../../docs/customer-ide-ux.md)).

**This folder must not contain API/provisioning logic.** All backend behavior lives in Go (`internal/`, `cmd/`).

Desktop build & distribution plan: [docs/control-ide-build.md](../../docs/control-ide-build.md).
Visual tokens + dual theme + updates: [docs/control-ide-design.md](../../docs/control-ide-design.md).
Brand restyle (navy/gold + sourced logo, shipped): [ide-brand-visual-build-plan.md](../../docs/architecture/ide-brand-visual-build-plan.md) / [BP-068](../../backlog/BP-068-ide-brand-visual.md).
UX (modes, tiles, agent stream): [docs/customer-ide-ux.md](../../docs/customer-ide-ux.md).
Private update CDN / signing E2E is frozen ([ADR-030](../../docs/adr/030-install-agent-runtime.md)).

## Features

- **Workspace chrome** — Operate / Build / Govern / Settings; Build & Govern exclusive tabs + optional agent chat
- **Dark / light theme** — `data-theme` tokens; toggle in top bar (`Ctrl/Cmd+Shift+L`)
- **Connect** — Majesta One JWT via Settings → Environments; OS `safeStorage` for the session
- **Build → Objects / Automations / Repo** — Metadata API + code automations + commit allowlisted customer paths (no CLI required)
- **Build → Deploy** — pack committed HEAD → Validate vs org → customer tests → Deploy to org; zip upload under Advanced
- **Govern** — users, integrations, permissions
- **Sample customer repo** — `deploy/customer-repo-template` (`CreateAccount_From_Contact` + `Referral__c`); Repo → Initialize from sample
- **Updates** — `electron-updater` gated on packaged app + `UPDATE_FEED_URL` (disabled; private CDN frozen under ADR-030)

## CLI-less Build → Deploy loop

1. Settings → Environments → connect.
2. Build → Repo → **Initialize from sample** (or Clone).
3. Edit Objects / Automations; dual-write into the local repo.
4. Repo → **Commit** → optional Push.
5. Build → Deploy → **Pack from local repo (HEAD)** → **Validate vs org** → suite `CreateAccountFromContact` → **Deploy to org**.

External editors and `cmd/one` remain optional helpers.

## Develop

```bash
cd tools/control-ide
npm ci
npm test                 # Vitest unit + component tests
npm run test:coverage    # coverage thresholds enforced in CI
npm run test:integration # live-API contracts (needs running install; see below)
npm run dev              # Vite UI (browser); filesystem/git IPC needs Electron
npm run electron:dev     # Electron shell
```

Two isolated sessions (switch **before** `.`):

```bash
npm run build
npx electron --user-data-dir="$HOME/.local/share/one-control-ide-a" .
npx electron --user-data-dir="$HOME/.local/share/one-control-ide-b" .
```

`npm run dev` is vendor-local UI iteration only. Customer distribution is **installers**, not a hosted browser app.

**Mac local (API + Postgres + JWT + IDE):** [docs/local-development-mac.md](../../docs/local-development-mac.md). Customer-rollout campaign (2+ IDEs, two installs): [docs/customer-rollout-test-run.md](../../docs/customer-rollout-test-run.md).

### Live-API integration tests

Against a running `make api` install:

```bash
export ONE_API_URL=http://localhost:8080
export ONE_API_KEY=dev-admin-key   # or ONE_JWT=<access_token>
npm run test:integration
# or from repo root: make test-ide-integration
```

Skips cleanly when credentials are unset.

## Installers

`electron-builder.yml` defines mac (dmg/zip), win (nsis), and linux (AppImage). CI path-filters run IDE tests/build when this folder changes; product `go test` is skipped for IDE-only PRs.

```bash
npm run dist:mac
npm run dist:win
npm run dist:linux
```

Tagged releases use `control-ide-vX.Y.Z` (see `.github/workflows/control-ide-release.yml`). Signed publish is a later milestone.

## Auth

Every Majesta One call sends `Authorization: Bearer <one_jwt>`. Effective AuthZ is resolved on the install (ADR-006). Git credentials for CodeCommit are separate from the JWT.
