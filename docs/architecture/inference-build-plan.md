# Customer inference (BYO + DigitalOcean native) — build plan

**Active plan** for install-local model routing that powers agentic Client runs.

**Playbooks:** [agent-api-families.md](./agent-api-families.md) · [agent-deploy.md](./agent-deploy.md) · [agent-authz.md](./agent-authz.md) · [agent-worker.md](./agent-worker.md)  
**Domain agents:** `api-families` + `deploy-ops` + `authz-security` + `worker-jobs` (Go). Control IDE Settings UI is frozen ([ADR-030](../adr/030-install-agent-runtime.md)).  
**Backlog:** [BP-052](../../backlog/BP-052-customer-inference.md) · de-risks [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) BYO LLM · enables [BP-006](../../backlog/BP-006-agent-guardrails.md) hosted loop ([hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md))  
**Related:** [ADR-010](../adr/010-customer-agentic-platform.md) · [deploy-cloud-capability-contract.md](./deploy-cloud-capability-contract.md) · [ADR-030](../adr/030-install-agent-runtime.md)

---

## Thesis

> Customers configure **where** LLM calls go (BYO OpenAI-compatible providers and/or **Native DigitalOcean** Serverless Inference). Majesta One routes agent runs through that **install-local** config (Metadata + Deploy). Control IDE Settings → Inference is an optional JWT client of those APIs, not the SoR ([ADR-030](../adr/030-install-agent-runtime.md)). Client `/agents/runs` streams tokens (SSE) — **no** separate `/inference/chat/completions` surface in v1.

```text
Settings → Inference (BYO URL+key) ──Metadata──► install_inference_providers + secrets
Settings → Inference (Native DO)    ──Deploy───► install_inference_config (mode)
Operate / agents                     ──Client──► POST /agents/runs (+ SSE stream)
                                                      │
                                                      ▼
                                              internal/inference router
                                                      │
                         ┌────────────────────────────┼────────────────────────────┐
                         ▼                            ▼                            ▼
                   DO native                    BYO default                   none → error
            inference.do-ai.run            customer base URL
            model = Dev|Std|Pro map        + secret ref
```

---

## Locked product decisions

| Decision | Choice | Rationale |
|---|---|---|
| Client surface | Extend `/client/v1/agents/runs` with SSE streaming + model routing | Native agentic UX without a second inference family (choice **1B**) |
| Separate OpenAI proxy route | **Out of scope v1** | Add later only if external SDKs need it |
| DO modes | `dev` / `standard` / `pro` → **fixed model IDs** (constants, easy to retune) | Choice **2C** — fast to test; not live catalog picking |
| Provisional DO model map (OSS / DO-hosted) | Dev=`openai-gpt-oss-20b`; Standard=`llama3.3-70b-instruct`; Pro=`openai-gpt-oss-120b` | Swap in one constants file when product picks finals |
| Active source | Install singleton `active_source`: `none` \| `digitalocean` \| `byo` | Explicit routing; enabling DO or setting BYO default flips source |
| DO auth | Install `DIGITALOCEAN_API_TOKEN` (same as Deploy cloud) against `https://inference.do-ai.run` | PAT works for Serverless Inference; scoped model access keys later |
| BYO shape | OpenAI-compatible `baseUrl` + `secretRef` + `defaultModel` | Covers OpenAI, OpenRouter, Groq, Together, Azure OpenAI-compat, etc. |
| Secrets | Reuse `install_secrets` + `secretcrypt` | No plaintext in IDE storage |
| Billing copy | IDE + Deploy status always state: inference + hosting billed by DigitalOcean; Serverless Inference is **prepaid** | Legal/product requirement |
| Hosted tool loop | **Not** in this plan | Remains BP-006; native `tools` / `tool_calls` on this client land with the loop — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md). This plan only completes LLM reply generation + stream |
| Generation vs mutation approval | Streaming create **does not** park on `awaiting_approval`; `approved: false` still gates tools. JSON approve queues a worker job; SSE approve streams in-process and must not also enqueue | Pre-LLM Approve blocked chat; mutations still go through Apply / future tool loop |
| Plane fence | Go owns API/router; any JWT client (CLI, MCP, optional IDE) consumes it | ADR-012 / ADR-030 |

---

## API contract

### Metadata — BYO providers

| Method | Path | AuthZ |
|---|---|---|
| `GET` | `/metadata/v1/inference/providers` | `metadata` |
| `POST` | `/metadata/v1/inference/providers` | `metadata` + `metadata.build` |
| `PATCH` | `/metadata/v1/inference/providers/{apiName}` | `metadata` + `metadata.build` |
| `DELETE` | `/metadata/v1/inference/providers/{apiName}` | `metadata` + `metadata.build` |
| `GET` | `/metadata/v1/inference/config` | `metadata` |
| `PATCH` | `/metadata/v1/inference/config` | `metadata` + `metadata.build` (set `activeSource=byo` + `defaultProviderApiName`) |

Provider body: `{ apiName, label, baseUrl, secretRef?, apiKey? (write-only→secret), defaultModel, active }`.  
Creating with `apiKey` upserts secret `inference.{apiName}` when `secretRef` omitted.

