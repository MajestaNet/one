# BP-052: Customer inference (BYO providers + DigitalOcean native + agent run streaming)

- **Severity:** High
- **Status:** Partially mitigated (Phases 0–5 plus Operate SSE token rendering; **streaming create skips pre-LLM `awaiting_approval`**; **SSE approve continues generation without a worker job**; **Settings → Inference Test chat** probes the same SSE path; **local Ollama / `APP_ENV=development` BYO loopback** supported; **DO model IDs + `modelId` alias landed**. Remainders: SSE reconnect/cancel polish. Hosted tool loop is [BP-006](./BP-006-agent-guardrails.md).)
- **Track:** Finish install inference SoR. Settings UI frozen ([ADR-030](../docs/adr/030-install-agent-runtime.md)). Hosted tool loop stays [BP-006](./BP-006-agent-guardrails.md).
- **Area:** `internal/inference`, `internal/httpapi` (Client agent runs + Metadata inference + Deploy cloud inference), `internal/worker`, `internal/authz`, `migrations/`, `tools/control-ide` (Settings → Inference)
- **Design:** [inference-build-plan.md](../docs/architecture/inference-build-plan.md) · **Finish remainder:** [04-bp-052-customer-inference.md](../docs/architecture/agentic-remainders/04-bp-052-customer-inference.md) (SSE reconnect/cancel, API/docs; hosted loop stays BP-006; Settings UI frozen)
- **Related:** [BP-014](./BP-014-agent-outbound-integrations.md) (BYO LLM deferred → this item) · [BP-006](./BP-006-agent-guardrails.md) (hosted tool loop still separate) · [BP-030](./BP-030-deploy-api-digitalocean-apps.md) · [BP-051](../docs/adr/030-install-agent-runtime.md) · [ADR-010](../docs/adr/010-customer-agentic-platform.md)

## Problem

Agentic Operate / agent runs cannot call a model. BYO LLM keys and DigitalOcean Serverless Inference were documented as a future “managed inference” contract but never implemented. Without install-local routing + streaming, customers cannot deliver native agentic chat experiences.

## Why it matters

Cutting-edge agent UX needs streaming tokens and a clear path to either BYO OpenAI-compatible APIs or Path A DigitalOcean Inference (prepaid, billed on the customer DO account).

## Plan (locked)

1. Settings → **Inference** tool for BYO URL + credentials (+ DO native controls)
2. Deploy `/deploy/v1/cloud/inference` to enable Native DO Inference with Dev / Standard / Pro (fixed OSS model IDs for easy retune)
3. Billing disclosure: DO hosts + bills inference; Serverless Inference is prepaid
4. Client `/client/v1/agents/runs` routes through install config (no separate inference family in v1)
5. SSE streaming on agent runs

## Acceptance

See [inference-build-plan.md](../docs/architecture/inference-build-plan.md) acceptance checklist.

Control IDE consumes the run SSE stream directly, updates the assistant response as tokens arrive, handles approval-shaped JSON fallbacks, and renders safe rich Markdown without adding a second chat runtime.

## Explicit non-goals

- Completing BP-006 tool loop
- Catalog-driven DO model picking
- Product-side inference billing

## Related

- [deploy-cloud-capability-contract.md](../docs/architecture/deploy-cloud-capability-contract.md) — managed inference row
- [ADR-030](../docs/adr/030-install-agent-runtime.md)
- [customer-agents.md](../docs/customer-agents.md)
