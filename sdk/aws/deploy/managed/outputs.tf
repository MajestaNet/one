output "sns_topic_arn" {
  value       = aws_sns_topic.fleet.arn
  description = "SNS topic for managed-cell quota and ops alarms"
}

output "cognito_pool_alarm_names" {
  value       = { for k, a in aws_cloudwatch_metric_alarm.cognito_pools_utilization : k => a.alarm_name }
  description = "CloudWatch alarm names for 50/70/85% Cognito User pool utilization"
}

output "fleet_ops_role_arn" {
  value       = try(aws_iam_role.fleet_ops[0].arn, null)
  description = "Vendor FleetOps role (describe + upgrade Automation; deny secrets)"
}

output "breakglass_role_arn" {
  value       = try(aws_iam_role.breakglass[0].arn, null)
  description = "MFA break-glass role for tagged managed install secrets"
}

output "permission_boundary_arn" {
  value       = try(aws_iam_policy.permission_boundary[0].arn, null)
  description = "Attach as permissions boundary on human/CI roles in this cell"
}

output "cell_id" {
  value = var.cell_id
}

output "cognito_user_pools_quota" {
  value = var.cognito_user_pools_quota
}
