#!/usr/bin/env bash
# Write the customer-install simulation fixtures into gitignored .customer-sandbox/.
# Does not start APIs, enable packages, or org-deploy. See
# docs/customer-install-simulation-test-run.md card S-B / S-C.
#
#   scripts/customer-install-sim-generate.sh
#   STUB_COUNT=48 scripts/customer-install-sim-generate.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SANDBOX="${SANDBOX:-${ROOT}/.customer-sandbox/one-acme-sim}"
STUB_COUNT="${STUB_COUNT:-48}"
CUSTOMER_ID="${CUSTOMER_ID:-acme-sim}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing $1" >&2; exit 2; }; }
need mkdir
need python3

if [[ ! -f "${SANDBOX}/one.yaml" ]]; then
  echo "== one project init → ${SANDBOX} =="
  mkdir -p "$(dirname "$SANDBOX")"
  go run ./cmd/one project init -dir "$SANDBOX" --customer-id "$CUSTOMER_ID"
fi

python3 - "$SANDBOX" "$CUSTOMER_ID" "$STUB_COUNT" <<'PY'
import pathlib, sys, textwrap

root = pathlib.Path(sys.argv[1])
customer_id = sys.argv[2]
stub_count = int(sys.argv[3])
if stub_count < 1 or stub_count > 200:
    raise SystemExit(f"STUB_COUNT must be 1–200, got {stub_count}")

# Drop the product sample (Referral__c / CreateAccount_From_Contact) so this
# sandbox is the simulation story only. Skills / README from init may stay.
for rel in (
    "metadata/objects/Referral__c.yaml",
    "metadata/fields/Referral__c/AccountId.yaml",
    "metadata/fields/Referral__c/ContactId.yaml",
    "metadata/automations/CreateAccount_From_Contact.yaml",
    "src/automations/create_account_from_contact.ts",
    "tests/CreateAccountFromContact.yaml",
    "tests/automations/create_account_from_contact_test.ts",
):
    p = root / rel
    if p.exists():
        p.unlink()
ref_dir = root / "metadata/fields/Referral__c"
if ref_dir.is_dir() and not any(ref_dir.iterdir()):
    ref_dir.rmdir()

def write(rel, body):
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(body).lstrip("\n"), encoding="utf-8")

write("one.yaml", f"""
# one/v1 — customer-install simulation (lab only; never commit to product Git)
customerId: {customer_id}
packageName: customer.default
productVersionRange: ">=0.1.0 <0.2.0"
repoFormat: one/v1
defaultOrg: dev
requiredTestSuites:
  - SiteVisitFromOpportunity
""")

for role, install, port in (
    ("dev", "acme-dev", 8082),
    ("test", "acme-test", 8081),
    ("prod", "acme-prod", 8080),
):
    write(f"environments/{role}.yaml", f"""
installId: {install}
installRole: {role}
baseUrl: http://localhost:{port}
# Loopback baseUrl is for CLI hints only. Do not POST it as a peer baseUrl (SSRF guard).
""")

write("metadata/objects/SiteVisit__c.yaml", """
apiName: SiteVisit__c
label: Site Visit
pluralLabel: Site Visits
storageMode: flexible
ownership: custom
packageName: customer.default
features: {}
""")

write("metadata/objects/ScalePing__c.yaml", """
apiName: ScalePing__c
label: Scale Ping
pluralLabel: Scale Pings
storageMode: flexible
ownership: custom
packageName: customer.default
features: {}
""")

fields = [
    ("SiteVisit__c", "Name", "Name", "text", True, None),
    ("SiteVisit__c", "Status", "Status", "picklist", True, None),
    ("SiteVisit__c", "VisitDate", "Visit Date", "date", False, None),
    ("SiteVisit__c", "AccountId", "Account", "lookup", False, "Account"),
    ("SiteVisit__c", "OpportunityId", "Opportunity", "lookup", True, "Opportunity"),
    ("SiteVisit__c", "ProjectId", "Project", "lookup", False, "Project"),
    ("SiteVisit__c", "ContactId", "Contact", "lookup", False, "Contact"),
    ("ScalePing__c", "Name", "Name", "text", True, None),
    ("Account", "LastSiteVisitId__c", "Last Site Visit", "lookup", False, "SiteVisit__c"),
    ("Opportunity", "SiteVisitCount__c", "Site Visit Count", "number", False, None),
    ("Project", "EngagementFlag__c", "Engagement Flag", "boolean", False, None),
]
for obj, api, label, ftype, required, ref in fields:
    extra = ""
    if ftype == "lookup":
        extra = f"referenceTo: {ref}\n"
    if ftype == "picklist":
        extra = 'picklistValues: ["Planned", "In Progress", "Completed", "Cancelled"]\n'
    write(
        f"metadata/fields/{obj}/{api}.yaml",
        f"""
objectApiName: {obj}
apiName: {api}
label: {label}
fieldType: {ftype}
required: {str(required).lower()}
{extra}ownership: custom
packageName: customer.default
""",
    )

