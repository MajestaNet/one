# Campaign finding (SI rollout)

## Campaign

- Run date: 2026-09-06
- Beat id (`G-…` or `S-…`): S-E-SINGLE-INSTANCE
- Scenario card (`A`–`F` or `S-A`–`S-E`): S-E
- Outcome: `fail`
- DX (1–5): 2
- Class: `product-bug`
- Operator doc you used first: docs/customer-install-simulation-test-run.md (S-E dual-IDE recipe), docs/customer-ide-ux.md, docs/local-development-mac.md
- Gap-log row: [docs/customer-rollout-gap-log.md](../customer-rollout-gap-log.md)
- GitHub issue: [#39](https://github.com/MajestaNet/one/issues/39)

## What happened

Expected: Control IDE launched with distinct `--user-data-dir` values can run concurrently (runbook: `one-control-ide-sim-a` and optional `…-sim-b`). Quitting A must not sign out B. Persona tests for sales / govern / restricted users need isolated sessions.

Actual: `tools/control-ide/src/main/main.ts` calls `app.requestSingleInstanceLock()` with no partition. If a second process starts while the first is alive, `gotLock` is false and the new process `app.quit()`s; `second-instance` focuses the first window. The second `--user-data-dir` is never used.

During the persona lab, launching Sam while Riley was still running reused Riley’s window (looked like “session cache”). Isolated tests only worked after fully killing the first process.

This is the dual-IDE recipe in the S-E runbook, not a request for new chrome.

## Fix-it (for the implementing agent)

Playbook (one): docs/architecture/agent-control-ide.md
Domain agent: `control-ide`
Packages (stay in): `tools/control-ide/src/main/` (and tests under `tools/control-ide`)
Out of scope: product Go; unfreezing chrome; Identity/AuthZ

1. Scope the single-instance lock by `userData` path (or skip the lock when `--user-data-dir` is an explicit isolated dir).
2. Two processes with different `--user-data-dir` must both stay open with independent `session.bin`.
3. Keep a single-instance lock for the default userData so `one-control://` OAuth callbacks still focus the existing window.

Verify:

- [ ] `make test-ide` (unit covering lock / second-instance behavior if you add a test)
- [ ] Manual: two `npx electron --user-data-dir=…-sim-a|b` processes both remain; each keeps its JWT
- [ ] PR description includes `Fixes #<this-issue>`
- [ ] Gap-log **Issue registry** marks this beat `closed` and links the PR

## Related (optional)

Campaign 1 `G-IDE-USERDATA` (recipe unproven). Architecture remainder is not a substitute: do not open a new BP.
