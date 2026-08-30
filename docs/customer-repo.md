# Customer repository format (`one/v1`)

Normative layout for the auto-provisioned per-tenant AWS CodeCommit repository. See [ADR-012](./adr/012-customer-repo-and-control-ide.md).

## Identity

| Concept | Value |
|---|---|
| Repo name | `one-<CUSTOMER_ID>` |
| Format | `repoFormat: one/v1` in `one.yaml` |
| Default package | `customer.default` (matches Deploy `DefaultTenantPackage`) |
| Runtime apply | Deploy bundle → validate → tests → promote |

One repo per `CUSTOMER_ID`. Test/staging/prod installs share it; they do not each get a repo.

## Tree

```text
one-<CUSTOMER_ID>/
├── README.md
├── AGENTS.md                        # builder instructions (project init)
├── skills/                          # SKILL.md fragments: connect, query, customize, ship, govern
│   └── connect/SKILL.md
├── one.yaml
├── .one/
│   ├── ignore
│   └── baseline/                    # read-only managed reference (never packed)
│       ├── manifest.yaml
│       ├── objects/
│       │   └── Account.yaml
│       └── fields/
│           └── Account/
│               └── Name.yaml
├── metadata/
│   ├── objects/
│   │   └── MyObject__c.yaml
│   ├── fields/
│   │   └── MyObject__c/
│   │       └── Region__c.yaml
│   ├── validation-rules/
│   │   └── MyObject__c/
│   │       └── Region_Required.yaml
│   ├── automations/
│   │   └── Notify_On_Create.yaml
│   ├── permission-sets/
│   │   └── Builder_Base.yaml
│   ├── webhooks/
│   │   └── External_CRM.yaml
│   ├── agents/
│   │   └── playbooks/
│   │       └── Triage_Case.yaml
│   ├── canvases/
│   │   └── BigDeals_Pipeline.yaml   # deprecated alias — prefer tools/ (ADR-018 → ADR-021)
│   ├── tools/
│   │   └── Sales_Open_Pipeline.yaml # ToolSpec (ADR-021 / Run mode; optional)
│   ├── experiences/
│   │   └── Acme_Portal.yaml         # Client Experience config (ADR-019; optional)
│   ├── data-roles/
│   │   └── Sales.yaml
│   ├── object-sharing/
│   │   └── MyObject__c.yaml
│   └── sharing-rules/
│       └── MyObject__c/
│           └── Sales_Read.yaml
├── src/
│   └── automations/
│       └── create_opp_on_account.ts
├── tests/
│   ├── SmokeMetadata.yaml
│   └── automations/
│       └── create_opp_on_account_test.ts
├── environments/
│   ├── test.yaml
│   ├── staging.yaml
│   └── prod.yaml
├── manifests/
│   └── sample-objects.yaml          # optional selective path list
├── data/                            # optional one-datapack/v1 (business rows; not Deploy)
│   └── demo-seed/
│       ├── datapack.yaml
│       ├── accounts.jsonl
│       └── contacts.jsonl
└── changes/
    ├── _template/
    │   └── CHANGE.yaml
    └── <slug>/
        └── CHANGE.yaml              # created by `one change create` / IDE New change
```

## `one.yaml`

```yaml
customerId: acme-corp
packageName: customer.default
productVersionRange: ">=0.1.0 <0.2.0"
repoFormat: one/v1
# Optional DX defaults:
# defaultOrg: test
# requiredTestSuites:
#   - CreateAccountFromContact
# apiVersion: "1"
```

## Metadata file shapes

YAML maps to Deploy snapshot JSON fields (`internal/deploy` types). Every customer artifact must use `ownership: custom`. Managed package names are rejected at pack time.

