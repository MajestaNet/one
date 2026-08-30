# Majesta One customer implementation repository (`one/v1`)

This repository holds **customer-owned** metadata, AgentSpecs, and Deploy tests for one commercial customer.
It is not Majesta One product source. Ship with **repo → org** validate/deploy (never peer promote).

## Quick start

### CLI (path of record)

```bash
one project init -dir . --customer-id acme
one auth login --base-url https://test.example --token "$JWT" --alias test
one org validate -dir .
one org deploy -dir . --suite CreateAccountFromContact
```

`--suite` runs **after** the pack is applied so a first deploy can create the suite, then execute it. Guest automation assertions need Deno 2.9.3+ on `PATH`. `org deploy` does not delete extra customer metadata that exists only on the install (delete-by-absence is informational in v1).

See [docs/architecture/one-cli-build-plan.md](../../docs/architecture/one-cli-build-plan.md).

### Control IDE (optional)

1. Connect to a Majesta One install (**Settings → Environments**).
2. Build → **Repo** → **Initialize remote** (once) → **Sync from Git remote** / **Pull from org**.
3. **New change…** creates `change/<slug>` + `changes/<slug>/CHANGE.yaml`.
4. Edit Objects / Agents (YAML dual-write); commit in your editor.
5. Build → Deploy → **Validate vs org** → tests → **Deploy to org**.

## Sample contents

| Artifact | Purpose |
|---|---|
| `Referral__c` | Custom object with `ContactId` + `AccountId` lookups |
| `CreateAccount_From_Contact` | Code automation: Contact create → Account + Referral |
| Suite `CreateAccountFromContact` | `objectExists` + `automationUnitPass` + `automationContract` |
| `manifests/sample-objects.yaml` | Selective pack path list example |
| `AGENTS.md` + `skills/*/SKILL.md` | Builder instructions (connect, query, customize, ship, govern, skill) |

## Layout

```
AGENTS.md
skills/{connect,query,customize,ship,govern,skill}/SKILL.md
one.yaml
metadata/objects|fields|validation-rules|automations|permission-sets|webhooks|agents/playbooks|tools|canvases|experiences|…
src/automations/          # guest TypeScript (Deno)
tests/                    # Deploy suites + automations harness
environments/             # non-secret install pointers (CLI/IDE stage order)
changes/<slug>/CHANGE.yaml
manifests/<name>.yaml     # optional selective path lists
.one/baseline/        # managed reference — never packed
```

SDK: [docs/automation-sdk.md](../../docs/automation-sdk.md). Repo format: [docs/customer-repo.md](../../docs/customer-repo.md).

Never commit secrets, API keys, or business records.
