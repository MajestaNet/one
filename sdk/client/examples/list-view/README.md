# List view sample — Client Experience

Minimal React + Vite app that queries Majesta One Client API after PKCE login.

## Prerequisites

1. Local Majesta One install (`docker compose -f deploy/docker-compose.yml up`)
2. Public Connected App registered:
   - `clientKind`: `public`
   - `oauthFlows`: [`authorization_code`]
   - `callbackUrls`: [`http://127.0.0.1:5174/oauth/callback`]
   - Scopes default to `client` only

## Run

```bash
cd sdk/client/examples/list-view
npm install
npm run dev
```

Open `http://127.0.0.1:5174`, sign in, and view Account records (requires `core` seed data).

## Environment

Copy `.env.example` to `.env` and set:

- `VITE_ONE_BASE_URL` — e.g. `http://127.0.0.1:8080`
- `VITE_ONE_CLIENT_ID` — Connected App **`apiName`** (not a break-glass `API_KEYS` entry)

After PKCE, the sample calls `client.query({ object: "Account", select: ["Name"], limit: 25 })` (`POST /client/v1/query`). Access JWT is stored in `sessionStorage` for the sample only; do not put refresh tokens in `localStorage`.

## Deploy

Build static assets (`npm run build`) and host on customer infra (CDN, App Platform static site, etc.). Promote Experience metadata via Deploy (`metadata/experiences/*.yaml`).

See [docs/client-experience-security.md](../../../../docs/client-experience-security.md).
