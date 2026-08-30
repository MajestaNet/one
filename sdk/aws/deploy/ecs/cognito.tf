# Cognito User Pool — default identity backend per install (ADR-006 amended).
# One pool for humans (passwordless-oriented + BYO IdP federation).
# Service/agent app clients are created via Client identity admin write-through.
# Cognito groups are transitional only — Majesta One Roles/PS remain AuthZ SoR.

resource "aws_cognito_user_pool" "main" {
  name = "${local.name}-users"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  # Password policy kept for federated-fallback / admin set-password paths.
  # Default product UX is passwordless-oriented (email OTP / magic link via Hosted UI
  # or CUSTOM_AUTH Lambdas — configure in managed overlay as needed).
  password_policy {
    minimum_length    = 12
    require_lowercase = true
    require_numbers   = true
    require_symbols   = true
    require_uppercase = true
  }

  # Enterprise baseline: require software-token MFA.
  mfa_configuration = "ON"
  software_token_mfa_configuration {
    enabled = true
  }

  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = true
    mutable             = true
  }

  schema {
    name                = "name"
    attribute_data_type = "String"
    required            = false
    mutable             = true
  }

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }
}

# Public SPA / Admin UI client (no secret) — authorization code + PKCE + SRP.
# ALLOW_USER_PASSWORD_AUTH intentionally omitted (raw password on public client).
resource "aws_cognito_user_pool_client" "api" {
  name         = "${local.name}-ui"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = var.cognito_callback_urls
  logout_urls                          = var.cognito_logout_urls
  explicit_auth_flows = [
    "ALLOW_REFRESH_TOKEN_AUTH",
    "ALLOW_USER_SRP_AUTH",
  ]
  prevent_user_existence_errors = "ENABLED"
  enable_token_revocation       = true
}

# Transitional groups (legacy OIDC path). Do not use as AuthZ SoR — Majesta One Roles/PS win.
resource "aws_cognito_user_group" "client" {
  name         = "one-client"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Legacy hint only — prefer Majesta One Roles"
}

resource "aws_cognito_user_group" "metadata" {
  name         = "one-metadata"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Legacy hint only — prefer Majesta One Roles"
}

resource "aws_cognito_user_group" "deploy" {
  name         = "one-deploy"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Legacy hint only — prefer Majesta One Roles"
}

resource "aws_cognito_user_group" "ops" {
  name         = "one-ops"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Legacy hint only — prefer Majesta One Roles"
}

resource "aws_cognito_user_group" "admin" {
  name         = "one-admin"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Legacy hint only — Majesta One admin from Roles/DB"
}

resource "aws_cognito_user_pool_domain" "main" {
  domain       = "${var.project}-${replace(var.install_id, "_", "-")}"
  user_pool_id = aws_cognito_user_pool.main.id
}

# IAM: API task may AdminCreateUser / CreateUserPoolClient for identity write-through.
resource "aws_iam_role_policy" "ecs_task_cognito_identity" {
  name = "${local.name}-task-cognito-identity"
  role = aws_iam_role.ecs_api_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "CognitoIdentityWriteThrough"
      Effect = "Allow"
      Action = [
        "cognito-idp:AdminCreateUser",
        "cognito-idp:AdminDisableUser",
        "cognito-idp:AdminEnableUser",
        "cognito-idp:AdminGetUser",
        "cognito-idp:CreateUserPoolClient",
        "cognito-idp:UpdateUserPoolClient",
        "cognito-idp:DeleteUserPoolClient",
        "cognito-idp:DescribeUserPoolClient",
      ]
      Resource = [aws_cognito_user_pool.main.arn]
    }]
  })
}
