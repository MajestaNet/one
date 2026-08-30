---
name: one-query
description: Query and describe business data on a Majesta One install. Use when listing objects, running query/search, or reading records via MCP or Client HTTP.
---

# Query

Job class: `query` (harness floor: `sobjects.read`, `query`, `search`). Writes stay off unless the customer widens the allowlist.

## Tools

MCP: `describe_global`, `describe_object`, `get_record`, `query`, `search`.

Prefer describe + query before proposing any write. Stay within the caller's object/field grants. Do not invent schema.

Hosted `/client/v1/agents/runs` may execute these read tools as the run actor. Dry-run plans tools but does not execute them.
