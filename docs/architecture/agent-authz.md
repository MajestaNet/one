# Agent playbook: AuthZ

For agents changing Majesta One AuthN/AuthZ: JWT, API keys, Roles, scopes, permission sets, or principals. Follow this before writing code.

## Where to look

| Concern | Path |
|---|---|
| Decisions | [`docs/adr/006-jwt-auth.md`](../adr/006-jwt-auth.md), [`docs/adr/015-idp-agnostic-social-login.md`](../adr/015-idp-agnostic-social-login.md), [`docs/adr/009-record-audit-authz-packaging.md`](../adr/009-record-audit-authz-packaging.md) |
| Install claim / customer SSO / JIT | [install-claim-sso-build-plan.md](./install-claim-sso-build-plan.md), `internal/db/install_auth.go`, `install_claim_routes.go` |
| No product mailer / password set / system intents | [system-alerts-byo-build-plan.md](./system-alerts-byo-build-plan.md), [BP-038](../../backlog/BP-038-no-product-mailer-byo-alerts.md), `internal/db/outbox.go`, `credentials.go` |
| Scopes / API keys | `internal/authz/scopes.go`, `internal/authz/apikey.go` |
| Majesta One JWT | `internal/authz/jwt.go` |
| Refresh tokens | [refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) · `internal/authz/refresh_token.go` + `internal/httpapi/auth_routes.go` + `migrations/0058_refresh_tokens.sql` |
| Social login broker | `internal/authlogin` (Google/Apple); [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md) |
| Identity write-through | `internal/identity` (optional Cognito / memory) |
| Object / field perms | `internal/authz/object_perms.go`, `internal/db/object_perms.go`, `internal/db/data_access.go`, `internal/db/authz_adapter.go` |
| SCIM adapter | `internal/scim/`, `internal/httpapi/scim_routes.go`; [scim-provisioning.md](./scim-provisioning.md) · [user-identity-extension-build-plan.md](./user-identity-extension-build-plan.md) (custom attrs) |
| Principals / users | `internal/authz/users.go`, `internal/db/users.go`, `internal/db/credentials.go` |
| Transitional OIDC | `internal/authz/oidc.go` |
| HTTP Token Service | `internal/httpapi/auth_routes.go` |
| Security overview | [`docs/security.md`](../security.md) |
| Scale backlog | [`backlog/BP-013`](../../backlog/BP-013-jwt-unified-principals.md), [`BP-063`](../../backlog/BP-063-refresh-token-sessions.md) (refresh sessions), [`BP-017`](../../backlog/BP-017-identity-directory-productionization.md), [`BP-003`](../../backlog/BP-003-enterprise-auth.md) (mitigated), [`BP-058`](../../backlog/BP-058-user-identity-extension.md) (mitigated), [`BP-006`](../../backlog/BP-006-agent-guardrails.md), [`BP-038`](../../backlog/BP-038-no-product-mailer-byo-alerts.md) |
| Identity directory plan | [`identity-directory-productionization.md`](./identity-directory-productionization.md) |
| User identity extension | [`user-identity-extension-build-plan.md`](./user-identity-extension-build-plan.md) |
| IdP-agnostic login plan | [`idp-agnostic-login-build-plan.md`](./idp-agnostic-login-build-plan.md) |
| Customization AuthZ | [`customization-authz.md`](./customization-authz.md) |

## What ships today

```text
Principals: users row with principal_type user | service | agent
Roles → API family scopes (client / metadata / deploy / ops) + optional admin
Permission sets → object/field access; assigned to users only (not via roles)
AuthN paths: API_KEYS (env break-glass), install claim + password grant, Majesta One JWT, customer SSO (DB install auth), optional social broker (Google/Apple), OIDC exchange
Directory: Client principal admin + /scim/v2; JIT via install auth `jitProvisionUsers`
User object: kernel users (not records); customer fields + provisioning → user-identity-extension-build-plan.md
Social humans: email **required**; AuthN key = `identity_links`; Google/Apple **optional** (customer enable)
```

## What to do (change types)

### A. Change **scope parsing** or key format

1. Edit `internal/authz/scopes.go` / `apikey.go` — no substring matching for scopes.
2. Keys may be `name` (all scopes) or `name:client+metadata+deploy+ops`; add `+admin` for admin privilege.
3. Add/adjust unit tests in `scopes_test.go`; run `go test ./internal/authz/...`.

### B. Majesta One JWT / Token Service / social login

