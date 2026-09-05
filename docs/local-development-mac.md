# Local development on macOS

Run Majesta One API + Postgres and exercise Control IDE on a Mac (Apple Silicon or Intel). Product runtime is Go; the IDE is vendor-plane Electron under `tools/control-ide` ([ADR-012](./adr/012-customer-repo-and-control-ide.md)).

## Prerequisites

Install with [Homebrew](https://brew.sh):

```bash
brew install go@1.25 node docker deno
# optional
brew install golangci-lint
```

- **Go** 1.25+ (`go version`)
- **Node** 22.22.2+ (`node -v`) — Control IDE / Electron 43 / jsdom 30; Node 20 is insufficient for current Electron engine requirements
- **Deno** 2.9.3+ (`deno --version`) — guest customer automations and Deploy `automationUnitPass` / `automationContract` suites (`deploy/Dockerfile` installs the same version). Without Deno on `PATH`, `one org deploy --suite` fails after apply with “deno binary not found”
- **Docker Desktop** — local Postgres (or any Postgres 16+ URL)
- Open Docker Desktop before Compose commands

On Linux, Control IDE prefers the OS keyring (`safeStorage`) for the session. If no secret service is available, it still **persists** the session with a local AES-256-GCM file (mode `0600`) so closing and reopening the app does not require sign-in. The JWT is never written as cleartext (CIDE-10).

Clone the repo and enter it:

```bash
git clone <your-one-remote> one
cd one
```

To ship customer metadata from this Mac, download `one-darwin-arm64` or `one-darwin-amd64` from the product `v*` GitHub Release, or run `go run ./cmd/one` from this tree.

## 1. Environment

From the **repo root** (the directory that contains `Makefile` and `.env.example`):

```bash
go mod download
cp .env.example .env
```

`.env` is gitignored — it will not exist until you copy it. `make api`, `make worker`, `make migrate`, and `go run ./cmd/api` auto-load `.env` from the repo (already-exported shell variables win). You can still export it yourself:

```bash
set -a && source .env && set +a
```

Important `.env` values for local IDE work:

| Variable | Local default | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://one:one@localhost:5432/one` | Compose Postgres — **required** for social login |
| `API_KEYS` | `dev-admin-key+admin,dev-agent-key:client` | Bootstrap keys |
| `AUTH_JWT_SIGNING_KEY` | set in `.env.example` | Required to mint Majesta One JWTs |
| `PLATFORM_PUBLIC_URL` | `http://localhost:8080` | JWT issuer + Google/Apple callback base |
| `AUTO_SEED` | `1` | Seeds managed `core` on API boot |
| `SEED_CONTROL_IDE` | `1` | Seeds managed `one.controlIde` PKCE app when `AUTO_SEED=1`; set `0` to skip |
| `PORT` | `8080` | IDE Connect default |
| `HOST` | `0.0.0.0` | Dual-stack bind (`:8080`); `http://localhost` works from Control IDE |

For local Google-style sign-in you do **not** need Google Cloud credentials. In development the API enables a built-in `dev` login provider when `AUTH_LOGIN_PROVIDERS` is unset. Control IDE **Sign in** opens `/auth/v1/login`; choose **Continue with Google** (local stand-in) and the IDE receives a Majesta One JWT via `one-control://oauth/callback`.

To use real Google later, set `AUTH_LOGIN_PROVIDERS=google` plus `AUTH_GOOGLE_CLIENT_ID` / `AUTH_GOOGLE_CLIENT_SECRET`, and register redirect `http://localhost:8080/auth/v1/callback/google`.

## 2. Postgres

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
make migrate
```

## 3. API

```bash
make api
```

API logs should include `kernel migrations applied`, then **`one-api listening`** (`addr` is `:8080` for dual-stack localhost), then `bootstrap/seed starting` / `bootstrap/seed complete`. `/healthz` answers as soon as the listen line appears; `/readyz` stays `starting` until seed finishes. If you only see `one-api listening` with no migrate/seed lines, `DATABASE_URL` was empty.

In another terminal:

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -s -H "Authorization: Bearer dev-admin-key" http://localhost:8080/client/v1/me
```

## 4. Mint a Majesta One JWT (for Control IDE)

Control IDE stores a **Majesta One JWT**, not a raw API key. With the API running and `AUTH_JWT_SIGNING_KEY` set:

```bash
curl -s -X POST http://localhost:8080/auth/v1/token \
  -H 'Content-Type: application/json' \
  -d '{"grant_type":"client_credentials","client_secret":"dev-admin-key"}'
```

Copy `access_token` from the JSON response. You can also mint from the IDE Connect panel (client id + secret / form-urlencoded).

## 5. Control IDE

```bash
cd tools/control-ide
npm ci
npm test
npm run electron:dev
```

In Settings → Environments:

1. Base URL: `http://localhost:8080`
2. Paste the JWT from step 4 (or use client credentials / PKCE)
3. Save & verify `/client/v1/me`
4. Smoke Environments (`GET /deploy/v1/environment`)

### CLI-less Build → Deploy smoke

1. Build → Repo → **Initialize from sample** (or clone `customerRepoUrl`) into a local path.
2. Optionally tweak Automations / Objects; Repo → **Commit**.
3. Build → Deploy → **Pack from local repo (HEAD)** → **Validate vs org** → run suite `CreateAccountFromContact`.
4. Connect Settings → Environments to a second install and deploy the same Git SHA (optional). Do not peer-promote.

Notes:

- `npm run dev` opens the Vite UI in a browser — fine for layout iteration; filesystem/git IPC needs Electron (`electron:dev`).
- Session tokens are stored via OS `safeStorage` in Electron userData. Two Control IDE processes that share `userData` share the JWT.
- Sample template lives at `deploy/customer-repo-template` (not product seed).

### Two Control IDE processes (isolated sessions)

Chromium’s `--user-data-dir` switch must come **before** the app path. `npm run electron:dev -- --user-data-dir=…` appends the flag after `.` and may not isolate sessions.

```bash
cd tools/control-ide
npm run build
npx electron --user-data-dir="${ONE_IDE_A_DATA:-$HOME/.local/share/one-control-ide-a}" .
# second terminal
npx electron --user-data-dir="${ONE_IDE_B_DATA:-$HOME/.local/share/one-control-ide-b}" .
```

Connect each window independently. Multi-env + dual-IDE campaign: [customer-rollout-test-run.md](./customer-rollout-test-run.md).

### Optional Mac installer (unsigned)

```bash
cd tools/control-ide
npm run dist:mac
```

Produces dmg/zip under `tools/control-ide/dist/`. Code signing and notarization are a later milestone ([control-ide-build.md](./control-ide-build.md)).

## 6. Local inference (Ollama)

Spin agent runs against a model on your Mac without DigitalOcean Inference.

1. Install and pull a model:

```bash
brew install ollama
ollama serve   # if not already running as a service
ollama pull llama3.2
```

2. Ensure the API/worker shell has **non-production** env (default from `.env.example`):

```bash
# .env
APP_ENV=development
```

Restart `make api` after changing `APP_ENV`. Production (`APP_ENV=production`) still rejects loopback BYO URLs. `make worker` is only required for **non-stream** agent runs (JSON create/approve that enqueue `agent.run`).

3. In Control IDE: **Settings → Inference → Preset: Ollama (local)**.

   - Base URL: `http://127.0.0.1:11434/v1` (use `http://host.docker.internal:11434/v1` if the API runs in Docker Desktop and Ollama is on the Mac host)
   - Model: e.g. `llama3.2` (must match `ollama list`)
   - API key: any placeholder such as `ollama` (required by Majesta One; ignored by Ollama)
   - Save BYO provider (sets active source to BYO)

4. In Control IDE: **Settings → Inference → Test chat** — send the default prompt. Tokens should appear in the transcript on that page (no Approve). Then enter **Operate**, open RunCoach (or any agent), send a short prompt — tokens should stream **without an Approve click**. Approval still gates applying mutations (Change bar / future tool loop), not the LLM reply.

If Operate still shows **Approve run** / `awaits approval before tools execute`, the connected API parked a streaming create. Use Test chat first: if it streams, restart `make api` on this branch and retry Operate. If Test chat also errors, the model/provider is not reachable (`INFERENCE_NOT_CONFIGURED`, Ollama not running, or loopback URL blocked).

Loopback BYO hosts skip the install egress allowlist in development only. Cloud OpenAI-compatible hosts still require Govern → Integrations egress allowlist entries.

See [inference-build-plan.md](./architecture/inference-build-plan.md) / [BP-052](../backlog/BP-052-customer-inference.md).

## 7. Tests

From the repo root:

```bash
# Go (skips DB suites if DATABASE_URL unset)
export DATABASE_URL=postgres://one:one@localhost:5432/one
make test

# Control IDE unit + component
make test-ide

# Live-API contracts (API must be up)
export ONE_API_URL=http://localhost:8080
export ONE_API_KEY=dev-admin-key
# or: export ONE_JWT='<access_token from step 4>'
make test-ide-integration
```

Product CI equivalent: `make ci` (Go only — does not run IDE tests). IDE changes are covered by path-filtered GitHub Actions ([control-ide-build.md](./control-ide-build.md)).

## Agent scope reminder

| Work | Stay in | Domain agent |
|---|---|---|
| Electron / React / Vitest | `tools/control-ide/**` | `control-ide` |
| API / AuthZ / Deploy / data | `cmd/`, `internal/`, … | Go domain agents |

See [AGENTS.md](../AGENTS.md) and [architecture/agent-routing.md](./architecture/agent-routing.md).

## Troubleshooting

| Symptom | Check |
|---|---|
| `source: no such file or directory: .env` | You are not in repo root, or never ran `cp .env.example .env` |
| Only `kernel migrations applied`, then IDE auth fails | Wait for `one-api listening` — `/healthz` should work immediately after that line. `/auth/v1` returns `STARTING` until `bootstrap/seed complete` |
| Connection refused to `http://localhost:8080` | API not bound yet, or an old process used IPv4-only `0.0.0.0:8080`. Restart on this branch (`addr` should be `:8080`). Try `curl -s http://127.0.0.1:8080/healthz` |
| `DB_UNAVAILABLE` / social login | API started without `DATABASE_URL` — confirm `.env` exists in the repo root; restart `make api`; logs should show migrations/seed |
| `DATABASE_URL` connection refused | Docker Desktop running; Compose Postgres up; port 5432 free |
| `/auth/v1/token` 503 / disabled | `AUTH_JWT_SIGNING_KEY` non-empty; restart `make api` |
| Google `PROVIDER_DISABLED` / unavailable | `AUTH_LOGIN_PROVIDERS=google` + client id/secret in the same shell as `make api` |
| IDE Connect 401 | Token expired or wrong base URL; remint JWT |
| IDE Connect works but Environments 403 | JWT scopes lack `deploy`; use admin bootstrap key to mint |
| `npm run electron:dev` fails | Run `npm ci` in `tools/control-ide`; Node 20+ |
| Go not found after brew | `brew link go@1.25` or add Homebrew Go to `PATH` |
| Ollama BYO rejected / `baseUrl must be https` | `APP_ENV=development` on API; restart after change; URL host must be `127.0.0.1`, `localhost`, or `host.docker.internal` |
| Agent run fails connecting to Ollama | `ollama serve` running; model pulled; if API is in Docker use `host.docker.internal` |
| Chat asks to Approve then shows `Agent run queued` | Streaming create should skip pre-LLM approval; restart API on this branch. **Settings → Inference → Test chat** validates the same SSE path. Non-stream clients still park; Approve with SSE does not need the worker |
