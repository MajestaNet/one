---
name: one-govern
description: Identity, permission sets, and install policy on a Majesta One install. Use when creating principals, assigning Roles, or checking AgentSpec harness floors.
---

# Govern

Job class: `govern`. Prefer read before write. High-risk identity or AuthZ changes require human approval.

## Do

- Create `user` / `service` / `agent` principals via Client identity admin; assign Roles (API scopes) and permission sets.
- Keep AgentSpec harness floors. Customers may widen `allowedTools` / `allowedSkills` within AuthZ; they may not PATCH the floor away.
- `invoke_skill` still needs AgentSpec `allowedSkills` (for agent principals) **and** permission-set `canRun`.

## Do not

- Echo Hosting secrets, API keys, or tokens.
- Drop below a job-class tool floor.
- Treat Control IDE docks as where agents live — harnesses bind to job class.
