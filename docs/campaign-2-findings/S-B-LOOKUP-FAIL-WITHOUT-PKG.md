# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-05
- Beat id (`G-…` or `S-…`): S-B-LOOKUP-FAIL-WITHOUT-PKG
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-B
- Outcome: `fail`
- DX (1–5): 2
- Class: `product-bug`
- Operator doc you used first: docs/modules/README.md, docs/customer-repo.md, docs/customer-install-simulation-test-run.md (S-B)
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#34](https://github.com/MajestaNet/one/issues/34)

## What happened

Expected (runbook S-B): `one org validate` / deploy of `SiteVisit__c` with lookup `OpportunityId` → Opportunity fails on an install that has not enabled `sales` (“lookup target missing”).

Actual, native three-install lab, **before** any optional pack enable on `acme-dev`:

1. `GET /client/v1/describe/Opportunity` and `GET /metadata/v1/objects/Opportunity` → 404 `not found: object not found: Opportunity` (same for Project / Lead). Good.
2. `POST /metadata/v1/fields` with `objectApiName=SiteVisit__c`, `fieldType=lookup`, `referenceTo=Opportunity` → **404** `not found: object not found: Opportunity`. Same for `Project`. This is the error we wanted.
3. `one org validate -dir .customer-sandbox/one-acme-sim --alias dev --metadata metadata/objects/SiteVisit__c.yaml --metadata metadata/fields/SiteVisit__c/` → **ok=true** (exit 0). The Opportunity lookup was treated as an additive field, not a missing target.
4. `one org deploy` of that same selective pack **applied** the field (`fields.SiteVisit__c.OpportunityId` created) while Opportunity still did not exist. Subsequent `GET /metadata/v1/objects/SiteVisit__c` shows `referenceTo: Opportunity`, `ownership: custom`.
5. Selective pack that also included custom fields **on** Opportunity/Project (`Opportunity.SiteVisitCount__c`, `Project.EngagementFlag__c`) did fail org validate with `MISSING_PARENT` / `Field Opportunity.SiteVisitCount__c references unknown object`. So parent-object existence is gated for fields **on** the missing object, but **not** for lookups **to** it.

Ship of record is CLI org validate/deploy. Metadata POST is the only path that rejected the dangling lookup. After packs were enabled, a second org deploy of objects+fields applied cleanly.

No secrets. Lab left running.

## Fix-it (for the implementing agent)

Playbook (one): docs/architecture/agent-api-families.md (Deploy validate) then docs/architecture/agent-data-architecture.md
Domain agent: `api-families` then `db-backend-perf`
Packages (stay in): Deploy bundle validation (lookup `referenceTo` must exist on the target install, same gate as Metadata field create); tests under `internal/testutil` / deploy validate tests
Out of scope: Control IDE; enabling packs via Deploy; new BP; customer YAML in `internal/seed`

1. Org validate (and therefore org deploy) must fail closed when a customer lookup `referenceTo` names an object that is not present on that install (disabled/not-enabled managed pack or unknown apiName). Match Metadata create: 404 / not found Opportunity.
2. Prefer a stable issue code (e.g. `LOOKUP_TARGET_MISSING` or reuse `MISSING_PARENT`) and an operator-readable message that names the field, the missing object, and that the managed package may not be enabled.
3. Add an integration test: custom object + lookup to Opportunity on an install without `sales` → validate not ok; enable `sales` → validate ok.
4. Do not treat CLI `--metadata` selective packs as exempt.

Verify:

- [ ] `make test` as the playbook says
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

Runbook claim: docs/customer-install-simulation-test-run.md S-B negatives item 1. Metadata enable API already correct per docs/modules/README.md.
