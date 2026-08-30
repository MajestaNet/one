# DigitalOcean App Platform — live smoke checklist

Manual checklist against a **real** DigitalOcean team token. Not run in CI.

Prerequisites: Path A install running; `DIGITALOCEAN_API_TOKEN` set on that install; bootstrap key with `deploy` + `admin`.

```bash
BASE=https://<your-app>.ondigitalocean.app
AUTH="Authorization: Bearer <bootstrap+admin>"
```

1. **Status (no binding yet)** — prefer host-free routes (DO aliases also work)

```bash
curl -fsS -H "$AUTH" "$BASE/deploy/v1/cloud/status"
# expect configured=true, host=digitalocean, binding null or partial
# alias: .../cloud/digitalocean/status
```

2. **Bind** this install to its App + Managed DB ids (from DO console)

```bash
curl -fsS -X PUT -H "$AUTH" -H 'Content-Type: application/json' \
  "$BASE/deploy/v1/cloud/binding" \
  -d '{"appResourceId":"<APP_ID>","databaseResourceId":"<DB_ID>","region":"nyc"}'
# legacy body keys appId/databaseId also accepted
```

3. **App summary**

```bash
curl -fsS -H "$AUTH" "$BASE/deploy/v1/cloud/app"
```

4. **Scale** (api size/count — use a cheap size class or slug for smoke)

```bash
curl -fsS -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  "$BASE/deploy/v1/cloud/app/scale" \
  -d '{"apiInstanceCount":1,"apiSizeClass":"small","workerInstanceCount":1,"workerSizeClass":"small"}'
```

5. **Resize database** (smallest plan; expect brief DO-side wait)

```bash
curl -fsS -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  "$BASE/deploy/v1/cloud/database/resize" \
  -d '{"sizeClass":"small","numNodes":1}'
```

6. **Provision peer environment** (creates **another** App Platform app + Managed Postgres + peer row)

```bash
curl -fsS -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  "$BASE/deploy/v1/cloud/environments" \
  -d '{
    "installId":"smoke-dev-1",
    "installRole":"dev",
    "region":"nyc",
    "apiKeys":"smoke-admin+admin",
    "authJwtSigningKey":"smoke-jwt-signing-key-change-me-32b",
    "apiSizeClass":"small",
    "workerSizeClass":"small",
    "databaseSizeClass":"small",
    "databaseNodes":1
  }'
```

7. **List** peers / provision runs

```bash
curl -fsS -H "$AUTH" "$BASE/deploy/v1/cloud/environments"
curl -fsS -H "$AUTH" "$BASE/deploy/v1/environment"  # cloudHost=digitalocean, capabilities.cloud=true
```

8. **Cleanup** — destroy the smoke peer App + DB in the DO console (or `doctl`) to avoid ongoing charges.

See [BP-030](../../backlog/BP-030-deploy-api-digitalocean-apps.md) · [self-host Path A](../../docs/self-host.md).
