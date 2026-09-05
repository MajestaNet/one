# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-05
- Beat id (`G-…` or `S-…`): S-C-SUITE
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-C
- Outcome: `fail`
- DX (1–5): 2
- Class: `docs-drift`
- Operator doc you used first: docs/automation-sdk.md, docs/customer-install-simulation-test-run.md (S-C), docs/customer-developer-workflow.md
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#37](https://github.com/MajestaNet/one/issues/37)

## What happened

Expected: `one org deploy --alias dev --suite SiteVisitFromOpportunity` runs the generated suite green after applying named automations (runbook Band 1 live path uses the same Opportunity create body).

Actual (acme-dev, packs enabled, objects already on the install):

- Apply succeeded (`created: 60`, 57 automations). CLI **exit 0**.
- Suite `status=failed` (run `554072e2-9218-40b8-95f6-b1f2e402010d`). CLI truncated the JSON (#29); full report via `GET /deploy/v1/tests/runs/:id`.
- Steps 0–7 **passed** (5× `objectExists`, 3× `automationUnitPass` including Deno unit harness).
- Step 8 `automationContract` **failed** in 5ms: `contract fixture create: Opportunity requires AccountId and/or ContactId`.
- Same 400 from the runbook live curl (`Name`/`StageName`/`CloseDate` only). Workaround: create Account, then Opportunity with `AccountId` → 201; worker created `SiteVisit__c` (“North Plant kickoff visit”) in ~2s.

Root copies: generated `tests/SiteVisitFromOpportunity.yaml` `data:` omits `AccountId`; `scripts/customer-install-sim-generate.sh` writes that fixture; runbook S-C curl matches it. Product sales validation requires AccountId and/or ContactId (S-B already saw this on AuthZ CRUD). Operator docs and the campaign generator do not.

S-D same-SHA `--suite` will fail the same contract unless the fixture (or docs) is fixed. Do not open a new BP.

## Fix-it (for the implementing agent)

Playbook (one): docs/architecture/customer-install-simulation-playbook.md (fixtures) / docs/automation-sdk.md
Domain agent: docs / `deploy-ops` only if the contract runner should seed required lookups
Packages (stay in): `scripts/customer-install-sim-generate.sh`, `docs/customer-install-simulation-test-run.md` (Opportunity create example). Optional: customer test fixture docs in `docs/customer-repo.md`.
Out of scope: Control IDE; changing sales-pack Opportunity validation unless product intended optional AccountId (then this is a product-bug instead — do not weaken validation just to green the lab curl)

1. Give the generated `automationContract` (and the S-C runbook curl) a legal Opportunity create: create/link Account (or Contact) so `AccountId` and/or `ContactId` is set.
2. Keep unit-test fixtures as-is if they only mock `ctx` (they already pass).
3. Re-run `one org deploy --suite SiteVisitFromOpportunity` on a lab install and confirm suite `status=passed` without a silent CLI exit-0.

Verify:

- [ ] Generator + runbook Opportunity create matches live sales validation
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

CLI suite JSON truncation remains [#29](https://github.com/MajestaNet/one/issues/29) (retested this card). Opportunity AccountId requirement also noted on S-B-AUTHZ-STUBS (not filed there).
