resource "aws_cloudwatch_log_group" "api" {
  name              = "/one/${local.name}/api"
  retention_in_days = 30
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/one/${local.name}/worker"
  retention_in_days = 30
}

resource "aws_iam_role" "ecs_execution" {
  name = "${local.name}-ecs-exec"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "ecs_execution_secrets" {
  name = "${local.name}-exec-secrets"
  role = aws_iam_role.ecs_execution.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = [aws_secretsmanager_secret.install.arn]
    }]
  })
}

# API task role: identity write-through, WAF reconcile, Ops ECS rolls.
resource "aws_iam_role" "ecs_api_task" {
  name = "${local.name}-ecs-api-task"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

# Worker task role: logs only (no Cognito / WAF / ECS mutate).
resource "aws_iam_role" "ecs_worker_task" {
  name = "${local.name}-ecs-worker-task"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "ecs_api_task_logs" {
  name = "${local.name}-api-task-logs"
  role = aws_iam_role.ecs_api_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogStream",
        "logs:PutLogEvents",
      ]
      Resource = ["${aws_cloudwatch_log_group.api.arn}:*"]
    }]
  })
}

resource "aws_iam_role_policy" "ecs_worker_task_logs" {
  name = "${local.name}-worker-task-logs"
  role = aws_iam_role.ecs_worker_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogStream",
        "logs:PutLogEvents",
      ]
      Resource = ["${aws_cloudwatch_log_group.worker.arn}:*"]
    }]
  })
}

data "aws_caller_identity" "current" {}

# Ops API (/ops/v1/upgrades) may drive ECS rolls from the API task when enabled.
# Scoped to this install's cluster / services / task families (ARN patterns; no resource cycle).
resource "aws_iam_role_policy" "ecs_api_task_ops_upgrade" {
  name = "${local.name}-api-task-ops-upgrade"
  role = aws_iam_role.ecs_api_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "EcsUpgradeMutate"
        Effect = "Allow"
        Action = [
          "ecs:UpdateService",
          "ecs:RegisterTaskDefinition",
        ]
        Resource = [
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:service/${local.name}/${local.name}-api",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:service/${local.name}/${local.name}-worker",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:task-definition/${local.name}-api:*",
          "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:task-definition/${local.name}-worker:*",
        ]
      },
      {
        # Describe/List/Register on ECS often require broader Resource; still no UpdateService on *.
        Sid    = "EcsUpgradeRead"
        Effect = "Allow"
        Action = [
          "ecs:DescribeServices",
          "ecs:DescribeTaskDefinition",
          "ecs:DescribeTasks",
          "ecs:ListTasks",
          "ecs:RegisterTaskDefinition",
        ]
        Resource = "*"
      },
      {
        Sid    = "PassTaskRoles"
        Effect = "Allow"
        Action = ["iam:PassRole"]
        Resource = [
          aws_iam_role.ecs_execution.arn,
          aws_iam_role.ecs_api_task.arn,
          aws_iam_role.ecs_worker_task.arn,
        ]
      },
    ]
  })
}

resource "aws_ecs_cluster" "main" {
  name = local.name
  setting {
    name  = "containerInsights"
    value = var.enable_container_insights ? "enabled" : "disabled"
  }
}

resource "aws_ecs_cluster_capacity_providers" "main" {
  cluster_name       = aws_ecs_cluster.main.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]
  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
    base              = 1
  }
}

