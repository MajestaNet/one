# BP-048 — one CLI productization

**Severity:** Medium  
**Status:** Partially mitigated (keychain + builder templates + cross-platform `one` assets shipped; scratch orgs deferred)  
**Track:** Finish (Ship path of record). IDE Ship/Repo parity is **frozen** ([ADR-030](../docs/adr/030-install-agent-runtime.md)).  
**Area:** `cmd/one`; `internal/customerrepo`; product `v*` release assets; docs (`one-repo`, customer DX, [builder-connect.md](../docs/builder-connect.md))  
**Remainder (Finish, not a re-plan of Phases 1–4):** [agentic-remainders/02-bp-048-one-cli.md](../docs/architecture/agentic-remainders/02-bp-048-one-cli.md)

## Problem

BP-032 shipped org validate/deploy/retrieve, but `one` remained a thin CI helper:

1. No auth login / org aliases / default target-org config
2. No `project init` (template only via IDE sample or manual copy)
3. Whole-tree pack only — no selective path / manifest deploy
4. CLI not published on product `v*` releases (CI samples need ad-hoc URL)
5. Repo folders `environments/` and `changes/` documented but unused by tools
6. Doc drift (tooling tables omit `org *`; template README still peer-promote language)

Relative to incumbent CRM DX CLIs, the gap is **productized developer tooling**, not Deploy apply capability.

## Direction

Follow [one-cli-build-plan.md](../docs/architecture/one-cli-build-plan.md):

- Go CLI only (ADR-005)
- `~/.config/one` config + `0600` credentials
- Path allowlist / `manifests/*.yaml` selective pack (not `package.xml`)
- Publish binary on product releases
- Wire `environments/` + operational `changes/` workflow
- IDE Ship/Repo parity for stage order, selective pack, New change

**Production DX safety (not owned here):** [BP-033](./BP-033-customer-runtime-isolation.md) (validate/deploy must not starve Client) and [BP-025](./BP-025-ide-api-version-compatibility.md) (IDE/CLI ↔ API version handshake). Treat those as prerequisites for safe tight DX loops.

## Mitigation

| Slice | Status |
|---|---|
| Build plan + this BP | Done |
| Auth / aliases / org list\|use + default org for `org *` | Done |
| `project init` (scaffold + optional from-org) | Done |
| Selective pack (`IncludePaths` / manifests) + CLI flags | Done |
| `change create` + environments load helpers | Done |
| Publish `one` on `v*` releases + CI sample update | Done |
| IDE: env YAML stage order, selective Ship, New change, copy drift | Done |
| `org retrieve --baseline-only` | Done |
| OS keychain credential backend | Done — OS keychain with file `0600` fallback (`ONE_CREDENTIAL_STORE`) |
| Scratch-like ephemeral installs | Deferred (Wave D) |
| Cross-platform `one` release assets (linux/amd64+arm64, darwin/amd64+arm64, windows/amd64; unadorned `one` = linux/amd64) | Done (remainder Phase R1) |
| Datapack `OrgClient` `One-API-Revision` pin | Done (`internal/datapack/apply.go`; `TestOrgClientDoJSONSetsAPIRevisionHeader`) |
| auth `client_credentials` mint; `environments/` alias hints; org httptest | Open — [remainder plan](../docs/architecture/agentic-remainders/02-bp-048-one-cli.md) Phase R2 |
| Async Deploy waiter (`202` / `DEPLOY_BUSY`) | Open — [BP-033](./BP-033-customer-runtime-isolation.md) Phase 1 HTTP is in tree; CLI waiter is not |

## Related

- [Remainder tech design + agentic build plan](../docs/architecture/agentic-remainders/02-bp-048-one-cli.md)
- [one-cli-build-plan.md](../docs/architecture/one-cli-build-plan.md)
- [customer-dx-build-plan.md](../docs/architecture/customer-dx-build-plan.md) · [BP-032](./BP-032-customer-dx-validate-deploy.md)
- [BP-023](./BP-048-one-cli.md) · [BP-031](./BP-031-customer-repo-init-sync.md)
- [BP-033](./BP-033-customer-runtime-isolation.md) · [BP-025](./BP-025-ide-api-version-compatibility.md)
- [ADR-012](../docs/adr/012-customer-repo-and-control-ide.md)
