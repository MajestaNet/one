# Campaign finding (SI rollout)

Use this for a **confirmed** gap from [docs/customer-rollout-test-run.md](../docs/customer-rollout-test-run.md) (beats `G-…`) or [docs/customer-install-simulation-test-run.md](../docs/customer-install-simulation-test-run.md) (beats `S-…`). Do **not** open a new `backlog/BP-*.md` item. Do **not** use this template for security vulnerabilities ([SECURITY.md](../SECURITY.md)).

## Campaign

- Run date:
- Beat id (`G-…` or `S-…`):
- Scenario card (`A`–`F` or `S-A`–`S-E`):
- Outcome: `fail` / `pass-with-workaround`
- DX (1–5):
- Class: `product-bug` / `docs-drift` / `missing-lab-packaging` / `authz-confusion`
- Operator doc you used first:
- Gap-log row: [docs/customer-rollout-gap-log.md](../docs/customer-rollout-gap-log.md)

## What happened

(Expected vs actual. No secrets, customer data, or exploit detail.)

## Fix-it (for the implementing agent)

Playbook (one):
Domain agent:
Packages (stay in):
Out of scope:

1.
2.

Verify:

- [ ] `make test` and/or `make test-ide` as the playbook says
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

Architecture remainder only if this is a symptom of a BP, not a substitute for this issue: `BP-…`
