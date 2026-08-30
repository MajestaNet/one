# Agent conversation + preferences build plan

**ADR:** [022](../adr/022-agent-conversations.md) · **BP:** [BP-054](../../backlog/BP-054-agent-conversation-preferences.md)

## IDE (Control)

- `ContextExcerpt` — drag or “Add to chat” from Run tables / Object Home; pending chips in composer; human bubbles with excerpt cards.
- Run sends include `input.contextExcerpts`, `input.activeTool` when a Tool tile is open, and `input.conversation` (prior user/assistant turns) so the model sees the thread, not only the latest `goal`.
- Chat hydrates from `GET /client/v1/agents/conversations/{id}` on connect; append on send via `POST .../messages`.

## Client API

| Route | Purpose |
|---|---|
| `GET/POST /client/v1/agents/conversations` | List / create principal-owned threads |
| `GET/PATCH /client/v1/agents/conversations/{id}` | Load / rename |
| `POST /client/v1/agents/conversations/{id}/messages` | Append transcript rows |
| `GET/PUT/DELETE /client/v1/preferences/{kind}` | Principal JSON documents (`ide.settings`, `savedViews`, …) |

`POST /client/v1/agents/runs` accepts optional `conversationId`.

## vs CRM Message

Optional `messages` module = channel/timeline on Account/Case/… — **not** IDE agent chat.
