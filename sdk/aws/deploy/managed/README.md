# Majesta One — managed subscription fleet overlay

Vendor-plane Terraform for a **managed regional cell**: fleet IAM fence + Cognito User Pool quota alarms.  
**Does not** create customer installs — each install remains a separate apply of [`../ecs/`](../ecs/) with unique `customer_id` / `install_id` / Cognito pool.

Docs: [managed-channel.md](../../docs/managed-channel.md), [managed-channel-security.md](../../docs/managed-channel-security.md).

## What this creates

| Resource | Purpose |
|---|---|
| SNS topic (optional email) | Quota / ops notifications |
| CloudWatch alarms @ 50 / 70 / 85% | Cognito User pools per Region utilization |
| IAM role `OneManagedFleetOps` | Describe/inventory + start install upgrade Automation; **deny** secret reads |
| IAM role `OneManagedBreakglass` | MFA-required secret read (tag-conditioned) |
| Permission boundary policy | Attach to human/CI roles that assume fleet roles |

## Prerequisites

- Terraform ≥ 1.5, AWS credentials in the **vendor regional cell account**
- Apply **once per cell** (not per customer)
- Per-customer stacks: still `cd ../ecs && terraform apply …` with `channel=managed` and `cell_id=…`

## Apply

```bash
cd sdk/aws/deploy/managed
terraform init
terraform apply \
  -var="aws_region=us-east-1" \
  -var="cell_id=us-east-1-a" \
  -var="cognito_user_pools_quota=1000" \
  -var="alarm_email=ops@example.com"
```

Publish utilization metrics (cron / EventBridge):

```bash
./scripts/publish-cognito-pool-utilization.sh
```

Isolation checklist after provisioning two installs:

```bash
./scripts/isolation-checklist.sh \
  --install-a-url https://a.example \
  --install-b-url https://b.example \
  --pool-a-id us-east-1_AAA \
  --pool-b-id us-east-1_BBB
```

## Non-goals

- Multi-tenant control plane inside `cmd/api`
- Shared Cognito User Pool or shared RDS across commercial customers
- Automatic `terraform apply` of customer stacks (orchestration stays vendor-side, BP-002)
