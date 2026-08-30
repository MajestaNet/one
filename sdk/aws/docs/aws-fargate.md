> **Not product GA.** Community-maintained under [`sdk/aws`](../README.md) — optional Path B **power** path only. Preferred install: [docs/self-host.md](../../../docs/self-host.md) (Path A App Platform / Path B Compose+Helm). For an AWS **managed PaaS** analog to App Platform, see [managed-paas-profile.md](./managed-paas-profile.md) — do **not** equate Fargate with App Platform.

# AWS Fargate reference architecture

Community **AWS ECS Fargate** reference for Majesta One: dedicated install **ECS Fargate** across **two AZs**, **ALB** edge, **Majesta One JWT** AuthN (ADR-006 / BP-013), with opaque API keys as bootstrap only. AMI/EC2 is explicitly out of scope. This is the **power / network-control** AWS shape (closer to DOKS+Helm than to App Platform).

Cognito User Pool Terraform may still exist in the reference stack as a **transitional** option; it is **not** the product AuthN default going forward.

## Components

| Component | Role |
|---|---|
| Application Load Balancer | TLS termination; routes to API target group; **WAFv2 WebACL** (path-family exposure) — **no** Cognito authenticate action required |
| ECS service `one-api` | Go API (`/client/v1`, `/metadata/v1`, `/deploy/v1`, `/auth/v1`); desired count ≥ 2, spread across AZs |
| ECS service `one-worker` | Go jobs / outbox / automations (`FOR UPDATE SKIP LOCKED`) |
| RDS Postgres 16 Multi-AZ | One DB per install (users, roles, permission sets, credentials) |
| Secrets Manager | `DATABASE_URL`, bootstrap `API_KEYS`, `AUTH_JWT_SIGNING_KEY`, `DEPLOY_SHARE_SECRET`, install identity, `OIDC_*` |
| CloudWatch / OTEL | Logs + optional `OTEL_EXPORTER_OTLP_ENDPOINT` |
| Cognito | MFA (software token) on; public UI client SRP + auth code only |
| IAM | Separate API vs worker task roles (Cognito/WAF/Ops on API only) |

Terraform lives in [`sdk/aws/deploy/ecs/`](../deploy/ecs/). Default WAF exposure: Client/Auth public; Metadata/Deploy/Ops blocked. TLS (`certificate_arn`) required unless `allow_http=true`.

## Auth split (target — ADR-006)

| Traffic | Edge (ALB) | Application |
|---|---|---|
| Humans, agents, services | TLS (+ optional WAF) | Majesta One JWT; humans via Cognito login → `/auth/v1/token/exchange`; machines via Majesta One client credentials |
| Bootstrap / break-glass | TLS | API keys bound to a service principal |
| Cognito User Pool | Hosted UI / passwordless-oriented + BYO IdP | One pool per install; app clients for service/agent (write-through from Client identity admin) |
| Install exposure | TLS + WAFv2 | Desired state via `PUT /metadata/v1/install/exposure` |
| Health probes | Allow `/healthz` (and `/readyz`) unauthenticated | — |

Family scopes (`client` / `metadata` / `deploy`) are enforced in the API for every principal. Admin does **not** bypass missing scopes. Effective object/field AuthZ is loaded from the DB by `sub`.

### Transitional OIDC (deprecated as product default)

Until BP-013 P1, installs may still set `OIDC_*` and verify Cognito-compatible JWTs. Prefer migrating to Majesta One JWT. Do **not** put machine or Deploy traffic behind Cognito-only ALB rules.

Legacy claim map (transitional only):

| Claim / group | Effect |
|---|---|
| `sub` | Stable `users.oidc_sub`; auto-provision when enabled |
| `email` / `name` | User profile fields |
| `one_scopes` | Space/`+`-separated scopes (`client`, `metadata`, `deploy`) |
| Cognito groups `one-client`, `one-metadata`, `one-deploy` | Same scopes when `one_scopes` absent |
| Cognito group `one-admin` | `isAdmin` |

## Multi-replica safety

- API replicas share metadata cache epoch in Postgres (BP-004 mitigated).
- Workers use lease + `SKIP LOCKED` claims (BP-005 mitigated).
- Set unique `WORKER_ID` per task (ECS task ARN / metadata).

## Product vs customer upgrades

- **Product** upgrades: new container image tags + ECS rolling deploy with circuit-breaker rollback; confirm via SSM Automation or `/ops/v1/upgrades` ([product-upgrades.md](../../../docs/product-upgrades.md), [ADR-007](../../../docs/adr/007-platform-ops-upgrades.md)). Multi-AZ enables zero-downtime **task** replace — not AZ-staged canaries (one shared RDS).
- **Customer** metadata/tests: Deployment API between same-`CUSTOMER_ID` installs ([multi-env-deploy.md](./multi-env-deploy.md)).

## Managed subscription (future channel)

Same ECS Fargate reference stack; different account owner.

| | Marketplace / self-host | Managed subscription |
|---|---|---|
| AWS account | Subscriber (or customer) account | Vendor account (~one per offered region; optional **cells**) |
| Isolation | One VPC+RDS+ECS+Cognito stack per install | Same — one isolated stack per customer env inside the regional account |
| Images | Marketplace/ECR fulfillment into subscriber account | Same `PRODUCT_VERSION` tags; vendor pulls/pushes into regional ECR |
| Upgrade | Install-local SSM / `/ops/v1` image roll | Same playbook, **orchestrated by vendor** across the regional fleet (BP-002); no Marketplace re-subscribe |
| Fleet overlay | N/A | [`sdk/aws/deploy/managed/`](../deploy/managed/) — FleetOps IAM fence + Cognito pool quota alarms |

Provisioning assumes the target region is known before apply (`aws_region` + `customer_id` / `install_id` / optional `channel=managed` + `cell_id` + `vpc_cidr` in [`sdk/aws/deploy/ecs/`](../deploy/ecs/)). Do not collapse multiple commercial customers onto one RDS or one API service.

Full runbook: [managed-channel.md](./managed-channel.md). Isolation proofs: [managed-channel-security.md](./managed-channel-security.md).

## Related

- [ADR-006: Majesta One JWT auth](../../../docs/adr/006-jwt-auth.md)
- [ADR-007: Platform Ops upgrades](../../../docs/adr/007-platform-ops-upgrades.md)
- [marketplace.md](./marketplace.md)
- [ADR-001: Dedicated install deploy](../../../docs/adr/001-dedicated-install.md)
- [BP-002](../../../backlog/BP-002-dedicated-install-fleet-ops.md)
- [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md)
- [BP-013](../../../backlog/BP-013-jwt-unified-principals.md)
- [architecture.md](../../../docs/architecture.md) (community AWS notes)
- [managed-channel.md](./managed-channel.md)
- [security.md](../../../docs/security.md)
- [tech-stack.md](../../../docs/tech-stack.md)
