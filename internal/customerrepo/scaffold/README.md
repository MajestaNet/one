# Majesta One customer implementation repository (`one/v1`)

This repository holds **customer-owned** metadata, AgentSpecs, and Deploy tests for one commercial customer.
It is not Majesta One product source. Ship with **repo → org** validate/deploy (never peer promote). Control IDE is optional.

## Quick start

### CLI (path of record)

```bash
one project init -dir . --customer-id REPLACE_CUSTOMER_ID
one auth login --base-url https://<install> --token "$ONE_JWT" --alias test
# edit metadata/ and src/, then:
one org validate -dir .
one org deploy -dir . --suite CreateAccountFromContact
```

`--suite` runs **after** apply so a first deploy can create the suite, then execute it. Guest automation assertions need Deno 2.9.3+ on `PATH`. `org deploy` does not delete extra customer metadata that exists only on the install (delete-by-absence is informational in v1).

Point an MCP host at `POST /mcp` (see `AGENTS.md` and `skills/connect/SKILL.md`). Pin `One-API-Revision` from `GET /version` (`apiRevision.recommended` or `current`).

### Control IDE (optional)

Connect as a JWT client (**Settings → Environments**). It is not required to build or ship.

## Sample contents

| Artifact | Purpose |
|---|---|
| `Referral__c` | Custom object with `ContactId` + `AccountId` lookups |
| `CreateAccount_From_Contact` | Code automation: Contact create → Account + Referral |
| Suite `CreateAccountFromContact` | `objectExists` + `automationUnitPass` + `automationContract` |
| `AGENTS.md` + `skills/*/SKILL.md` | Builder instructions (connect, query, customize, ship, govern, skill) |

## Layout

```
AGENTS.md
skills/{connect,query,customize,ship,govern,skill}/SKILL.md
metadata/objects|fields|validation-rules|automations|permission-sets|webhooks|agents/playbooks|tools|canvases
src/automations/          # guest TypeScript (Deno)
tests/
  *.yaml                  # Deploy suites (incl. automationUnitPass / automationContract)
  automations/            # unit harness guests
environments/
manifests/
changes/
```

Repo format: [docs/customer-repo.md](../../../docs/customer-repo.md). Automations use the frozen guest SDK (`one:automation`). Never commit secrets, API keys, or business records.

`AGENTS.md` and `skills/` are customer-owned after `project init`. They are not packed into Deploy bundles.
