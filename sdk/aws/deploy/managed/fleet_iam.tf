# Vendor fleet IAM fence: product task roles stay install-scoped in ../ecs.
# These roles are for vendor operators/automation in the regional cell account.

data "aws_iam_policy_document" "fleet_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["arn:${local.partition}:iam::${local.account_id}:root"]
    }
    condition {
      test     = "Bool"
      variable = "aws:MultiFactorAuthPresent"
      values   = ["true"]
    }
  }
}

data "aws_iam_policy_document" "fleet_ops" {
  # Inventory / describe across the cell — no secret plaintext.
  statement {
    sid    = "DescribeFleet"
    effect = "Allow"
    actions = [
      "ecs:Describe*",
      "ecs:List*",
      "rds:Describe*",
      "rds:ListTagsForResource",
      "cognito-idp:Describe*",
      "cognito-idp:ListUserPools",
      "cognito-idp:ListUserPoolClients",
      "elasticloadbalancing:Describe*",
      "ec2:DescribeVpcs",
      "ec2:DescribeSubnets",
      "ec2:DescribeSecurityGroups",
      "ec2:DescribeRouteTables",
      "logs:DescribeLogGroups",
      "logs:DescribeLogStreams",
      "cloudwatch:DescribeAlarms",
      "cloudwatch:GetMetricData",
      "cloudwatch:ListMetrics",
      "tag:GetResources",
      "resource-groups:SearchResources",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "StartInstallLocalUpgrade"
    effect = "Allow"
    actions = [
      "ssm:StartAutomationExecution",
      "ssm:GetAutomationExecution",
      "ssm:DescribeAutomationExecutions",
      "ssm:DescribeDocument",
      "ssm:ListDocuments",
    ]
    resources = [
      "arn:${local.partition}:ssm:${var.aws_region}:${local.account_id}:document/One-ProductUpgrade-*",
      "arn:${local.partition}:ssm:${var.aws_region}:${local.account_id}:automation-execution/*",
      "arn:${local.partition}:ssm:${var.aws_region}:${local.account_id}:automation-definition/*",
    ]
  }

  # Hard fence: fleet ops must never read install secrets.
  statement {
    sid    = "DenyInstallSecrets"
    effect = "Deny"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:UpdateSecret",
      "secretsmanager:DeleteSecret",
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["*"]
  }
}

data "aws_iam_policy_document" "breakglass" {
  statement {
    sid    = "ReadTaggedInstallSecrets"
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret",
    ]
    resources = [
      "arn:${local.partition}:secretsmanager:${var.aws_region}:${local.account_id}:secret:one-*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/Channel"
      values   = ["managed"]
    }
    condition {
      test     = "NumericLessThan"
      variable = "aws:MultiFactorAuthAge"
      values   = [tostring(var.breakglass_mfa_age_seconds)]
    }
  }

  statement {
    sid    = "DenyUntaggedOrMarketplaceSecrets"
    effect = "Deny"
    actions = [
      "secretsmanager:GetSecretValue",
    ]
    resources = ["*"]
    condition {
      test     = "StringNotEquals"
      variable = "aws:ResourceTag/Channel"
      values   = ["managed"]
    }
  }
}

data "aws_iam_policy_document" "permission_boundary" {
  statement {
    sid       = "AllowManagedFleetSurface"
    effect    = "Allow"
    actions   = ["*"]
    resources = ["*"]
  }

  statement {
    sid    = "DenyDangerousAccountWide"
    effect = "Deny"
    actions = [
      "iam:CreateUser",
      "iam:CreateAccessKey",
      "iam:AttachUserPolicy",
      "iam:DeleteRolePermissionsBoundary",
      "iam:PutRolePermissionsBoundary",
      "organizations:*",
      "account:*",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "permission_boundary" {
  count       = var.enable_fleet_roles ? 1 : 0
  name        = "${local.name_prefix}-boundary"
  description = "Permission boundary for managed-cell human/CI roles"
  policy      = data.aws_iam_policy_document.permission_boundary.json
}

resource "aws_iam_role" "fleet_ops" {
  count                = var.enable_fleet_roles ? 1 : 0
  name                 = "${local.name_prefix}-FleetOps"
  assume_role_policy   = data.aws_iam_policy_document.fleet_assume.json
  permissions_boundary = aws_iam_policy.permission_boundary[0].arn
  tags = {
    RolePurpose = "fleet-ops"
  }
}

resource "aws_iam_role_policy" "fleet_ops" {
  count  = var.enable_fleet_roles ? 1 : 0
  name   = "${local.name_prefix}-fleet-ops"
  role   = aws_iam_role.fleet_ops[0].id
  policy = data.aws_iam_policy_document.fleet_ops.json
}

resource "aws_iam_role" "breakglass" {
  count                = var.enable_fleet_roles ? 1 : 0
  name                 = "${local.name_prefix}-Breakglass"
  assume_role_policy   = data.aws_iam_policy_document.fleet_assume.json
  permissions_boundary = aws_iam_policy.permission_boundary[0].arn
  tags = {
    RolePurpose = "breakglass-secrets"
  }
}

resource "aws_iam_role_policy" "breakglass" {
  count  = var.enable_fleet_roles ? 1 : 0
  name   = "${local.name_prefix}-breakglass"
  role   = aws_iam_role.breakglass[0].id
  policy = data.aws_iam_policy_document.breakglass.json
}
