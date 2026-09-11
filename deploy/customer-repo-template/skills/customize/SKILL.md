---
name: one-customize
description: Shape customer-owned Majesta One metadata. Use when creating or updating objects and fields via MCP upsert_object / upsert_field, toggling automations via patch_automation, or Metadata HTTP.
---

# Customize

Job class: `customize`. Never mutate `ownership=managed` package definitions except managed automation `PATCH { "active" }` (admin + `metadata.build`). Prefer dry-run and approval for writes.

## Tools

MCP (requires `metadata` scope + `metadata.build`): `list_objects_metadata`, `get_object_metadata`, `upsert_object`, `upsert_field`, `patch_automation`.

Edit YAML under `metadata/` in this repo, then Ship with `one org validate` / `org deploy`. Pack only customer-owned artifacts.

Hosted `/agents/runs` does **not** execute Metadata upserts in v1 — use MCP or Metadata HTTP.
