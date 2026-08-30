# Customer repo init — consistent customization model

**Status:** Active (implementation in progress / shipped in this change set)  
**Playbooks:** [agent-deploy.md](./agent-deploy.md) · [agent-control-ide.md](./agent-control-ide.md) · [agent-api-families.md](./agent-api-families.md)  
**Related:** [ADR-012](../adr/012-customer-repo-and-control-ide.md) · [customer-repo.md](../customer-repo.md) · [BP-031](../../backlog/BP-031-customer-repo-init-sync.md) · [BP-023](../../backlog/BP-048-one-cli.md)

## Goal

Make customer customizations follow one consistent model:

1. Admin on **prod** initializes the remote customer Git repo from the install snapshot.
2. Repo layout is **one folder per metadata type**, plus a **read-only managed baseline**.
3. Control IDE picks a local folder and **clones** or **pulls `origin/main`**.
4. Git stays out of the product image: day-to-day Git is IDE/client; remote seed uses **pure-Go go-git**.

## Locked decisions

| Decision | Choice |
|---|---|
| Baseline | Customer-owned artifacts **plus** read-only managed object/field defs under `.one/baseline/` |
| Initialize target | Seed remote `main` **and** IDE clone/pull locally |
| Git in API | No OS `git` binary; `internal/deploy/gitremote` via go-git when `CUSTOMER_REPO_URL` is set |

## Workflow

```text
Admin (JWT admin+deploy) on prod
  → POST /deploy/v1/packages/initialize-repo { confirm: true }
  → snapshot (customer + managed baseline) → unpack one/v1 → go-git push main

Control IDE
  → Connect to prod
  → Build → Repo → Initialize remote (if needed)
  → Browse local folder → Sync local
       empty / no git  → clone customerRepoUrl
       existing repo   → fetch + ff-only pull origin/main
  → Edit customer metadata / automations → commit → push
  → Ship → pack HEAD → validate → promote
```

## Repo layout amendment (`one/v1`)

Additive folders only (same `repoFormat`):

- `.one/baseline/{manifest.yaml,objects/,fields/}` — managed reference; **never packed/promoted**
- `metadata/data-roles/`, `metadata/object-sharing/`, `metadata/sharing-rules/<Object>/` — ADR-016 sharing

Pack paths remain `metadata/**` (customer), `src/automations/**`, `tests/**`.

## API

| Route | Auth | Role |
|---|---|---|
| `POST /deploy/v1/packages/initialize-repo` | `scope:deploy` + admin + `CapDeployPromote` | Seed remote |
| `GET /deploy/v1/packages/export` | `scope:deploy` | Air-gap zip (includes baseline) |
| `GET /deploy/v1/environment` | `scope:deploy` | `packageInitializeRepo` capability + `customerRepoUrl` |

Env: `CUSTOMER_REPO_URL`, optional `CUSTOMER_REPO_GIT_USER` / `CUSTOMER_REPO_GIT_TOKEN`.

## IDE

- **Initialize remote**, **Sync local**, Advanced: sample / import export / force clone
- IPC: `git:pull`, `repo:importExportZip`
- Writes to `.one/baseline/**` rejected; managed ownership skipped in YAML dual-write

## Non-goals

- Git as apply SoR
- OS `git` in product images
- Editable managed metadata / promote managed via Deploy
- Full DO-native Git host provisioning (operator-supplied URL + token works today)
