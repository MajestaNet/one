# BP-026: OSS security process & public backlog hygiene

- **Severity:** Medium
- **Status:** Partially mitigated (Dependabot + IDE `npm audit`; one-time Go vulnerability-database baseline — **not** a CI `govulncheck` job; advisory policy remains)
- **Area:** repo root (`SECURITY.md`), `backlog/`, release/docs hygiene
- **Remainder:** [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)

## Problem

Moving the CRM backend to a maintained **open-source** repo without a disclosure process and public-risk hygiene invites two failure modes: (1) vulnerability reports land in public issues before fixes exist; (2) agents and contributors treat “OSS” as “secure by default” and under-invest in AuthZ/hardening BPs.

## Why it matters

Community review can make Majesta One **more** secure when practiced (more eyes on Go AuthZ, query, and packaging). It does not replace [BP-003](./BP-003-enterprise-auth.md), [BP-006](./BP-006-agent-guardrails.md), [BP-013](./BP-013-jwt-unified-principals.md), or [BP-017](./BP-017-identity-directory-productionization.md). Attackers also see the code; misconfigured self-host installs remain the dominant operational risk. A public backlog builds trust only if vendor secrets and live vuln details stay out of it.

## What shipped

- Root [`SECURITY.md`](../SECURITY.md) — private reporting channel + expectations
- Public backlog rules documented in [README alignment](./README.md#security--transparency)
- Entire repository Apache-2.0 (including Control IDE) ([NOTICE](../NOTICE))
- Dependabot for `tools/control-ide` npm, Go modules, and GitHub Actions (`.github/dependabot.yml`)
- Control IDE CI gate: lint + `npm audit --audit-level=high` (required) + trust-boundary smoke
- Go backend dependencies refreshed to supported releases and checked once against the
  official Go vulnerability database (August 2026 production-hardening baseline). **There is no CI `govulncheck` gate.**
- Root [`CONTRIBUTING.md`](../CONTRIBUTING.md) Security section points at [`SECURITY.md`](../SECURITY.md) (one line; no secrets/PoC blurb yet)
- Vendor-plane IDE security register ([control-ide-security-audit.md](../docs/architecture/control-ide-security-audit.md)) — findings and remediation live under `docs/` / `tools/`; **do not** copy advisory detail into this public backlog

## Remaining

1. **Advisory policy** — publish after fix (GitHub Security Advisories); wire contact alias if `security@majestanet.com` is not yet live
2. Optional: security.txt
3. Contributor guidance: expand the CONTRIBUTING Security blurb (no secrets / customer data / exploit PoCs in public issues)

## Explicit non-goals (now)

- Claiming OSS alone closes AuthZ or identity backlog items
- Building a second tracking system parallel to `backlog/`
- DRM or obfuscation of the OSS backend (IDE commercial controls remain frozen — ADR-030)
- Publishing live vulnerability detail, PoCs, or unfixed advisory IDs in `backlog/`

## Related

- Remainder design: [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)
- [docs/security.md](../docs/security.md) — install AuthN/AuthZ posture
- [ADR-030](../docs/adr/030-install-agent-runtime.md) — optional IDE distribution channel is frozen
- [control-ide-security-audit.md](../docs/architecture/control-ide-security-audit.md) — IDE vendor-plane register
- Backlog [Alignment — Security & transparency](./README.md#security--transparency)
