# WAFv2 WebACL + per-family IP sets for Metadata install/exposure reconcile.
# Desired state is written via /metadata/v1/install/exposure; API task updates rules.
# Bootstrap matches edge.DefaultPolicy: allow Client/Auth (+ health); block control plane.

locals {
  waf_ip_prefix = "${local.name}-exp-"
  waf_families  = ["client", "auth", "metadata", "deploy", "ops"]
}

resource "aws_wafv2_ip_set" "family" {
  for_each = toset(local.waf_families)

  name               = "${local.waf_ip_prefix}${each.key}"
  description        = "Majesta One exposure allowlist for /${each.key}"
  scope              = "REGIONAL"
  ip_address_version = "IPV4"
  # Placeholder; API reconciler overwrites addresses on apply.
  addresses = ["192.0.2.0/32"]
}

resource "aws_wafv2_web_acl" "api" {
  name        = "${local.name}-api"
  description = "Majesta One path-family exposure (managed by API reconcile)"
  scope       = "REGIONAL"

  default_action {
    block {}
  }

  rule {
    name     = "one-allow-health"
    priority = 0

    action {
      allow {}
    }

    statement {
      or_statement {
        statement {
          byte_match_statement {
            positional_constraint = "STARTS_WITH"
            search_string         = "/healthz"
            field_to_match {
              uri_path {}
            }
            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
        statement {
          byte_match_statement {
            positional_constraint = "STARTS_WITH"
            search_string         = "/readyz"
            field_to_match {
              uri_path {}
            }
            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "oneAllowHealth"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "one-exp-client"
    priority = 10

    action {
      allow {}
    }

    statement {
      or_statement {
        statement {
          byte_match_statement {
            positional_constraint = "STARTS_WITH"
            search_string         = "/client/"
            field_to_match {
              uri_path {}
            }
            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
        statement {
          byte_match_statement {
            positional_constraint = "STARTS_WITH"
            search_string         = "/v1/"
            field_to_match {
              uri_path {}
            }
            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "oneExpclient"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "one-exp-auth"
    priority = 11

    action {
      allow {}
    }

    statement {
      byte_match_statement {
        positional_constraint = "STARTS_WITH"
        search_string         = "/auth/"
        field_to_match {
          uri_path {}
        }
        text_transformation {
          priority = 0
          type     = "NONE"
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "oneExpauth"
      sampled_requests_enabled   = true
    }
  }

  # Explicit block rules for control-plane families (default_action also blocks).
  dynamic "rule" {
    for_each = {
      metadata = 20
      deploy   = 21
      ops      = 22
    }
    content {
      name     = "one-exp-${rule.key}"
      priority = rule.value

      action {
        block {}
      }

      statement {
        byte_match_statement {
          positional_constraint = "STARTS_WITH"
          search_string         = "/${rule.key}/"
          field_to_match {
            uri_path {}
          }
          text_transformation {
            priority = 0
            type     = "NONE"
          }
        }
      }

      visibility_config {
        cloudwatch_metrics_enabled = true
        metric_name                = "oneExp${rule.key}"
        sampled_requests_enabled   = true
      }
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${replace(local.name, "-", "")}ApiAcl"
    sampled_requests_enabled   = true
  }
}

resource "aws_wafv2_web_acl_association" "alb" {
  resource_arn = aws_lb.main.arn
  web_acl_arn  = aws_wafv2_web_acl.api.arn
}

# WAF mutate is API-task only (worker has a separate least-privilege role).
resource "aws_iam_role_policy" "ecs_task_waf_exposure" {
  name = "${local.name}-task-waf-exposure"
  role = aws_iam_role.ecs_api_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "WafExposureUpdate"
        Effect = "Allow"
        Action = [
          "wafv2:GetWebACL",
          "wafv2:UpdateWebACL",
          "wafv2:GetIPSet",
          "wafv2:UpdateIPSet",
          "wafv2:ListIPSets",
        ]
        Resource = concat(
          [aws_wafv2_web_acl.api.arn],
          [for f in local.waf_families : aws_wafv2_ip_set.family[f].arn],
        )
      }
    ]
  })
}