| Path | Primary keys | Notes |
|---|---|---|
| `metadata/objects/<apiName>.yaml` | `apiName`, `label`, `pluralLabel`, `storageMode`, `features` | |
| `metadata/fields/<object>/<apiName>.yaml` | `objectApiName`, `apiName`, `label`, `fieldType`, … | |
| `metadata/validation-rules/<object>/<apiName>.yaml` | `objectApiName`, `apiName`, `expression`, … | |
| `metadata/automations/<apiName>.yaml` | `apiName`, `objectApiName`, `triggerEvent`, `execution`, `entryFile`, … | Code automations: pair with `src/automations/` ([ADR-014](./adr/014-customer-code-automations.md)) |
| `src/automations/<name>.ts` | `export default async function run(ctx)` | No third-party imports; Deno guest only |
| `tests/automations/<name>_test.ts` | Unit tests against mock `ctx` | Pack / Deploy gate |
| `metadata/permission-sets/<apiName>.yaml` | `apiName`, `label`, object/field/system permissions | Definitions only; assignments stay Client |
| `metadata/webhooks/<apiName>.yaml` | `apiName`, `url`, `eventTypes`, `active` | Secrets never stored in Git |
| `metadata/agents/playbooks/<apiName>.yaml` | AgentSpec fields including `allowedSkills` (automation apiNames) | No provider secrets |
| `metadata/canvases/<apiName>.yaml` | Deprecated CanvasSpec path (alias during ToolSpec migration; [ADR-018](./adr/018-crm-canvas-document.md)) | Prefer `metadata/tools/` |
| `metadata/tools/<apiName>.yaml` | ToolSpec (rail chrome + `one.canvas/v1` document; [ADR-021](./adr/021-run-mode-toolspec.md)) | Declarative only — no JS/React assets; [ADR-021](./adr/021-run-mode-toolspec.md) |
| `metadata/experiences/<apiName>.yaml` | Client Experience (`homeUrl`, `connectedAppApiName`, `allowedOrigins`; [ADR-019](./adr/019-client-experience-oss-kits.md)) | Config only — SPA code hosted on customer infra |
| Connectors / install secrets / egress | Managed via Metadata API on each install | Secret ciphertext is install-local; re-bind after promote |
| `metadata/data-roles/<apiName>.yaml` | `apiName`, `label`, optional parent | ADR-016 |
| `metadata/object-sharing/<object>.yaml` | `objectApiName`, `defaultAccess`, `sharingRulesEnabled` | OWD |
| `metadata/sharing-rules/<object>/<apiName>.yaml` | criteria rule fields | |
| `.one/baseline/**` | managed objects/fields + `manifest.yaml` | **Read-only**; ignored by pack/promote |
| `tests/<apiName>.yaml` | `apiName`, `label`, `steps` | Customer test suites |

## Environments

`environments/<role>.yaml` holds non-secret install pointers. `one` org hints read these files (secrets stay in `auth login` — OS keychain, else `credentials.json` mode `0600`). Control IDE Ship is an optional frozen twin.

```yaml
installId: acme-test
installRole: test
baseUrl: https://one-test.example.com
# optional alias: test
```

## Data packs (optional, BP-041)

`data/<packName>/datapack.yaml` describes an ordered multi-object sync. **Preferred source is a peer org** named by `sourceEnv` (resolves to `environments/<role>.yaml`). Apply authenticates to the source and target with Connected App / `auth login` aliases — Git holds the recipe + peer pointer, not PII dumps.

```yaml
apiVersion: one-datapack/v1
name: crm-seed
sourceEnv: test
steps:
  - id: accounts
    object: Account
    operation: upsert
    externalIdField: ERP_Id__c
    query: { select: [Id, Name, ERP_Id__c] }
  - id: contacts
    object: Contact
    operation: upsert
    externalIdField: ERP_Id__c
    after: [accounts]
    query: { select: [Id, ERP_Id__c, AccountId] }
    references:
      - field: AccountId
        toObject: Account
        toExternalIdField: ERP_Id__c
```

