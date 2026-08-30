resource "aws_db_subnet_group" "main" {
  name       = "${local.name}-db"
  subnet_ids = aws_subnet.private[*].id
}

resource "aws_db_instance" "main" {
  identifier                 = local.name
  engine                     = "postgres"
  engine_version             = "16"
  instance_class             = var.db_instance_class
  allocated_storage          = 50
  max_allocated_storage      = 200
  db_name                    = var.db_name
  username                   = var.db_username
  password                   = var.db_password
  db_subnet_group_name       = aws_db_subnet_group.main.name
  vpc_security_group_ids     = [aws_security_group.rds.id]
  multi_az                   = true
  publicly_accessible        = false
  storage_encrypted          = true
  backup_retention_period    = 7
  deletion_protection        = true
  skip_final_snapshot        = false
  final_snapshot_identifier  = "${local.name}-final"
  auto_minor_version_upgrade = true
}

resource "random_password" "auth_jwt_signing_key" {
  length  = 48
  special = false
}

resource "random_password" "deploy_share_secret" {
  length  = 48
  special = false
}

locals {
  database_url = "postgres://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:5432/${var.db_name}"
  # Prefer operator-supplied share secret; otherwise generate one so prod Deploy HMAC is always on.
  effective_deploy_share_secret = var.deploy_share_secret != "" ? var.deploy_share_secret : random_password.deploy_share_secret.result
  platform_public_url           = var.certificate_arn != "" ? "https://${aws_lb.main.dns_name}" : "http://${aws_lb.main.dns_name}"
}

resource "aws_secretsmanager_secret" "install" {
  name = "${local.name}/install"
  # Channel tag enables managed breakglass IAM condition (deploy/aws/managed/fleet_iam.tf).
  # default_tags already set Channel/CellId/CustomerId; explicit tags keep the secret discoverable.
  tags = {
    Name = "${local.name}/install"
  }
}

resource "aws_secretsmanager_secret_version" "install" {
  secret_id = aws_secretsmanager_secret.install.id
  secret_string = jsonencode({
    DATABASE_URL         = local.database_url
    API_KEYS             = var.api_keys
    AUTH_JWT_SIGNING_KEY = random_password.auth_jwt_signing_key.result
    AUTH_JWT_ISSUER      = "${local.platform_public_url}/auth/v1"
    CUSTOMER_ID            = var.customer_id
    INSTALL_ID           = var.install_id
    INSTALL_ROLE         = var.install_role
    PRODUCT_VERSION      = var.product_version
    FEATURE_FLAGS        = var.feature_flags
    DEPLOY_PEER_MODE     = var.deploy_peer_mode
    DEPLOY_SHARE_SECRET  = local.effective_deploy_share_secret
    # Optional smoke-test key for SSM upgrade automation (deploy-scoped). Empty skips suite runs.
    DEPLOY_SMOKE_API_KEY      = ""
    OIDC_ISSUER               = local.oidc_issuer
    OIDC_AUDIENCE             = aws_cognito_user_pool_client.api.id
    OIDC_JWKS_URI             = "${local.oidc_issuer}/.well-known/jwks.json"
    OIDC_DEFAULT_SCOPES       = "client"
    OIDC_AUTO_PROVISION_USERS = "0"
  })
}
