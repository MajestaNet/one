# Security policy

Majesta One is in **alpha (`0.1.0`)**. Breaking changes are still expected. Report issues against the current tree.

## Supported versions

Security fixes target the latest released Majesta One product `v*` tag and, for Control IDE compatibility, the latest **N = 2** backend minor versions ([BP-025](backlog/BP-025-ide-api-version-compatibility.md)).

Self-host operators are responsible for applying image upgrades promptly ([docs/self-host.md](docs/self-host.md), [docs/product-upgrades.md](docs/product-upgrades.md)).

## Reporting a vulnerability

**Do not** open a public GitHub issue for security vulnerabilities.

Email **security@majestanet.com** (or the address listed in the GitHub Security Advisories contact for this repository) with:

- Affected component (`cmd/api`, `cmd/worker`, Helm chart, Control IDE, etc.)
- Product version / image digest if known
- Description and reproduction steps
- Impact assessment if you have one

We aim to acknowledge reports within **5 business days** and to publish a fix or mitigation advisory after a patch is available. Do not include customer data or exploit PoCs that are unnecessary to understand the issue.

## Public backlog

Product risk tracking lives in [`backlog/`](backlog/). That tree is intended as a public product risk list; it must not contain live vulnerability detail, secrets, or customer data ([BP-026](backlog/BP-026-oss-security-public-backlog.md)).

## Scope notes

- Open source does **not** by itself close AuthZ / identity backlog items ([BP-003](backlog/BP-003-enterprise-auth.md), [BP-006](backlog/BP-006-agent-guardrails.md), [BP-013](backlog/BP-013-jwt-unified-principals.md), [BP-017](backlog/BP-017-identity-directory-productionization.md)).
- Control IDE is included in the same Apache-2.0 repository; report IDE vulnerabilities through the same private channel.
