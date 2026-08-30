# ADR-001: Dedicated install

## Status

Accepted

## Context

Shared multi-tenant SaaS platforms often use shared pods. Majesta One targets B2B customers who install an instance on their own infra or cloud (Path A DigitalOcean App Platform or Path B Compose/Helm). Historical community AWS Marketplace / managed-cell materials live under [`sdk/aws`](../../sdk/aws/README.md) and are **not** a product GA channel.

## Decision

Each customer runs a **dedicated install**: one Postgres database, one API control plane, no SaaS `tenant_id` isolation column on business tables. Cross-customer isolation is provided by infrastructure (separate VPC/cluster/database), not by the application.

**Cloud-account ownership is not product tenancy.** Isolation may live in a subscriber-owned cloud account (App Platform / self-host / optional community AWS) as long as each commercial customer still gets a dedicated database and control plane. Multi-org SaaS on one shared database remains out of scope. A vendor-operated managed subscription fleet is **not** a product goal.

In-org sharing (owner, roles, permission sets) still exists inside that single org.

## Consequences

- Simpler data model and query paths.
- Dual-path packaging: App Platform (Path A) and Compose/Helm (Path B). AMI is not used.
- Community AWS ECS/managed-cell notes under [`sdk/aws/docs/`](../../sdk/aws/docs/) do not relax the one-DB / one-control-plane rule and are not a managed subscription GA. See [managed-channel.md](../../sdk/aws/docs/managed-channel.md) and [managed-channel-security.md](../../sdk/aws/docs/managed-channel-security.md).
- Multi-org SaaS on one DB is explicitly out of scope.
