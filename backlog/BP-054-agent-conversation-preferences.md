# BP-054: Agent conversation persistence + principal preferences

- **Severity:** High
- **Status:** Mitigated
- **Area:** Client `agent_conversations` / `principal_preferences`

## Outcome

Principal-scoped conversation threads and preferences ([ADR-022](../docs/adr/022-agent-conversations.md)). Not CRM Message.

## Do not reopen

IDE-only persist expansion is frozen. Chrome Client conversation island may be removed under [BP-065](./BP-065-ide-backend-coupling.md). Plan: [agent-conversation-build-plan.md](../docs/architecture/agent-conversation-build-plan.md).
