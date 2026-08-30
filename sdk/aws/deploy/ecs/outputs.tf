output "alb_dns_name" {
  value       = aws_lb.main.dns_name
  description = "Public ALB DNS name for the Majesta One API"
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "api_service_name" {
  value = aws_ecs_service.api.name
}

output "worker_service_name" {
  value = aws_ecs_service.worker.name
}

output "rds_endpoint" {
  value = aws_db_instance.main.address
}

output "secrets_arn" {
  value = aws_secretsmanager_secret.install.arn
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.main.id
}

output "cognito_app_client_id" {
  value = aws_cognito_user_pool_client.api.id
}

output "oidc_issuer" {
  value = local.oidc_issuer
}

output "oidc_jwks_uri" {
  value = "${local.oidc_issuer}/.well-known/jwks.json"
}

output "customer_repo_url" {
  value       = local.customer_repo_url_effective
  description = "HTTPS clone URL for the customer one/v1 CodeCommit repo (shared across peer installs)"
}

output "customer_repo_name" {
  value       = var.provision_customer_repo ? aws_codecommit_repository.customer[0].repository_name : ""
  description = "CodeCommit repository name when provisioned by this stack"
}

output "customer_repo_git_policy_arn" {
  value       = var.provision_customer_repo ? aws_iam_policy.customer_repo_git[0].arn : ""
  description = "IAM policy ARN granting GitPull/GitPush on the customer repo (attach to human/CI roles)"
}

output "waf_web_acl_arn" {
  value       = aws_wafv2_web_acl.api.arn
  description = "WAFv2 WebACL ARN associated with the API ALB (Metadata install/exposure)"
}

output "waf_ip_set_ids" {
  value = { for f, s in aws_wafv2_ip_set.family : f => s.id }
}

output "upgrade_automation_document_name" {
  value       = try(aws_ssm_document.product_upgrade[0].name, null)
  description = "SSM Automation document for guided product image upgrades (ADR-007)"
}
