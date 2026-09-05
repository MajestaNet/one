## Campaign 2 retest (S-A-MIGRATE-RACE) — 2026-09-05

Native three-DB fallback (this VM has no Docker). Fresh empty `one_sim_prod`. Started API + worker **together** (prebuilt binaries so compile time would not serialize the race).

**Still fails.** Worker exited immediately:

```
migrate failed err="ERROR: duplicate key value violates unique constraint \"pg_type_typname_nsp_index\" (SQLSTATE 23505)"
```

API continued, kernel migrate + seed completed, `/readyz` became `{"status":"ready"}`. Sequential workaround (start API, wait `/readyz`, then worker) succeeded on prod retry and on test + dev first boot.

Error shape differs from campaign 1 (`0038_hard_delete_no_default`); concurrent `EnsureKernel` on first boot is still unsafe. Do not treat this as a new issue.

Workaround still required for native labs. Compose overlay was not run here (`docker` not installed).
