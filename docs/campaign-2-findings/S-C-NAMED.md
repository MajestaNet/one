# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-05
- Beat id (`G-…` or `S-…`): S-C-NAMED
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-C
- Outcome: `pass-with-workaround`
- DX (1–5): 3
- Class: `docs-drift`
- Operator doc you used first: docs/automation-sdk.md, tools/automation-sdk/one_automation.d.ts, docs/adr/029-platform-actions.md
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#38](https://github.com/MajestaNet/one/issues/38)

## What happened

Expected: nine named guest TS automations from `scripts/customer-install-sim-generate.sh` are real and run on the host (`one:automation` only).

Actual: all nine deployed to acme-dev. Live:

- `CreateSiteVisit_From_Opportunity` — Opportunity create (with AccountId) → SiteVisit__c in ~2s. Pass.
- `Fanout_TimeEntries` — Project create (`Status=Active`) → 3 TimeEntry rows. Pass.
- `ConvertLead_When_Qualified` — PATCH Lead `Status=Qualified` → Status=Converted, Account+Contact+Opportunity (`createOpportunity: true`). Pass.
- `Reject_Missing_Opportunity` — SiteVisit__c create without OpportunityId → 400 `Missing required field: OpportunityId` (required field fires before the sync automation). Pass for the negative; sync `{ ok: false }` not independently reached.
- `StampAccount_LastVisit` — **failed** job: `updateRecord requires objectApiName and recordId` (twice: North Plant visit + convert-created Opportunity visit).

Cause: generator writes `ctx.updateRecord({ objectApiName, id, data })` in `stamp_account_last_visit.ts` and `close_visit_when_opp_closed.ts`. Frozen SDK stubs and ADR-029 use **`recordId`**, not `id`. Host does not alias `id`. Unit tests never call `updateRecord`, so the suite did not catch it.

Workaround for the SI: pass `recordId` (do not add npm imports). Did not patch the sandbox so S-D keeps generator output.

## Fix-it (for the implementing agent)

Playbook (one): docs/automation-sdk.md
Domain agent: docs / campaign fixtures (`scripts/customer-install-sim-generate.sh`)
Packages (stay in): generator + optional one-line in `docs/automation-sdk.md` showing `updateRecord({ recordId })`. Host already errors clearly.
Out of scope: `internal/automation` alias for `id` unless product wants a documented alias; Control IDE; new BP

1. Change generated `updateRecord` calls to `recordId`.
2. Optionally add a unit test that asserts `getCalls({ method: "updateRecord" })[0].recordId`.
3. Re-deploy to a lab install; Opportunity+Account SiteVisit must stamp `Account.LastSiteVisitId__c` without a failed `automation.run`.

Verify:

- [ ] `make test` not required unless a product unit test is added
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

`docs/automation-sdk.md` create example returns `{ id }` from `createRecord`; guests may copy that key onto `updateRecord`. Stubs already name `recordId`.
