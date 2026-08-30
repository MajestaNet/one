# Customer implementation Git (ADR-012): one CodeCommit repo per CUSTOMER_ID.
# Secondary peer installs should set provision_customer_repo=false and pass customer_repo_url.

variable "provision_customer_repo" {
  type        = bool
  default     = true
  description = "Create CodeCommit one-<customer_id>. Set false on peer installs that reuse an existing customer repo."
}

variable "customer_repo_url" {
  type        = string
  default     = ""
  description = "Override CUSTOMER_REPO_URL (required when provision_customer_repo=false)."
}

resource "aws_codecommit_repository" "custom" {
  count           = var.provision_customer_repo ? 1 : 0
  repository_name = "one-${var.customer_id}"
  description     = "Majesta One customer implementation (one/v1) for ${var.customer_id}"
  default_branch  = "main"

  tags = {
    CustomerId    = var.customer_id
    OneRole = "customer-repo"
  }
}

locals {
  customer_repo_url_effective = var.provision_customer_repo ? aws_codecommit_repository.customer[0].clone_url_http : var.customer_repo_url
  customer_repo_provider      = local.customer_repo_url_effective != "" ? "codecommit" : ""
}

resource "aws_iam_policy" "customer_repo_git" {
  count       = var.provision_customer_repo ? 1 : 0
  name        = "${local.name}-customer-repo-git"
  description = "Git pull/push on the customer CodeCommit repo"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "codecommit:GitPull",
        "codecommit:GitPush",
        "codecommit:GetRepository",
        "codecommit:ListBranches",
        "codecommit:GetBranch",
        "codecommit:GetCommit",
        "codecommit:GetDifferences",
      ]
      Resource = aws_codecommit_repository.customer[0].arn
    }]
  })
}