1. Follow ADR-006 + **ADR-015** / [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md); do not reintroduce Cognito as the product GA default.
2. Mint/verify paths live in `jwt.go` + `auth_routes.go`; social broker in `internal/authlogin`.
3. Every caller remains a `users` row with ≥1 Role → scopes.
4. Principal/credential admin: `principal_routes.go` under **Client** (`identity.users` / `identity.integrations`); Role/PS **assignment** is `authz.manage`.
4b. Integration configs (Connected Apps): `integration_routes.go` + `internal/integration` — OAuth client shapes, optional IdP write-through, encrypted secret reveal; optional `allowedCidrs`.
5. Permission-set **definitions**: Metadata (`authz.manage` on create/patch; supports `systemPermissionsAdd`/`Remove`).
6. Optional Cognito write-through: `internal/identity` + `identity_links` when `IDENTITY_SYNC=cognito`.
7. Token exchange: external IdP ID token → Majesta One JWT (`auth_routes.go`) with `azp`; social PKCE uses authorize/callback + `authorization_code`.
8. Refresh tokens (BP-063): follow [refresh-token-session-build-plan.md](./refresh-token-session-build-plan.md) — opaque hashed RTs, rotation, `/auth/v1/token` `grant_type=refresh_token`; do not lengthen access JWT TTL. Control IDE consumption is a `control-ide` follow-up in the same plan.
9. Install exposure / WAF: `exposure_routes.go` + `internal/edge` (`govern.network`); device enroll under `/client/v1/devices`.
10. Update BP-013 / BP-022 / BP-063 status when a phase lands.

### C. Object / field AuthZ (BP-003 remainders)

1. Enforce via permission sets assigned to the user; Roles grant scopes only (ADR-009).
2. Prefer extending `object_perms.go` / `data_access.go` + DB adapter rather than ad-hoc checks in handlers.
3. Data-access catalog: `EnsureObjectInDataAccessCatalog` / `EnsureFieldInDataAccessCatalog` on metadata insert; Metadata `dataAccess` section on GET/PATCH.
4. Field FLS is **deny-by-default** (OR-union across assigned PSs). See [authz-ide-fls-build-plan.md](./authz-ide-fls-build-plan.md).
5. Control IDE chrome: `ide.*` system capabilities (mode + tool); fail-closed after `/me` is an **IDE client** contract — Go HTTP does not `requireCapability` on `ide.*`. Phase 1 AuthN neutrality (generic `azp=one.install`, no Control IDE refresh shortcut, `ide_users` removed) is in [ide-backend-coupling-review.md](./ide-backend-coupling-review.md) / [BP-065](../../backlog/BP-065-ide-backend-coupling.md). Do not add `ide.*` as family-route gates.
6. Metadata / Deploy system capabilities: see [customization-authz.md](./customization-authz.md) and `system_perms.go` — replace blanket admin on Metadata writes.

### C2. Record sharing (ADR-016)

1. Read [record-sharing.md](./record-sharing.md) + ADR-016 before changing visibility.
2. API scope `roles` ≠ `data_roles`; never use `roles.parent_role_id` for sharing. The column is unused (Keep); sharing walks `data_roles` only.
3. Extend `internal/authz/sharing.go` and `RecordAccessEvaluator`; wire through Client GET/query/PATCH/DELETE and MCP/composite.
4. Enqueue `sharing.recalc` after rule/OWD/data-role mutations; grants live in `record_access_grants`.
5. When `recordSharingEnabled=false`, legacy `CanViewRecord` / `CanModifyRecord` behavior must be unchanged.

### D. Agent principals (feeds BP-006)

1. Register agents as `principal_type = agent` with Roles + permission sets — not a shared `DEFAULT_OWNER_ID` key. Agents share the same customer-customization AuthZ path as `user` / `service` (BP-006); do not authorize by type alone.
2. Agent **runs** stay on Client API (`client_extras.go`); playbook **definitions** stay Metadata.
3. Coordinate with [agent-api-families.md](./agent-api-families.md) when touching HTTP. Hosted tool execution reconstructs the run Actor and calls `mcp.CallTool` — [hosted-agent-tool-loop-build-plan.md](./hosted-agent-tool-loop-build-plan.md). Enforce playbook tool/object allowlists in the loop (worker + SSE share it). Never fall back to `DEFAULT_OWNER_ID`.

### E. Kernel schema for AuthZ

1. Add numbered SQL under `migrations/` + journal entry.
2. Update `internal/db` stores; keep forward-compatible for existing installs.

## Explicit non-goals (until docs say otherwise)

- Multi-tenant SaaS `tenant_id` on AuthZ rows (ADR-001)
- Cognito (or any IdP) as the long-term Majesta One **AuthZ** system (groups as SoR)
- Embedded Keycloak / full in-process OAuth authorization server (ADR-015)
- Email OTP / magic-link login
- Cognito ALB authenticate for machine traffic
- Three Cognito User Pools (one per principal type)
- Assigning permission sets through Roles (`role_permission_sets` removed)
- Putting AuthZ policy documents inside product images beyond the Go binary
- Requiring email on Google/Apple-provisioned humans (SCIM create may still require email)

## Checklist before merging an AuthZ PR

- [ ] Read ADR-006 / ADR-015 / ADR-009 + this playbook
- [ ] Scope matching remains exact (no substring)
- [ ] Principals stay `user` | `service` | `agent` on `users`
- [ ] Social/login work follows [idp-agnostic-login-build-plan.md](./idp-agnostic-login-build-plan.md) (no Cognito-as-default, no embedded Keycloak)
- [ ] Tests cover the AuthN path touched
- [ ] BP-013 / BP-003 / BP-058 updated if risk materially changed
