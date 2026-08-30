# Module: `messages` (retired)

**Retired** in [ADR-032](../adr/032-retire-messages-polymorphic-lookup.md). The optional `messages` package and `Message` object are **not** in the product catalog.

Do **not** reintroduce a CRM Message object as:

- a polymorphic parent / regarding column
- an audit trail of agent or chat interactions

Agent chat transcripts persist on kernel `agent_conversations` / `agent_conversation_messages` via `/client/v1/agents/conversations` ([ADR-022](../adr/022-agent-conversations.md)). Hosted run audit is `agent_runs`. Platform mutation audit is `audit_log`. High-volume **storage** (`records_hv`) remains for a future append-heavy object (planned: `ExecutionLogEntry` on [BP-033](../../backlog/BP-033-customer-runtime-isolation.md)).

Structured CRM work items stay on optional [`activities`](./activities.md). `GET /client/v1/activity-feed` composes those work items only.

Upgraded installs that had enabled `messages` lose Message metadata and `records_hv` Message rows in kernel migration `0060`.
