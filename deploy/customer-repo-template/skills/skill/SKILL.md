---
name: one-skill
description: Invoke named automations granted on an AgentSpec. Use when running a skill via invoke_skill on MCP or a hosted agent run.
---

# Skill

Job class: `skill`. Invoke only named automations in the AgentSpec `allowedSkills` list, and only when the principal's permission sets grant `canRun`. Do not use Metadata or Deploy from this class.

## Do

- Call MCP `invoke_skill` with `apiName` (the automation api name). Agent principals must also pass `playbookApiName`.
- Keep grants in `allowedSkills`. Execution still requires permission-set `canRun` (or admin / `allAutomations`).
- Hosted `/client/v1/agents/runs` may execute `invoke_skill` when `skills.invoke` is admitted. Writes park at `awaiting_tool_approval` when `requireApproval` is true.

## Do not

- Invent automations that are not in `allowedSkills`. Empty `allowedSkills` is not a grant — invoke fails closed until names are listed.
- Call Metadata upserts or Deploy `org_*` from this job class — those stay on builder MCP / family HTTP.
- Treat Control IDE docks as where agents live — harnesses bind to job class.
