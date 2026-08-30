# ADR-022: Agent IDE conversations and principal preferences

- **Status:** Accepted
- **Date:** 2026-08-02

## Context

Control IDE agent chat transcripts were session-local React state. Run mode needed durable multi-turn threads, selection-as-context for agent prompts, and principal-scoped IDE preferences — without inventing a CRM **Message** object as a chat or audit SoR.

## Decision

1. **Agent IDE chat** — kernel tables `agent_conversations` + `agent_conversation_messages`; Client `/client/v1/agents/conversations` (+ `.../messages`). Owner = authenticated principal only (same pattern as `/client/v1/canvases`).
2. **Agent runs** — `agent_runs.conversation_id` FK links execution audit to a conversation thread; run `input` may include `contextExcerpts` and `activeTool` for IDE bridge context.
3. **Principal preferences** — `principal_preferences` (principal_id + kind PK); Client `/client/v1/preferences/{kind}` for IDE settings, saved views, composer defaults — not `/sobjects/User`.
4. **CRM Message** — **retired** ([ADR-032](./032-retire-messages-polymorphic-lookup.md)). Do **not** store IDE or hosted agent transcripts as business records. Execution audit is `agent_runs`; mutation audit is `audit_log`.

## Consequences

- Control IDE hydrates and persists chat via Client conversations on connect/send.
- Customer business definitions (email templates, ranking fields) stay Metadata objects + Client CRUD.
- Hosted multi-step tool loops remain BP-006; IDE still applies `tool.*` bridge effects from run output.

## Related

- [BP-054](../backlog/BP-054-agent-conversation-preferences.md)
- [customer-ide-ux.md](../customer-ide-ux.md)
- [ADR-021](./021-run-mode-toolspec.md)
- [ADR-032](./032-retire-messages-polymorphic-lookup.md)