```bash
one auth login --alias test --base-url https://… --token …
one auth login --alias prod --base-url https://… --token …
one datapack validate data/crm-seed -dir .
one datapack apply data/crm-seed --alias prod --source-alias test -dir .
```

Offline/demo: add step `file: rows.jsonl` and `datapack apply … --offline`.

See [external-id-upsert-bulk-build-plan.md](./architecture/external-id-upsert-bulk-build-plan.md).

## Changes

`changes/<slug>/CHANGE.yaml` is the reviewable change record for branch `change/<slug>`:

```yaml
title: "Add Referral fields"
risk: low
targetEnvs:
  - staging
  - prod
summary: "Describe the metadata change"
```

Create via `one change create <slug>`. Control IDE Repo → **New change…** is an optional twin.

## Selective manifests

Optional `manifests/<name>.yaml` lists path prefixes for selective pack (Majesta One-native — not `package.xml`):

```yaml
paths:
  - metadata/objects/Referral__c.yaml
  - metadata/fields/Referral__c/
```

CLI: `one org validate --manifest sample-objects` or `--metadata metadata/objects/Foo__c.yaml`.

## Branch / PR → Deploy

| Git | Deploy |
|---|---|
| Branch `change/<slug>` | Local work + CHANGE.yaml |
| CI / IDE pack | Pack tree (or selective paths) → validate-local |
| Validate + customer tests | Gate |
| Merge / approve | `org deploy` on the **target** install |

## Forbidden in the repo

- Majesta One product source (`cmd/`, `internal/`, …)
- Raw business `records` / PII dumps (curated non-PII **data packs** under `data/` are optional — [BP-041](../backlog/BP-041-record-external-id-upsert-bulk.md) / [external-id-upsert-bulk-build-plan.md](./architecture/external-id-upsert-bulk-build-plan.md); apply via Client, not Deploy)
- API keys, JWTs, Cognito secrets, webhook plaintext secrets
- Managed package definitions (`core`, `platform`, module internals)
- `node_modules/`, lockfiles for customer automation deps, or any third-party package imports (ADR-014 v1 ban)

## Tooling

| Tool | Role |
|---|---|
| `internal/customerrepo` | Pack / unpack / validate format; environments; changes; manifests; scaffold |
| `cmd/one` | Product CLI: `auth` (OS keychain), `project init` (includes builder `AGENTS.md` + skills), `change create`, `pack`/`validate`, `org validate\|deploy\|retrieve` ([BP-048](../backlog/BP-048-one-cli.md) · [builder-connect.md](./builder-connect.md)) |
| `POST /deploy/v1/packages/pack` | Upload zip/tar of tree → bundle |
| `POST /deploy/v1/packages/validate-local` | Diff + ValidateBundleArtifact (org validate) |
| `GET /deploy/v1/packages/export` | Current install → `one/v1` zip (includes baseline) |
| `POST /deploy/v1/packages/initialize-repo` | Admin+deploy: seed remote `main` from this install via go-git |
| Control IDE (`tools/control-ide`) | Optional JWT client — frozen chrome; not required to ship |

## Related

- [customer-customizations.md](./customer-customizations.md)
- [customer-developer-workflow.md](./customer-developer-workflow.md)
- [architecture/one-cli-build-plan.md](./architecture/one-cli-build-plan.md)
- [builder-connect.md](./builder-connect.md)
- [multi-env-deploy.md](./multi-env-deploy.md)
- [api-families.md](./api-families.md)
- [ci-customer-tests.md](./ci-customer-tests.md)
- [customer-repo-init-build-plan.md](./architecture/customer-repo-init-build-plan.md)
- [BP-031](../backlog/BP-031-customer-repo-init-sync.md) · [BP-032](../backlog/BP-032-customer-dx-validate-deploy.md) · [BP-048](../backlog/BP-048-one-cli.md) · [BP-041](../backlog/BP-041-record-external-id-upsert-bulk.md)
