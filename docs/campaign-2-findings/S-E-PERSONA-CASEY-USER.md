# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-06
- Beat id (`G-…` or `S-…`): S-E-PERSONA-CASEY-USER
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-E
- Outcome: `fail`
- DX (1–5): 2
- Class: `product-bug`
- Operator doc you used first: docs/customer-ide-ux.md (Operate graph + AuthZ), docs/customer-connect.md, docs/modules/README.md (deny stubs)
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#40](https://github.com/MajestaNet/one/issues/40)

## What happened

Expected: a non-admin Operate user without `identity.users` (Casey: StandardUser + Operate PS deny stubs; Sam: SalesRep + Operate + SalesData) sees business objects they can read. Kernel **User** is identity, not a CRM object. List View must not offer Create User / list Users when Client would 403.

Actual (acme-dev, Control IDE Operate):

- Casey’s graph: “Graph ready with **1 accessible object**” — that object is **Users**. Opportunity is correctly absent.
- Opening List View Users calls `GET /client/v1/principals?principalType=user` and surfaces a raw 403: `CAPABILITY_REQUIRED` / `capability identity.users required`.
- The same panel still shows **Create User**, “No User records yet… Create one, or ask an administrator to share existing records” — a lying empty-state after a capability miss.
- Sam (sales) also gets Users on the graph (“6 accessible objects” including Users) even though sales work is Account/Opportunity.

Honest 403 on a panel the chrome should not have opened is still a fail: the graph advertised Users as accessible.

Do not treat this as “please add Operate-as-CRM”. Hide or omit the User collection unless the JWT has `identity.users`.

## Fix-it (for the implementing agent)

Playbook (one): docs/architecture/agent-control-ide.md (+ agent-authz.md if describe lists User for everyone)
Domain agent: `control-ide` (then `authz-security` only if `/client/v1/describe` is the leak)
Packages (stay in): `tools/control-ide/src/renderer/run/` (graph home + List View). Go `internal/httpapi` describe only if User is incorrectly “accessible” without the cap.
Out of scope: new Govern chrome; Client Experience CRM (BP-040); filing a new BP

1. Do not mount a Users collection / List View on Operate unless `identity.users` (or equivalent) is in `/client/v1/me` `systemPermissions`.
2. On 403 `CAPABILITY_REQUIRED`, do not show Create / “no records yet — create one”. Show the capability miss without implying sharing.
3. Add a Vitest covering Operate List View User + 403 vs hidden collection.

Verify:

- [ ] `make test-ide`
- [ ] Casey JWT: graph has no Users node; List View has no User/Create User
- [ ] Jordan / Pat (identity.users): Users list still works in **Govern**, not as a CRM object unless intended
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

ADR-026 kernel User vs DataEngine query. List View already special-cases User via principals (`RunObjectHomePanel.test.tsx`). BP-066 honesty is adjacent; do not open a new BP.
