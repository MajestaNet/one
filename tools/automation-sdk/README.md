# automation-sdk (vendor plane)

Type stubs for Majesta One guest automations. Copy or path-map `one_automation.d.ts` in Control IDE / editor config.

Customers do **not** install this from npm. At runtime the platform injects `one:automation` via Deno import map ([ADR-014](../../docs/adr/014-customer-code-automations.md)).

See [docs/automation-sdk.md](../../docs/automation-sdk.md). Platform verbs use `invokeAction` ([ADR-029](../../docs/adr/029-platform-actions.md)); do not add per-verb helpers here.