write("metadata/validation-rules/SiteVisit__c/Name_Required.yaml", """
objectApiName: SiteVisit__c
apiName: Name_Required
label: Name required
active: true
errorMessage: Site Visit Name is required
ownership: custom
packageName: customer.default
expression:
  "!":
    var: Name
""")

def auto_yaml(api, label, obj, event, execution, entry):
    write(
        f"metadata/automations/{api}.yaml",
        f"""
apiName: {api}
label: {label}
objectApiName: {obj}
triggerEvent: {event}
active: true
runtime: code
execution: {execution}
entryFile: src/automations/{entry}
ownership: custom
packageName: customer.default
actions: []
""",
    )

auto_yaml("CreateSiteVisit_From_Opportunity", "Create Site Visit from Opportunity", "Opportunity", "create", "async", "create_site_visit_from_opportunity.ts")
auto_yaml("StampAccount_LastVisit", "Stamp Account last site visit", "SiteVisit__c", "create", "async", "stamp_account_last_visit.ts")
auto_yaml("CreateProjectTask_From_Visit", "Create Project Task from Site Visit", "SiteVisit__c", "create", "async", "create_project_task_from_visit.ts")
auto_yaml("ConvertLead_When_Qualified", "Convert Lead when Qualified", "Lead", "update", "async", "convert_lead_when_qualified.ts")
auto_yaml("Reject_Missing_Opportunity", "Reject Site Visit without Opportunity", "SiteVisit__c", "create", "sync", "reject_missing_opportunity.ts")
auto_yaml("Fanout_TimeEntries", "Fan-out Time Entries from Project", "Project", "create", "async", "fanout_time_entries.ts")
auto_yaml("CloseVisit_When_Opp_Closed", "Close Site Visits when Opportunity closes", "Opportunity", "update", "async", "close_visit_when_opp_closed.ts")
auto_yaml("CreateQuote_When_Won", "Create Quote when Opportunity is won", "Opportunity", "update", "async", "create_quote_when_won.ts")
auto_yaml("Expense_When_Visit_Complete", "Create Expense when Site Visit completes", "SiteVisit__c", "update", "async", "expense_when_visit_complete.ts")

write("src/automations/create_site_visit_from_opportunity.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const name = String(ctx.trigger.data?.Name ?? "Untitled Opportunity").trim();
  await ctx.createRecord({
    objectApiName: "SiteVisit__c",
    data: {
      Name: `${name} kickoff visit`,
      Status: "Planned",
      OpportunityId: ctx.trigger.recordId,
      AccountId: ctx.trigger.data?.AccountId ?? null,
      ContactId: ctx.trigger.data?.ContactId ?? null,
    },
  });
  ctx.log("CreateSiteVisit_From_Opportunity created SiteVisit__c for", name);
  return { ok: true };
}
""")

write("src/automations/stamp_account_last_visit.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const accountId = ctx.trigger.data?.AccountId;
  if (!accountId) {
    ctx.log("StampAccount_LastVisit skipped — no AccountId");
    return { ok: true };
  }
  await ctx.updateRecord({
    objectApiName: "Account",
    id: String(accountId),
    data: { LastSiteVisitId__c: ctx.trigger.recordId },
  });
  return { ok: true };
}
""")

write("src/automations/create_project_task_from_visit.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const projectId = ctx.trigger.data?.ProjectId;
  if (!projectId) {
    ctx.log("CreateProjectTask_From_Visit skipped — no ProjectId");
    return { ok: true };
  }
  const name = String(ctx.trigger.data?.Name ?? "Site visit").trim();
  await ctx.createRecord({
    objectApiName: "ProjectTask",
    data: {
      Name: `${name} follow-up`,
      ProjectId: projectId,
      Status: "Not Started",
    },
  });
  return { ok: true };
}
""")

write("src/automations/convert_lead_when_qualified.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const status = String(ctx.trigger.data?.Status ?? "");
  if (status !== "Qualified") {
    return { ok: true };
  }
  await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: ctx.trigger.recordId, createOpportunity: true },
  });
  return { ok: true };
}
""")

