# Customer developer workflow (best practice)

How a customer or SI should develop, review, and ship Majesta One customizations. Product binaries stay shared; implementation lives in the customer Git repo (`one/v1`) and install DBs. See [ADR-012](./adr/012-customer-repo-and-control-ide.md), [customer-customizations.md](./customer-customizations.md), and the DX plan [customer-dx-build-plan.md](./architecture/customer-dx-build-plan.md).

## Roles of the three planes

| Plane | What it is | Do not |
|---|---|---|
| **Product** | Majesta One API/worker images | Fork per customer; commit customer YAML into product Git |
| **Customer Git** | Reviewable source of change (`metadata/`, `src/automations/`, `tests/`) | Store secrets, records/PII, or edit managed baseline as if shippable |
| **Install DB** | Runtime SoR **after** org deploy | Treat Git push or another install’s Deploy API as already-live |

`.one/baseline/` is a **read-only** managed reference. Never pack or “fix” it via Deploy.

## Core rule: repo → org (never install → install)

```text
GitHub / GitLab / Bitbucket / CodeCommit
        └── one-<CUSTOMER_ID>

For each target install (test, staging, prod):
  connect → org validate (from local/CI checkout) → org deploy
```

- **Validate only first, always** — do not deploy until org validate is green for that pack.
- **Do not peer-promote** bundles from test to prod. That legacy path is not the modern DX model.
- Multi-env = same Git revision, **different connected org**, validate + deploy again.

## Recommended topology

```text
customer Git (one repo per CUSTOMER_ID)
        │
        ├─→ test     validate + deploy (daily)
        ├─→ staging  validate + deploy (optional)
        └─→ prod     validate + deploy (release); also initialize-repo once
```

- Host the remote on **any** HTTPS Git provider the customer controls.
- Point installs at the same `CUSTOMER_REPO_URL` when using initialize-repo / IDE Sync.
- Prefer **test** for day-to-day DX; connect to **prod** for release deploys and one-time repo init.

## Golden path (day-to-day)

```text
1. Connect one (or MCP) to the test install
2. Sync local folder from customer Git (clone or pull main / change/*)
3. one change create <slug>; edit YAML/TS (or Objects / Automations in an optional IDE)
4. Commit + push for review
5. one org validate   ← required; safe; no org mutation
   (MCP org_validate is the same Deploy API)
6. one org deploy [--suite <apiName>]  ← apply, then run customer tests
7. Release to staging/prod:
     one org use <alias>  (or CI job with that base URL)
     checkout the same Git SHA
     org validate → org deploy
```

CLI (productized — [BP-048](../backlog/BP-048-one-cli.md)):

```bash
one project init -dir . --customer-id acme
one auth login --base-url https://test.example --token "$ONE_JWT" --alias test

# PR / local — validate only
one org validate -dir .

# After green validate — deploy to that org (suite runs after apply)
one org deploy -dir . --suite SmokeMetadata

# Production release — same commit, different org
one org use prod   # or: --alias prod / ONE_ORG=prod
one org validate -dir .
one org deploy -dir .
```

## Branching and review

| Practice | Guidance |
|---|---|
| Default branch | `main` — keep deployable |
| Feature work | `change/<slug>` — validate from the branch before merge |
| PR checks | CI: `org validate` against **test**; fail on validation errors |
| Release | Tag or merge to `main`, then validate+deploy that SHA to staging/prod |
| Reviews | Review YAML/TS in Git — not peer Deploy payloads |

## Validate vs deploy

| Step | Purpose |
|---|---|
| **Validate vs org** | Diff local vs install + Deploy ownership/compat gates. **Always first.** No metadata apply. Removes and baseline rows are informational (v1 does not delete extra org metadata). |
| **Customer tests** | Behavioral gate on the target install; `one org deploy --suite` runs **after** apply so a first deploy can create the suite. |
| **Deploy to org** | Apply pack to the **connected** install only after green validate. |

There is no “Promote to peer” step in this workflow.

## Git host checklist (agnostic)

1. Create empty private repo on GitHub/GitLab/Bitbucket/CodeCommit.
2. Set install env: `CUSTOMER_REPO_URL`, optional `CUSTOMER_REPO_GIT_USER` / `CUSTOMER_REPO_GIT_TOKEN`.
3. Admin on prod: **Initialize remote** once.
4. Developers: normal `git` auth for clone/push; Majesta One JWT only for API validate/deploy.

## Local hygiene

- Clean working tree before validate/deploy from HEAD.
- Never commit secrets or record dumps.
- After product upgrades, re-run tests; refresh baseline via initialize-repo / export when managed shapes change.

## Anti-patterns

| Avoid | Why |
|---|---|
| Peer-to-peer / install→install bundle promote | Bypasses repo as source of truth; legacy path |
| Deploy without org validate | Violates validate-first invariant |
| Editing `.one/baseline` as shippable source | Not packable; managed reference only |
| Treating `git push` as live on an org | Orgs change only via org deploy |
| One Git repo per `INSTALL_ID` | One repo per `CUSTOMER_ID` |
| Checking customer fixtures into the product monorepo | Breaks product≠customer boundary |

## Related

- [customer-rollout-test-run.md](./customer-rollout-test-run.md) — scored SI journey (two installs, 2+ IDEs, MCP + `one`) and [gap log](./customer-rollout-gap-log.md)
- [customer-dx-build-plan.md](./architecture/customer-dx-build-plan.md)
- [customer-repo-init-build-plan.md](./architecture/customer-repo-init-build-plan.md)
- [customer-repo.md](./customer-repo.md)
- [multi-env-deploy.md](./multi-env-deploy.md) — install identity; multi-env is repo→org (peer push removed)
- [ci-customer-tests.md](./ci-customer-tests.md)
- [customer-ide-ux.md](./customer-ide-ux.md)
