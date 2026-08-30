# Capability catalog (system permissions)

Canonical flags on `permission_sets.system_permissions` (multi-cap per set; OR-union across assignments):

| Capability | Gates |
|---|---|
| `identity.users` | User principals, user credentials, freeze (BP-017) |
| `identity.integrations` | Connected Apps, service/agent credentials, secret rotate/reveal |
| `authz.manage` | Define permission sets (Metadata) **and** assign Roles/PS (Client) |
| `metadata.build` | Customer customize + package enable/disable |
| `deploy.promote` | Deploy mutates |
| `govern.network` | Install exposure / WAF / `clientAccessMode` |
| `govern.agents` | Agent run approve |
| `govern.audit` | Future audit export |
| `debug.read` | Read `ExecutionRun` / `ExecutionLogEntry` customer debug objects (BP-033) |
| `debug.trace` | Create/stop user `TraceFlag`s (BP-034 / Monitor); pairs with `debug.read` |
| `ide.operate` | Control IDE Operate mode (chrome; requires Role `client` for API) |
| `ide.run` | Control IDE Run mode (chrome; requires Role `client` for API) — [ADR-021](../adr/021-run-mode-toolspec.md) / [BP-050](../../backlog/BP-050-run-mode-toolspec.md) |
| `ide.build` | Control IDE Build mode (chrome; requires Role `metadata` for API) |
| `ide.ship` | Control IDE Ship mode (chrome; requires Role `deploy` for API) |
| `ide.govern` | Control IDE Govern mode (chrome) |
| `ide.settings` | Control IDE Account section (sixth launcher tile) — [BP-051](../adr/030-install-agent-runtime.md) |
| `ide.operate.query` | Operate → Query tool |
| `ide.operate.monitor` | Operate → Monitor tool (also needs `debug.read` or `debug.trace` for useful data) |
| `ide.operate.explorer` | Operate → Explorer tool |
| `ide.operate.canvases` | **Obsolete chrome** — do not wire Operate canvas; use `ide.run.tools` ([ADR-021](../adr/021-run-mode-toolspec.md)) |
| `ide.run.tools` | Run → see/open ToolSpecs on the dynamic Tool rail |
| `ide.build.objects` | Build → Objects |
| `ide.build.packages` | Build → Packages |
| `ide.build.agentSpecs` | Build → Agents |
| `ide.build.canvasSpecs` | **Obsolete chrome alias** — prefer `ide.build.tools` |
| `ide.build.tools` | Build → ToolSpec templates (Run Tools authoring) |
| `ide.build.repo` | Build → Repo |
| `ide.ship.deploy` | Ship → Deploy Pipeline |
| `ide.ship.env` | Ship → Environments |
| `ide.settings.account` | Account → Account |
| `ide.settings.hosting` | Account → Hosting (Deploy cloud admin; relocated from Ship/Govern Environments) |
| `ide.settings.inference` | Settings → Inference (BYO providers + Native DigitalOcean Inference; BP-052) |
| `ide.settings.env` | Settings → Environments (Connect); legacy `ide.ship.env` / `ide.govern.env` alias here |
| `ide.govern.users` | Govern → Users |
| `ide.govern.integrations` | Govern → Integrations |
| `ide.govern.experiences` | Govern → Experiences |
| `ide.govern.installAuth` | Govern → Install auth |
| `ide.govern.permissions` | Govern → Permissions |
| `ide.govern.env` | **Legacy alias** — satisfies Settings → Environments (`ide.settings.env`); no longer a Govern rail tool |

Legacy aliases still accepted on write and expand on check: `identity.manage`, `metadata.customize`, `metadata.packages`, `metadata.assignAuthz`, `metadata.network`, `agents.approve`.

**IDE chrome:** mode + tool caps are both required after `/me` loads **in Control IDE**. Existing API caps (`identity.*`, `authz.manage`, `metadata.build`, …) remain **server mutate** gates. Go HTTP does not `requireCapability` on `ide.*` — see [ide-backend-coupling-review.md](./ide-backend-coupling-review.md).

Seeded packs: `Admin` (all caps), `ManageUsers`, `ManageIntegrations`, `ManagePermissions`, `Build`, `Deploy`, `Govern`, `Operate` (+ legacy pack names remapped). Product packs include matching `ide.*` grants.

Live update: `PATCH /metadata/v1/permissions/sets/{apiName}` supports `systemPermissions` (replace), `systemPermissionsAdd`, `systemPermissionsRemove`. Changes apply immediately to assignees (DB reload).