write("src/automations/reject_missing_opportunity.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

/** Sync: fail closed so the triggering create rolls back (ADR-014). */
export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const opp = ctx.trigger.data?.OpportunityId;
  if (!opp) {
    ctx.log("Reject_Missing_Opportunity: OpportunityId required");
    return { ok: false };
  }
  return { ok: true };
}
""")

write("src/automations/fanout_time_entries.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const projectId = ctx.trigger.recordId;
  const name = String(ctx.trigger.data?.Name ?? "Project").trim();
  for (let i = 1; i <= 3; i++) {
    await ctx.createRecord({
      objectApiName: "TimeEntry",
      data: {
        Name: `${name} seed hours ${i}`,
        ProjectId: projectId,
        Hours: 1,
        Status: "Draft",
      },
    });
  }
  return { ok: true };
}
""")

write("src/automations/close_visit_when_opp_closed.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const closed = ctx.trigger.data?.IsClosed === true || ctx.trigger.data?.IsClosed === "true";
  if (!closed) {
    return { ok: true };
  }
  const found = await ctx.query({
    objectApiName: "SiteVisit__c",
    filters: [{ field: "OpportunityId", op: "eq", value: ctx.trigger.recordId }],
  });
  const rows = (found?.records ?? found?.items ?? []) as Array<{ Id?: string; id?: string }>;
  for (const row of rows) {
    const id = String(row.Id ?? row.id ?? "");
    if (!id) continue;
    await ctx.updateRecord({
      objectApiName: "SiteVisit__c",
      id,
      data: { Status: "Cancelled" },
    });
  }
  return { ok: true };
}
""")

write("src/automations/create_quote_when_won.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const won = ctx.trigger.data?.IsWon === true || ctx.trigger.data?.IsWon === "true";
  if (!won) {
    return { ok: true };
  }
  const name = String(ctx.trigger.data?.Name ?? "Opportunity").trim();
  await ctx.createRecord({
    objectApiName: "Quote",
    data: {
      Name: `${name} accepted quote`,
      Status: "Draft",
      OpportunityId: ctx.trigger.recordId,
      AccountId: ctx.trigger.data?.AccountId ?? null,
      ContactId: ctx.trigger.data?.ContactId ?? null,
    },
  });
  return { ok: true };
}
""")

write("src/automations/expense_when_visit_complete.ts", """
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  if (String(ctx.trigger.data?.Status ?? "") !== "Completed") {
    return { ok: true };
  }
  const projectId = ctx.trigger.data?.ProjectId;
  if (!projectId) {
    ctx.log("Expense_When_Visit_Complete skipped — no ProjectId");
    return { ok: true };
  }
  const name = String(ctx.trigger.data?.Name ?? "Visit").trim();
  await ctx.createRecord({
    objectApiName: "Expense",
    data: {
      Name: `${name} close-out`,
      ProjectId: projectId,
      Amount: 0,
      Category: "Other",
      Status: "Draft",
    },
  });
  return { ok: true };
}
""")

write("tests/automations/create_site_visit_from_opportunity_test.ts", """
import type { AutomationUnitContext } from "one:automation";

export default async function run(ctx: AutomationUnitContext) {
  await ctx.clearCalls();
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Opportunity",
      recordId: "00000000-0000-4000-8000-0000000000aa",
      data: { Name: "North Plant", AccountId: "00000000-0000-4000-8000-0000000000ac" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (!calls || calls.length !== 1) {
    throw new Error(`expected 1 createRecord, got ${calls?.length ?? 0}`);
  }
  if (calls[0].objectApiName !== "SiteVisit__c") {
    throw new Error(`expected SiteVisit__c, got ${calls[0].objectApiName}`);
  }
  const data = (calls[0].data ?? {}) as Record<string, unknown>;
  if (data.OpportunityId !== "00000000-0000-4000-8000-0000000000aa") {
    throw new Error("OpportunityId must match trigger");
  }
  return { ok: true };
}
""")

write("tests/automations/convert_lead_when_qualified_test.ts", """
import type { AutomationUnitContext } from "one:automation";

export default async function run(ctx: AutomationUnitContext) {
  await ctx.clearCalls();
  await ctx.runUnderTest({
    trigger: {
      action: "update",
      objectApiName: "Lead",
      recordId: "00000000-0000-4000-8000-0000000000ld",
      data: { LastName: "Nguyen", Status: "Qualified", Company: "Acme" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "invokeAction" });
  if (!calls || calls.length !== 1) {
    throw new Error(`expected 1 invokeAction, got ${calls?.length ?? 0}`);
  }
  if (calls[0].apiName !== "lead.convert") {
    throw new Error(`expected lead.convert, got ${calls[0].apiName}`);
  }
  return { ok: true };
}
""")

