## Campaign 2 retest (S-C-CLI-TRUNC) — 2026-09-05

Native three-install lab, `one org deploy -dir .customer-sandbox/one-acme-sim --alias dev --suite SiteVisitFromOpportunity`.

**Still truncates.** After a successful apply (57 automations created), the CLI printed a single suite line of **233 characters** ending mid-key:

```
suite SiteVisitFromOpportunity: {"run":{"id":"554072e2-9218-40b8-95f6-b1f2e402010d","suiteApiName":"SiteVisitFromOpportunity","status":"failed","trigger":"api","results":{"steps":[{"type":"objectExists","index":0,"detail":{"objectAp…
```

Prefix `suite SiteVisitFromOpportunity: ` is 34 chars; the JSON body is cut at **200** characters (`objectAp…`). Same `truncate(..., 200)` shape as campaign 1 / G-CLI-SUITE-TRUNC.

`status":"failed"` was visible. Failed step `type`, `message`, and remaining step names were **not**. Diagnosing required `GET /deploy/v1/tests/runs/554072e2-9218-40b8-95f6-b1f2e402010d` (full 9-step report; contract step message: `Opportunity requires AccountId and/or ContactId`).

Additional honesty: process **exit 0** despite `status=failed` (apply succeeded; suite is informational in the CLI). Not filed separately.

Do not treat this as a new issue.
