# Install claim, customer SSO, and JIT — build plan

Executable plan for day-0 **install claim** (email + local password, no IDE required), **customer-configurable SSO**, demoted Google/Apple, and **JIT provisioning**.

**ADR:** [ADR-015](../adr/015-idp-agnostic-social-login.md) (amended)  
**Backlog:** [BP-037](../../backlog/BP-037-install-claim-customer-sso.md), [BP-013](../../backlog/BP-013-jwt-unified-principals.md)  
**Playbooks:** [agent-authz.md](./agent-authz.md), [agent-api-families.md](./agent-api-families.md), [agent-control-ide.md](./agent-control-ide.md), [agent-deploy.md](./agent-deploy.md)

## Locked decisions

| Topic | Choice |
|---|---|
| Day-0 claim | One-time `INSTALL_CLAIM_TOKEN` → first human `SystemAdmin` with email + **password** |
| IDE optional | Claim via `POST /auth/v1/install/claim` (curl) or Control IDE Connect |
| Login primary | Customer-configured SSO when set; password when enabled; Google/Apple only if customer enables |
| JIT | Customer toggle on Metadata install auth (`jitProvisionUsers`) |
| Bootstrap keys | `API_KEYS` remain break-glass only |

## APIs

| Method | Path | Notes |
|---|---|---|
| `GET` | `/auth/v1/install/status` | Public: claimed, SSO, password, social |
| `POST` | `/auth/v1/install/claim` | Public + rate limit; returns Majesta One JWT |
| `POST` | `/auth/v1/token` | `grant_type=password` when password login enabled |
| `GET`/`PUT` | `/metadata/v1/install/auth` | SSO / JIT / social / password (`identity.manage`) |

## Schema

`organization_settings` columns for claim hash, claimed_at, OIDC fields, JIT, social_providers, password_login_enabled.  
`principal_credentials.credential_kind` includes `password`.

## Phases

0. Docs + ADR-015 amend + BP-037 — this change set  
1. Claim + password grant — shipped in same PR  
2. Customer SSO + JIT + login page IdP-primary — shipped  
3. Control IDE claim + Govern SSO panel — shipped  
4. Deploy secrets / NOTES / curl runbook — shipped  

## Non-goals

Email OTP; embedded Keycloak; cross-install SSO; removing `API_KEYS`; requiring Google/Apple for App Platform.
