# BP-065: Neutralize Control IDE coupling on the Go install

- **Severity:** Medium
- **Status:** Partially mitigated (Phase 1 AuthN neutrality landed; Phases 2–4 remain)
- **Track:** Finish (install / builder) — **cross-plane**: refactor Control IDE when that cleans the backend
- **Area:** `internal/authz`, `internal/httpapi` (token/claim + chrome Client routes), `internal/inference`, `internal/agentharness`, `internal/seed`, `internal/integration`, `tools/control-ide/**` (lockstep only)
- **Design:** [ide-backend-coupling-review.md](../docs/architecture/ide-backend-coupling-review.md)
- **Remainder plan:** [01-bp-065-ide-backend-coupling.md](../docs/architecture/agentic-remainders/01-bp-065-ide-backend-coupling.md)
- **Related:** [ADR-030](../docs/adr/030-install-agent-runtime.md) (§5: IDE is optional and **refactorable** for install cleanup) · [BP-064](./BP-064-install-agent-runtime.md) · [BP-006](./BP-006-agent-guardrails.md) · [BP-022](./BP-022-client-access-ide-device.md) · [BP-053](./BP-053-agent-section-harness.md) · [BP-054](./BP-054-agent-conversation-preferences.md) · [BP-062](../docs/adr/030-install-agent-runtime.md) · [BP-063](./BP-063-refresh-token-sessions.md)

## Problem

ADR-030 makes the Go install the product and Control IDE an optional JWT client. The commercial families are already ungated on `ide.*` (no HTTP handler requires those caps). Remaining coupling still teaches the install that “the client is Control IDE”:

1. Password grant with empty `client_id`, install claim, and token-exchange fallback mint `azp=one.controlIde`. That azp always gets a refresh token (no `offline_access`).
2. `seed.Seed` always runs `EnsureControlIDE` (managed PKCE app + `one-control://` callbacks + `control-ide@one.local`).
3. `clientAccessMode=ide_users` would block MCP, CLI, and Client Experiences if an operator enabled it as hardening.
4. Hosted generation injects Control IDE `oneEffects` / `graphCalls` coaching; BP-053 section preambles and starter `RunCoach` still speak Electron Apply.
5. Kernel Client routes exist only for IDE chrome: run-graphs, conversations, preferences, principal canvas working-sets.

Control IDE is **not frozen** against this work. Do not leave those surfaces in the kernel “because the IDE still calls them.”

## Why it matters

Builders and Experiences must not inherit Electron azp, refresh privileges, or graph coaching. A permanent chrome island is worse than a lockstep IDE refactor that lets us delete the routes, caps, and coaching.

## Direction (locked)

Follow [ide-backend-coupling-review.md](../docs/architecture/ide-backend-coupling-review.md). **Prefer changing `tools/control-ide` when that yields a cleaner install.**

| Phase | Outcome |
|---|---|
| 0 | Done in the review change set — inventory + this BP + ADR-030 §5 |
| 1 | AuthN neutrality: generic azp defaults; no Control IDE refresh special-case; remove `ide_users`; IDE always sends `client_id=one.controlIde`; `EnsureControlIDE` optional |
| 2 | Coaching: drop auto `oneEffects` inject; rewrite `RunCoach` + section preambles; IDE Apply becomes client-local or Client-API-only; then drop `graphCalls` persist |
| 3 | **Remove** chrome Client routes after IDE uses local state (or drops the feature); later migrate drop tables |
| 4 | Drop `ide.*` from seed/catalog; IDE tiles gate on Role scopes + product caps |

**Do not:** `requireCapability(CapIDE*)` on family routes; implement BP-062 license JWS as an install gate; add `graph.*` to MCP; add new Electron-only product chrome.

## Explicit non-goals

- Keeping chrome routes indefinitely for an in-repo IDE we can update
- A fourth API family
- Breaking `primarySection` YAML (alias stays until jobClass-only is a later migrate)
- ToolSpec Metadata/Client catalog removal (dual-purpose product)
- In-IDE coding-agent host; Operate as end-user CRM

## Implementation agent prompt

Phase 1 AuthN neutrality is **in tree**. Next work is **Phase 2** (coaching + IDE Apply lockstep). Paste the Phase 2 prompt from [01-bp-065-ide-backend-coupling.md](../docs/architecture/agentic-remainders/01-bp-065-ide-backend-coupling.md) §5. Do not re-implement generic azp, `ide_users` rejection, or `SEED_CONTROL_IDE`.