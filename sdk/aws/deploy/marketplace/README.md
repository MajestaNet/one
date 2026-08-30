# Majesta One — Marketplace customer install package

Sanitized fulfillment assets for AWS Marketplace **container** Quick Launch.
This directory is what listing / portal reviewers should treat as the customer-facing install package.

Detailed self-managed ops remain in [`../ecs/`](../ecs/) (Terraform). Do **not** attach the full vendor monorepo (`docs/`, `backlog/`, `.cursor/`, agent playbooks) as Marketplace assets.

## One-shot vs separate channels

| Channel | Artifact | Notes |
|---|---|---|
| **Marketplace runtime (this package)** | Images + [`quickstart.yaml`](./quickstart.yaml) | Single Quick Launch for ECS Fargate install |
| Control IDE | `control-ide-v*` desktop installers | Separate private download — never in product images |
| Self-managed / ops | `sdk/aws/deploy/ecs/` Terraform | Full reference; optional beyond Quick Launch |
| Managed subscription (future) | Same images, vendor regional accounts | Reuses this image stream |

## Allowlisted customer assets

| Include | Exclude |
|---|---|
| Published `api` / `worker` distroless image URIs | Source monorepo, `tools/`, `scripts/` |
| This Quick Launch CloudFormation template | `docs/`, `backlog/`, `.cursor/`, `AGENTS.md` |
| Parameters: `CustomerId`, `InstallId`, `InstallRole`, image URIs, ACM cert, DB password | Secret **values** (use Secrets Manager / stack params NoEcho) |
| Least-privilege task role actions documented in template | Vendor agent routing / agent-config trees |
| Apache-2.0 license reference | Backlog severity language / internal BP write-ups |

## Quick Launch

```bash
aws cloudformation deploy \
  --stack-name one-acme-prod \
  --template-file sdk/aws/deploy/marketplace/quickstart.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
    CustomerId=acme \
    InstallId=acme-prod-use1 \
    InstallRole=prod \
    ApiImage=public.ecr.aws/one/api:0.1.0 \
    WorkerImage=public.ecr.aws/one/worker:0.1.0 \
    DbPassword='REDACTED_LONG_PASSWORD' \
    ApiKeys='prod-bootstrap:client+metadata+deploy+ops+admin' \
    CertificateArn=arn:aws:acm:REGION:ACCOUNT:certificate/ID
```

For multi-AZ HA, WAF reconcile, Cognito transitional IdP, CodeCommit customer repo, and SSM upgrade Automation parity, apply the Terraform reference in [`../ecs/`](../ecs/) after Quick Launch or instead of it for self-managed installs.

## Related

- [deploy/marketplace.md](../../docs/marketplace.md)
- [docs/security.md](../../../docs/security.md) — IP / distribution posture
- [BP-011](../../../backlog/BP-011-container-marketplace-fargate.md)