### Deploy — Native DigitalOcean Inference

| Method | Path | AuthZ |
|---|---|---|
| `GET` | `/deploy/v1/cloud/inference` | `deploy` |
| `PUT` | `/deploy/v1/cloud/inference` | `deploy` + admin |

`PUT` body: `{ enabled: bool, mode?: "dev"|"standard"|"pro" }`.  
When `enabled=true`, requires DO token configured; sets `active_source=digitalocean`.  
Response always includes `billingNotice` + `prepaid: true` + resolved `modelId`.

Compatibility alias: `/deploy/v1/cloud/digitalocean/inference`.

### Client — agent runs (streaming)

| Method | Path | Notes |
|---|---|---|
| `POST` | `/client/v1/agents/runs` | Body may include `stream: true`. When true (or `Accept: text/event-stream`), **generation runs immediately** even if the AgentSpec/harness requires approval (`approved: false` still recorded). SSE tokens, persist run. Non-stream + `requireApproval` + `approved: false` still returns `202 awaiting_approval`. Non-stream approved/queued path inserts `agent.run` for the worker. |
| `POST` | `/client/v1/agents/runs/{id}/approve` | JSON (default): queue `agent.run` job. SSE (`stream: true` or `Accept: text/event-stream`): continue generation in-process via the same LLM stream helper — **do not** also insert a job. |
| `GET` | `/client/v1/agents/runs/{id}/stream` | SSE replay/tail of `agent_run_events` (reconnect) |

SSE event types: `run` · `token` · `done` · `error`.

---

## Data model

Migration: `0050_inference.sql` (follows security uplift `0049_scrub_bootstrap_api_key_secrets`).

```sql
install_inference_config (
  id INT PRIMARY KEY CHECK (id = 1),  -- singleton
  active_source TEXT NOT NULL DEFAULT 'none',  -- none|digitalocean|byo
  do_enabled BOOLEAN NOT NULL DEFAULT false,
  do_mode TEXT,  -- dev|standard|pro
  default_provider_api_name TEXT REFERENCES install_inference_providers(api_name),
  updated_at TIMESTAMPTZ
);

install_inference_providers (
  api_name TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  base_url TEXT NOT NULL,
  secret_ref TEXT REFERENCES install_secrets(api_name),
  default_model TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at, updated_at
);

agent_run_events (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  seq INT NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);
```

BYO hosts must pass `egress.ValidateURL` and the install egress allowlist (same fail-closed pattern as connectors). Outbound LLM calls use `egress.NewSafeClient` (dial-time SSRF).

**Development exception:** when `APP_ENV` is not `production`, inference BYO may use `http://` to `localhost` / `127.0.0.1` / `::1` / `host.docker.internal` (local Ollama). Those hosts skip the install egress allowlist. Webhooks and other egress callers stay strict. See [local-development-mac.md](../local-development-mac.md) §6.
---

## IDE — Settings → Inference

| Cap | Gates |
|---|---|
| `ide.settings.inference` | Settings → Inference tool |

Panel sections:

1. **Active source** summary (none / DO / BYO).
2. **Native DigitalOcean** — enable toggle, Dev/Standard/Pro select, billing callout (prepaid + DO bill), deep-link to DO Inference console when host is DO.
3. **BYO providers** — list/add OpenAI-compatible URL + API key + default model; set as active BYO source.

No agent dock in Settings (same as Account/Hosting).

---

## Phases

| Phase | Owner | Deliverable |
|---|---|---|
| 0 | docs | This plan + BP-052 |
| 1 | Go | Migration + `internal/inference` (tiers, OpenAI client, router) |
| 2 | Go | Metadata provider/config routes + Deploy cloud inference |
| 3 | Go | Agent run streaming + worker LLM completion |
| 4 | authz | `ide.settings.inference` + seed |
| 5 | IDE | Inference panel + run SSE client uplift |
| 6 | docs | Caps, api-families, customer-agents, contract matrix |

---

## Explicit non-goals

- Full hosted tool-loop (BP-006 — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md))
- Live DO model catalog picker
- Electron-side LLM calls
- Multi-provider waterfall / automatic failover
- Dedicated GPU Inference provisioning
- Product billing for inference (always customer cloud account)

---

## Acceptance

- [ ] Settings → Inference tool visible with `ide.settings.inference`
- [ ] BYO provider CRUD stores key in `install_secrets`; never echoes plaintext
- [ ] Deploy `PUT .../cloud/inference` enables DO mode and returns billing notice
- [ ] `POST /agents/runs` with `stream:true` emits SSE tokens when inference configured
- [ ] Streaming create with `approved: false` does **not** return `202 awaiting_approval`
- [ ] SSE `POST .../approve` streams in-process and does not insert an `agent.run` job
- [ ] Settings → Inference **Test chat** streams tokens (or a clear inference error) without an Approve click
- [ ] Without config, runs fail with clear `INFERENCE_NOT_CONFIGURED`
- [ ] Worker non-stream path also uses the same router
- [ ] Provisional Dev/Standard/Pro model IDs live in one constants file