locals {
  common_secrets = [
    { name = "DATABASE_URL", valueFrom = "${aws_secretsmanager_secret.install.arn}:DATABASE_URL::" },
    { name = "API_KEYS", valueFrom = "${aws_secretsmanager_secret.install.arn}:API_KEYS::" },
    { name = "AUTH_JWT_SIGNING_KEY", valueFrom = "${aws_secretsmanager_secret.install.arn}:AUTH_JWT_SIGNING_KEY::" },
    { name = "AUTH_JWT_ISSUER", valueFrom = "${aws_secretsmanager_secret.install.arn}:AUTH_JWT_ISSUER::" },
    { name = "DEPLOY_SHARE_SECRET", valueFrom = "${aws_secretsmanager_secret.install.arn}:DEPLOY_SHARE_SECRET::" },
    { name = "OIDC_ISSUER", valueFrom = "${aws_secretsmanager_secret.install.arn}:OIDC_ISSUER::" },
    { name = "OIDC_AUDIENCE", valueFrom = "${aws_secretsmanager_secret.install.arn}:OIDC_AUDIENCE::" },
    { name = "OIDC_JWKS_URI", valueFrom = "${aws_secretsmanager_secret.install.arn}:OIDC_JWKS_URI::" },
    { name = "OIDC_DEFAULT_SCOPES", valueFrom = "${aws_secretsmanager_secret.install.arn}:OIDC_DEFAULT_SCOPES::" },
    { name = "OIDC_AUTO_PROVISION_USERS", valueFrom = "${aws_secretsmanager_secret.install.arn}:OIDC_AUTO_PROVISION_USERS::" },
  ]

  common_env = [
    { name = "APP_ENV", value = "production" },
    { name = "PORT", value = "8080" },
    { name = "CUSTOMER_ID", value = var.customer_id },
    { name = "INSTALL_ID", value = var.install_id },
    { name = "INSTALL_ROLE", value = var.install_role },
    { name = "PRODUCT_VERSION", value = var.product_version },
    { name = "API_REVISION_CURRENT", value = tostring(var.api_revision_current) },
    { name = "API_REVISION_MIN", value = tostring(var.api_revision_min) },
    { name = "FEATURE_FLAGS", value = var.feature_flags },
    { name = "DEPLOY_PEER_MODE", value = var.deploy_peer_mode },
    { name = "AUTO_SEED", value = "1" },
    { name = "LOG_LEVEL", value = "info" },
    { name = "ECS_CLUSTER", value = local.name },
    { name = "ECS_API_SERVICE", value = "${local.name}-api" },
    { name = "ECS_WORKER_SERVICE", value = "${local.name}-worker" },
    { name = "ECS_API_TASK_FAMILY", value = "${local.name}-api" },
    { name = "ECS_WORKER_TASK_FAMILY", value = "${local.name}-worker" },
    { name = "PLATFORM_PUBLIC_URL", value = local.platform_public_url },
    { name = "EXPOSURE_RECONCILE", value = "aws" },
    { name = "WAF_WEB_ACL_NAME", value = aws_wafv2_web_acl.api.name },
    { name = "WAF_WEB_ACL_ID", value = aws_wafv2_web_acl.api.id },
    { name = "WAF_IP_SET_PREFIX", value = local.waf_ip_prefix },
    { name = "WAF_IP_SET_IDS", value = join(",", [for f in local.waf_families : "${f}:${aws_wafv2_ip_set.family[f].id}"]) },
    { name = "WAF_SCOPE", value = "REGIONAL" },
    { name = "WAF_REGION", value = var.aws_region },
    { name = "IDENTITY_SYNC", value = "cognito" },
    { name = "COGNITO_USER_POOL_ID", value = aws_cognito_user_pool.main.id },
    { name = "COGNITO_ISSUER", value = local.oidc_issuer },
    { name = "COGNITO_REGION", value = var.aws_region },
    { name = "CUSTOMER_REPO_URL", value = local.customer_repo_url_effective },
    { name = "CUSTOMER_REPO_PROVIDER", value = local.customer_repo_provider },
    { name = "CUSTOMER_REPO_REGION", value = var.aws_region },
  ]
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_api_task.arn

  container_definitions = jsonencode([{
    name      = "api"
    image     = var.api_image
    essential = true
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    environment = local.common_env
    secrets     = local.common_secrets
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.api.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "api"
      }
    }
    # Distroless Go image has no shell/wget; ALB target-group health checks hit /healthz.
  }])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${local.name}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.worker_cpu
  memory                   = var.worker_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_worker_task.arn

  container_definitions = jsonencode([{
    name        = "worker"
    image       = var.worker_image
    essential   = true
    environment = concat(local.common_env, [
      { name = "WORKER_ID", value = "ecs-worker" },
    ])
    secrets = local.common_secrets
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.worker.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "worker"
      }
    }
  }])
}

resource "aws_ecs_service" "api" {
  name            = "${local.name}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = 8080
  }

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  depends_on = [
    aws_lb_listener.http_forward,
    aws_lb_listener.http_redirect,
    aws_lb_listener.https,
  ]
}

resource "aws_ecs_service" "worker" {
  name            = "${local.name}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}
