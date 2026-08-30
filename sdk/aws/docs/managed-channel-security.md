> **Not product GA.** Community-maintained under [`sdk/aws`](../README.md) — optional Path B extension only. Preferred install: [docs/self-host.md](../../../docs/self-host.md) (Path A App Platform / Path B Compose+Helm).

# Managed channel — isolation threat model

Threat model and **proof points** for the Majesta One **managed subscription** channel (vendor-operated regional AWS accounts). Complements [managed-channel.md](./managed-channel.md), [security.md](../../../docs/security.md), [ADR-001](../../../docs/adr/001-dedicated-install.md), and [ADR-006](../../../docs/adr/006-jwt-auth.md).

This is operational architecture guidance, not a certification.

## Threat

**Horizontal traversal:** a compromised human user, Cognito session, Majesta One JWT, or app-client secret belonging to **Customer A** must not grant access to **Customer B** (another commercial customer) in the same vendor regional account.

In-org privilege escalation inside Customer A (Roles / permission sets) is out of scope here — that is install-local AuthZ ([ADR-009](../../../docs/adr/009-record-audit-authz-packaging.md)).

## Isolation unit

| Layer | Boundary |
|---|---|
| Commercial customer | One or more installs sharing `CUSTOMER_ID` |
| Install / environment | Unique `INSTALL_ID` → dedicated ECS+RDS+secrets+Cognito stack |
| Identity directory | **One Cognito User Pool per install** (humans + BYO IdP) |
| Machine principals | Cognito **app clients inside that pool** (not a shared pool) |
| API bearer | Install-local Majesta One JWT (`AUTH_JWT_SIGNING_KEY`) |

Account ownership (Marketplace subscriber account vs vendor managed account) does **not** change the isolation unit.

## Attack paths and controls

| # | Attack path | Control | Proof |
|---|---|---|---|
| 1 | Steal Cognito ID token from A → call B `/auth/v1/token/exchange` | B’s OIDC verifier requires `iss` = Pool-B and `aud` = B’s UI client | Unit: foreign issuer/audience rejected (`internal/authz` isolation tests); HTTP exchange returns 401 |
| 2 | Steal Majesta One JWT from A → call B family routes | B verifies with a **different** `AUTH_JWT_SIGNING_KEY`; install-local `iss` | Unit: signer B rejects token minted by signer A |
| 3 | Steal Cognito app-client secret from A | Client exists only in Pool-A; B has no `identity_links` for that client | Pool-scoped write-through IAM (`Resource` = Pool-A ARN only) |
| 4 | Reuse Hosted UI session cookie across customers | Separate user pools → no shared Cognito session cookie | Cognito multi-tenant guidance; pool-per-install provisioning |
| 5 | SQL / API “switch customer” | No SaaS `tenant_id` isolation column on business rows; process only has `DATABASE_URL` for its RDS | ADR-001; task env wiring |
| 6 | Network pivot A → B (RDS / ECS) | Separate VPC + SGs; no peering by default | TF topology (`sdk/aws/deploy/ecs/network.tf`); managed overlay forbids peer routes |
| 7 | IAM abuse from A’s task role against Pool-B / Secret-B | Cognito + Secrets Manager policies scoped to **this install’s** ARNs | `cognito.tf` / `ecs.tf` resource ARNs |
| 8 | Deploy promote A → B (different commercial customer) | No install→install promote API; repo→org apply is install-local AuthZ | Peer push / inbound artifact promote removed |


## Blast radius of a compromised A user

**In scope for the attacker:** Cognito user in Pool-A, Majesta One principal in RDS-A, data visible under A’s permission sets, secrets injectable only into A’s tasks.

**Out of scope:** Pool-B users, RDS-B data, B’s JWT signing key, B’s network, B’s Secrets Manager secret.

## Vendor ops trust boundary (separate)

Fleet operators and automation in the vendor regional account are a **different** principal class from product task roles:

| Role | Intent | Forbidden by default |
|---|---|---|
| Product ECS task roles | Run install API/worker | Cross-install Cognito/Secrets ARNs |
| `OneManagedFleetOps` | Describe/inventory + start install-local upgrade Automation | `secretsmanager:GetSecretValue` on install secrets |
| `OneManagedBreakglass` | MFA break-glass secret access | Broad `Resource: *` without tag conditions |

Fleet orchestration stays **outside** `cmd/api` ([BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md)).

**CI / release fence:** product `release.yml` must not hold managed-prod credentials or auto-roll managed cells. Marketplace and managed consume the same published digests via separate, approved promotions ([release-cicd.md](../../../docs/release-cicd.md#channel-promotion-marketplace--managed), [managed-channel.md](./managed-channel.md#release-vs-roll-promotion-fence)).

## Automated proofs in-repo

| Test | What it asserts |
|---|---|
| `TestCrossInstallOneJWTRejected` | JWT minted for install A fails verify on install B’s signer |
| `TestCrossInstallOIDCIssuerRejected` | Cognito-shaped ID token for Pool-A fails B’s verifier |
| `TestCrossInstallOIDCAudienceRejected` | Valid issuer but wrong app-client `aud` fails |
| `TestAuthV1TokenExchangeRejectsForeignIssuer` | HTTP exchange on B rejects A’s ID token |
| ~~`TestAssertInboundTrustCustomerMismatch`~~ | Removed with inbound artifact promote |

Live AWS network/IAM denials (VPC-A ↛ RDS-B, task-A ↛ Pool-B) are validated at provision time via the managed checklist — not inside product unit tests.

## Related

- [managed-channel.md](./managed-channel.md) — provisioning, Cognito quotas, cells
- [security.md](../../../docs/security.md) — install AuthN/AuthZ posture
- [aws-fargate.md](./aws-fargate.md) — ECS reference
- [sdk/aws/deploy/managed/](../deploy/managed/) — fleet IAM + quota alarms
