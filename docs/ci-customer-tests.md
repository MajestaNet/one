# CI gate: customer tests before promote (Phase D)

Example flow for a customer **Test** install. Gate promotion on a green test run.

## Prerequisites

- Majesta One API reachable (e.g. `https://one-test.example.com`)
- Deploy-scoped API key: `API_KEYS=ci-deploy:deploy` (or `client+metadata+deploy` for a broader CI key)

## Steps

```bash
export ONE_URL=https://one-test.example.com
export ONE_DEPLOY_KEY=ci-deploy

# 1) Ensure customer test suite exists (idempotent upsert)
curl -sS -X POST "$ONE_URL/deploy/v1/tests" \
  -H "authorization: Bearer $ONE_DEPLOY_KEY" \
  -H "content-type: application/json" \
  -d '{
    "apiName": "SmokeMetadata",
    "label": "Smoke metadata",
    "steps": [
      { "type": "objectExists", "objectApiName": "MyCustom__c" },
      { "type": "fieldExists", "objectApiName": "MyCustom__c", "fieldApiName": "Name" }
    ]
  }'

# 2) Run suite synchronously (default) — fails CI if status != passed
RUN=$(curl -sS -X POST "$ONE_URL/deploy/v1/tests/runs" \
  -H "authorization: Bearer $ONE_DEPLOY_KEY" \
  -H "content-type: application/json" \
  -d '{"suiteApiName":"SmokeMetadata"}')

echo "$RUN" | jq .
STATUS=$(echo "$RUN" | jq -r .run.status)
test "$STATUS" = "passed"

# 3) Create bundle from current customer snapshot
BUNDLE=$(curl -sS -X POST "$ONE_URL/deploy/v1/bundles" \
  -H "authorization: Bearer $ONE_DEPLOY_KEY" \
  -H "content-type: application/json" \
  -d '{"label":"ci-'"$(date -u +%Y%m%dT%H%M%SZ)"'"}')

BUNDLE_ID=$(echo "$BUNDLE" | jq -r .id)

# 4) Validate on Test (same install)
curl -sS -X POST "$ONE_URL/deploy/v1/bundles/$BUNDLE_ID/validate" \
  -H "authorization: Bearer $ONE_DEPLOY_KEY" | jq .

# 5) Cross-env: switch to Prod URL and `one org deploy` from the same Git SHA (repo→org)
```

## Async runs

Pass `"async": true` on `POST /deploy/v1/tests/runs` to enqueue a `customer.test.run` worker job, then poll `GET /deploy/v1/tests/runs/:id` until `passed` or `failed`.

## Product upgrade gate

After an ECS product image roll (SSM Automation or `/ops/v1/upgrades`), the orchestrator runs:

1. Managed suite **`PlatformSmoke`** (seeded on boot; do not overwrite)
2. Optional customer suite **`PostUpgradeSmoke`** when present

Register a customer suite for custom post-upgrade checks:

```bash
curl -sS -X POST "$ONE_URL/deploy/v1/tests" \
  -H "authorization: Bearer $ONE_DEPLOY_KEY" \
  -H "content-type: application/json" \
  -d '{
    "apiName": "PostUpgradeSmoke",
    "label": "Post-upgrade smoke",
    "steps": [
      { "type": "objectExists", "objectApiName": "MyCustom__c" }
    ]
  }'
```

See [product-upgrades.md](./product-upgrades.md).

## Step types

| Type | Purpose |
|---|---|
| `objectExists` | Metadata object present |
| `fieldExists` | Metadata field present |
| `createRecord` | Client create via DataEngine (`storeAs` saves Id) |
| `assertValidation` | Expect create to fail (or succeed) |
| `query` | SQL query with `filters` + `expectMinRows` |
| `automationUnitPass` | Deno unit harness for `tests/automations/**` ([automation-sdk.md](./automation-sdk.md)) |
| `automationContract` | Fixture create → invoke automation → assert child rows |

## Promote gate (ADR-014 Phase 5)

Set `DEPLOY_REQUIRED_TEST_SUITES=CreateAccountFromContact` (comma-separated) on the install. Non-dry-run `PromoteBundle` runs those suites synchronously and fails the promotion when any suite is not `passed`.
