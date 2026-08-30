# Cognito User pools / Region utilization alarms for managed cells.
# Metric is published by scripts/publish-cognito-pool-utilization.sh
# (namespace Majesta One/Managed, metric CognitoUserPoolUtilizationPercent).

resource "aws_sns_topic" "fleet" {
  name = "${local.name_prefix}-fleet"
}

resource "aws_sns_topic_subscription" "fleet_email" {
  count     = var.alarm_email != "" ? 1 : 0
  topic_arn = aws_sns_topic.fleet.arn
  protocol  = "email"
  endpoint  = var.alarm_email
}

locals {
  quota_thresholds = {
    warn50  = 50
    plan70  = 70
    block85 = 85
  }
}

resource "aws_cloudwatch_metric_alarm" "cognito_pools_utilization" {
  for_each = local.quota_thresholds

  alarm_name          = "${local.name_prefix}-cognito-pools-${each.key}"
  alarm_description   = "Managed cell ${var.cell_id}: Cognito User pools utilization ≥ ${each.value}% of quota ${var.cognito_user_pools_quota}. See docs/managed-channel.md cell-split policy."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = each.value
  treat_missing_data  = "notBreaching"

  metric_name = "CognitoUserPoolUtilizationPercent"
  namespace   = "Majesta One/Managed"
  period      = 300
  statistic   = "Maximum"

  dimensions = {
    CellId = var.cell_id
    Region = var.aws_region
  }

  alarm_actions = [aws_sns_topic.fleet.arn]
  ok_actions    = [aws_sns_topic.fleet.arn]
}
