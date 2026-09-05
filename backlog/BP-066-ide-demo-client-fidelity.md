# BP-066: Control IDE demo-client fidelity (honest JWT client)

- **Severity:** Medium
- **Status:** Open (plan landed; implementation not started)
- **Track:** Finish — **explicit unfreeze of demo-client honesty**, not frozen Electron product chrome
- **Area:** `tools/control-ide/**` (primary); `internal/httpapi` only for documented API gaps in the plan
- **Design:** [ide-demo-client-uplift-build-plan.md](../docs/architecture/ide-demo-client-uplift-build-plan.md)
- **Related:** [ADR-030](../docs/adr/030-install-agent-runtime.md) §5 · [BP-065](./BP-065-ide-backend-coupling.md) · [BP-006](./BP-006-agent-guardrails.md) · [BP-048](./BP-048-one-cli.md) · Frozen chrome: [ADR-030 freeze table](../docs/architecture/agent-runtime-build-plan.md#freeze-vs-finish)

## Problem

The Go install implements a wide family HTTP surface (Client CRUD/query/search/composite/upsert/ingest, platform actions, automations invoke, Metadata objects/fields/packages/playbooks/ToolSpecs/experiences/sharing/inference, Deploy pack/validate/promote/cloud, Ops upgrades, Auth/SCIM/MCP) plus a hosted `/agents/runs` tool loop ([BP-006](./BP-006-agent-guardrails.md) mitigated).

Control IDE is an optional JWT demo client. Large parts of it already call live routes. Other parts **stub, auto-green, or call routes that are not registered**, so the demo disagrees with what the backend can do:

1. Chat still comments “there is no hosted tool loop”, auto-retries create with `approved: true`, and applies `graph.*` locally via `pendingToolApply` without `POST .../approve`.
2. Deploy marks customer tests **Passed** on HTTP 200 without reading the report / async work job.
3. Monitor prefers `/client/v1/debug/trace-flags` and `/debug/logs` — those handlers are **not** on the mux; ExecutionRun objects are still [BP-033](./BP-033-customer-runtime-isolation.md) Phase 3.
4. Offline CRM seed rows and seed agents can look like live data.
5. Shipped admin/builder APIs have no (or read-only) UI: FLS `fieldPermissions`, sharing OWD/rules, experiences CRUD, directory tags, data-roles, platform actions, ingest jobs, upsert, webhooks, projections rebuild, Ops read, MCP catalog copy.

ADR-030 freezes **new Electron-only product chrome** (license, update CDN, in-IDE agent host, Operate as end-user CRM). It does **not** require the demo client to lie about the install.

## Why it matters

Builders and reviewers use the IDE as a tour of the install. A stubbed chat and a fake-green Deploy pipeline teach the wrong product: that agents only stream text and that Ship is a local wizard. The honest demo is a thin Bearer client of the same APIs MCP and `one` already use.

## Direction (locked)

Follow [ide-demo-client-uplift-build-plan.md](../docs/architecture/ide-demo-client-uplift-build-plan.md).

| WS | Outcome |
|---|---|
| 0 | Honesty: no fake test green, no unregistered debug client, no seed data on a connected session, Automations back on the Build rail |
| 1 | Chat consumes hosted loop (`awaiting_tool_approval` + real approve) |
| 2 | Object Manager / Automations / Agents / Webhooks match Metadata |
| 3 | Govern FLS, sharing, experiences CRUD, directory, devices |
| 4 | Thin demo of actions, ingest, upsert, audit (not a CRM product) |
| 5 | Deploy work poll + Ops available-upgrades read |
| 6 | Do not deepen chrome Client island; gate new tools on product caps ([BP-065](./BP-065-ide-backend-coupling.md)) |

**Do not:** license JWS; update CDN; Operate record-UX / sales surfaces; peer-to-peer promote; in-IDE MCP host; new `ide.*` caps; `requireCapability(CapIDE*)` on family routes; wire `/client/v1/preferences` or principal canvases.

## Explicit non-goals

- Making Control IDE the GA path or Ship of record (`one` remains [BP-048](./BP-048-one-cli.md))
- End-user CRM (Client Experience / [BP-040](./BP-040-client-experience-oss-kits.md))
- Implementing BP-033 debug objects from the IDE side
- Keeping kernel chrome routes “because the panel needs them” ([BP-065](./BP-065-ide-backend-coupling.md) Phase 3)

## Implementation agent prompt

Paste after this docs PR is merged. Implement **WS0 first**, then WS1. Cite [agent-control-ide.md](../docs/architecture/agent-control-ide.md).

```text
Implement Majesta One BP-066 WS0 (Control IDE honesty pass).

Read first:
- docs/architecture/ide-demo-client-uplift-build-plan.md
- docs/architecture/agent-control-ide.md
- backlog/BP-066-ide-demo-client-fidelity.md

Scope (edit tools/control-ide/** only):
- DeployPanel: do not mark customer tests Passed on HTTP 200 alone
- MonitorPanel: stop treating unregistered /client/v1/debug/* as the primary API
- CrmPanel: no SEED rows when a JWT session is connected
- App.tsx: no stub agent replies when session.token is set
- Put automations back on MODE_WORKSPACE_TOOLS.build
- Fix the “no hosted tool loop” comment in agents/runs.ts

Verify: make test-ide
Out of scope: new panels, Go, license, graphs, hosted-loop Approve rewrite (that is WS1)
```
