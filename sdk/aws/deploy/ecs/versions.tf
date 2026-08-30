# Majesta One ECS Fargate reference stack (Phase 1)
#
# Dedicated install: ALB (2 AZs) → Fargate api/worker → RDS Multi-AZ
# Cognito User Pool for human OIDC (Phase 2 wiring via OIDC_* env).
#
# Usage:
#   cd deploy/aws/ecs
#   terraform init
#   terraform plan -var="project=one" -var="api_image=public.ecr.aws/one/api:0.1.0" \
#     -var="worker_image=public.ecr.aws/one/worker:0.1.0" \
#     -var="db_password=CHANGE_ME_LONG" -var="api_keys=prod-admin-key:client+metadata+deploy"
#   terraform apply ...
#
# Marketplace listing remains portal work (BP-011). This is the fulfillment topology.

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.5"
    }
  }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = merge(
      {
        Product        = "Majesta One"
        ProductVersion = var.product_version
        CustomerId       = var.customer_id
        InstallId      = var.install_id
        InstallRole    = var.install_role
        MarketplaceSku = var.marketplace_sku
        Channel        = var.channel
        ManagedBy      = "terraform"
      },
      var.cell_id != "" ? { CellId = var.cell_id } : {},
    )
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name        = "${var.project}-${var.install_id}"
  azs         = slice(data.aws_availability_zones.available.names, 0, 2)
  oidc_issuer = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.main.id}"
}
