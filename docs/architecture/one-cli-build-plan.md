# one CLI — productization build plan

**Status:** Active (Wave A+)  
**Backlog:** [BP-048](../../backlog/BP-048-one-cli.md)  
**Depends on:** [customer-dx-build-plan.md](./customer-dx-build-plan.md) / [BP-032](../../backlog/BP-032-customer-dx-validate-deploy.md) (mitigated)  
**Playbooks:** [agent-deploy.md](./agent-deploy.md)  
**Workflow:** [customer-developer-workflow.md](../customer-developer-workflow.md) · Format: [customer-repo.md](../customer-repo.md)

## Goal

Productize `cmd/one` from a CI helper into an **sf-like customer DX CLI**: auth/aliases, project init, selective pack/deploy, config defaults, and release-distributed binaries — without changing Deploy apply SoR (install DB after org deploy).

```text
one project init → auth login → edit → org validate → org deploy
```

Control IDE Ship is a **frozen** GUI twin of the same Deploy APIs. The **Ship path of record** for builders is this CLI + MCP ([ADR-030](../adr/030-install-agent-runtime.md) · [agent-runtime-build-plan.md](./agent-runtime-build-plan.md)).

## Locked decisions

| Decision | Choice |
|---|---|
| Language | Go binary only (ADR-005) — no Node CLI |
| Config | `~/.config/one/config.json` (+ `credentials.json` mode `0600`) |
| Env overrides | `ONE_ORG` (alias), `ONE_TOKEN` / `ONE_API_KEY`, `ONE_BASE_URL` |
| Selective deploy | Path allowlist under `metadata/`, `src/`, `tests/` and/or `manifests/<name>.yaml` — **not** `package.xml` |
| Credentials | OS keychain when available (`zalando/go-keyring`); file store `0600` fallback. `ONE_CREDENTIAL_STORE=auto\|file\|keychain` |
| Distribution | Attach `one` to product `v*` GitHub Releases (with api/worker/migrate) |
| Multi-org | One connected org per command (no fan-out) |

## Command matrix

| Command | Exit 0 | Exit 1 | Exit 2 |
|---|---|---|---|
| `auth login` | Stored alias | Auth/write failure | Bad flags |
| `auth logout` | Removed | — | Bad flags |
| `org list` / `org use` | Printed / switched | Unknown alias | Bad flags |
| `project init` | Scaffold written | Exists / IO | Bad flags |
| `pack` / `unpack` / `validate` | OK | Pack/parse fail | Bad flags |
| `org validate` | Green validate | Validation errors / HTTP | Bad flags |
| `org deploy` | Applied | Validate fail / promote fail | Bad flags |
| `org retrieve` | Unpacked | Dirty tree / HTTP | Bad flags |
| `--version` / `version` | Printed | — | — |
| `change create` | Branch + CHANGE.yaml | Git/IO | Bad flags |

## Config schema

```json
{
  "defaultOrg": "test",
  "orgs": {
    "test": { "baseUrl": "https://one-test.example.com", "credentialRef": "test" }
  }
}
```

`credentials.json` maps `credentialRef` → `{ "token": "..." }` or `{ "apiKey": "..." }` (mode `0600`).

Repo `environments/<role>.yaml` may supply `baseUrl` / `installRole` for alias hints; secrets never live there.

## Phased delivery

### Phase 1 — Spec + auth/config + project init + release asset (this change)

1. This plan + BP-048.
2. Config/auth/org list|use; existing `org *` resolve default alias.
3. `project init` from embedded scaffold (+ optional `--from-org` retrieve).
4. Publish `one` on `v*` releases; update CI samples.
5. CLI tests; usage/`--dry-run`/doc drift fixed.

### Phase 2 — Selective pack + manifests + change create

1. `PackOptions.IncludePaths` + `manifests/*.yaml`.
2. `org validate|deploy --metadata <path>...` / `--manifest <name>`.
3. `change create <slug>` writes `changes/<slug>/CHANGE.yaml` + `change/<slug>` branch guidance.
4. Enrich `one.yaml` (`defaultOrg`, `requiredTestSuites`).

### Phase 3 — IDE parity + baseline retrieve polish

1. Control IDE Ship uses same selective semantics + `environments/` stage order.
2. `org retrieve --baseline-only` (refresh `.one/baseline` without wiping customer YAML).
3. Cross-link BP-033 / BP-025 as production DX safety prerequisites — compat handshake design: [ide-api-version-compatibility-build-plan.md](./ide-api-version-compatibility-build-plan.md).

### Phase 4 — OS keychain + builder templates (BP-064)

1. `auth login` prefers OS keychain (`zalando/go-keyring`); file `0600` fallback; `ONE_CREDENTIAL_STORE`.
2. `project init` writes customer-owned `AGENTS.md` + `skills/{connect,query,customize,ship,govern}/SKILL.md`.
3. [builder-connect.md](../builder-connect.md) is the builder MCP + CLI recipe (no Control IDE required).

## Explicit non-goals

- Peer-to-peer promote
- Making Git the apply SoR
- Third-party DX plugin ecosystems / in-kernel language parity
- Multi-org fan-out in one invocation
- Scratch-org product (Wave D / Deploy cloud)

## Success criteria

- New tenant: `project init` → `auth login` → `org validate` → `org deploy` with no Electron and no pasted URL every command.
- CI installs CLI from product release assets (not ad-hoc `ONE_CLI_URL` only).
- Selective path deploy packs a single object folder without the whole tree.
- Docs prescribe repo→org only.