write("tests/automations/fanout_time_entries_test.ts", """
import type { AutomationUnitContext } from "one:automation";

export default async function run(ctx: AutomationUnitContext) {
  await ctx.clearCalls();
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Project",
      recordId: "00000000-0000-4000-8000-0000000000pj",
      data: { Name: "Plant turnaround" },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (!calls || calls.length !== 3) {
    throw new Error(`expected 3 TimeEntry creates, got ${calls?.length ?? 0}`);
  }
  for (const c of calls) {
    if (c.objectApiName !== "TimeEntry") {
      throw new Error(`expected TimeEntry, got ${c.objectApiName}`);
    }
  }
  return { ok: true };
}
""")

write("tests/SiteVisitFromOpportunity.yaml", """
apiName: SiteVisitFromOpportunity
label: Site Visit from Opportunity gate
description: Named automations + SiteVisit__c lookups onto enabled managed packages
active: true
ownership: custom
packageName: customer.default
steps:
  - type: objectExists
    objectApiName: SiteVisit__c
  - type: objectExists
    objectApiName: ScalePing__c
  - type: objectExists
    objectApiName: Opportunity
  - type: objectExists
    objectApiName: Project
  - type: objectExists
    objectApiName: Lead
  - type: automationUnitPass
    automationApiName: CreateSiteVisit_From_Opportunity
    testFile: tests/automations/create_site_visit_from_opportunity_test.ts
  - type: automationUnitPass
    automationApiName: ConvertLead_When_Qualified
    testFile: tests/automations/convert_lead_when_qualified_test.ts
  - type: automationUnitPass
    automationApiName: Fanout_TimeEntries
    testFile: tests/automations/fanout_time_entries_test.ts
  - type: automationContract
    automationApiName: CreateSiteVisit_From_Opportunity
    objectApiName: Opportunity
    data:
      Name: Simulation Opp
      StageName: Prospecting
      CloseDate: "2099-12-31"
    expectObjectApiName: SiteVisit__c
    expectMinRows: 1
    filters:
      - field: OpportunityId
        op: eq
        value: "$trigger.Id"
""")

for i in range(1, stub_count + 1):
    n = f"{i:02d}"
    api = f"ScaleStub_{n}"
    entry = f"scale_stub_{n}.ts"
    auto_yaml(api, f"Scale stub {n}", "ScalePing__c", "create", "async", entry)
    write(
        f"src/automations/{entry}",
        f"""
import type {{ AutomationContext, AutomationResult }} from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {{
  ctx.log("ScaleStub_{n}", ctx.trigger.recordId);
  return {{ ok: true }};
}}
""",
    )

# Negative fixture — not packed unless copied into src/automations/
write("_negative/forbidden_lodash_import.ts", """
import _ from "npm:lodash";
export default async function run() {
  return { ok: true };
}
""")

write("README.SIM.md", f"""
# Acme simulation customer repo (lab)

Generated by `scripts/customer-install-sim-generate.sh`. Gitignored.

- Custom objects: `SiteVisit__c` (lookups to Account, Opportunity, Project, Contact), `ScalePing__c`
- Custom fields on managed objects: `Account.LastSiteVisitId__c`, `Opportunity.SiteVisitCount__c`, `Project.EngagementFlag__c`
- Named code automations: 9
- Scale stubs: {stub_count} (`ScaleStub_01`… on `ScalePing__c` create)
- Negative import-ban file: `_negative/forbidden_lodash_import.ts` (copy into `src/automations/` only for the fail case)

Enable packages **on each install** before `one org deploy`: `catalog`, `sales`, `project_service`, `lead_marketing`.
""")

print(f"wrote {root}")
print(f"  named automations: 9")
print(f"  scale stubs: {stub_count}")
print(f"  negative fixture: _negative/forbidden_lodash_import.ts")
PY

echo "== done =="
echo "Sandbox: ${SANDBOX}"
echo "Next: enable packages on each install, then:"
echo "  go run ./cmd/one org validate -dir ${SANDBOX} --alias dev"
echo "  go run ./cmd/one org deploy  -dir ${SANDBOX} --alias dev --suite SiteVisitFromOpportunity"
