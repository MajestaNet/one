# Managed subscription fleet overlay (vendor cell account).
# Apply once per regional cell — not per customer install.
#
# See README.md and docs/managed-channel.md.

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = {
      Product   = "Majesta One"
      Channel   = "managed"
      CellId    = var.cell_id
      ManagedBy = "terraform"
      Plane     = "vendor-fleet"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  name_prefix = "one-managed-${var.cell_id}"
  account_id  = data.aws_caller_identity.current.account_id
  partition   = data.aws_partition.current.partition
}
