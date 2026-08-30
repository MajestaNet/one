variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "project" {
  type    = string
  default = "one"
}

variable "product_version" {
  type    = string
  default = "0.1.0"
}

variable "api_revision_current" {
  type        = number
  default     = 1
  description = "Install API_REVISION_CURRENT (ADR-025). Independent of product_version."
}

variable "api_revision_min" {
  type        = number
  default     = 1
  description = "Install API_REVISION_MIN (oldest pin this image still serves)."
}

variable "customer_id" {
  type    = string
  default = "change-me-customer"
}

variable "install_id" {
  type    = string
  default = "change-me-install"
}

variable "install_role" {
  type    = string
  default = "prod"
}

variable "marketplace_sku" {
  type    = string
  default = "one-standard"
}

variable "channel" {
  type        = string
  default     = "self-managed"
  description = "Commercial channel tag: self-managed | marketplace | managed"

  validation {
    condition     = contains(["self-managed", "marketplace", "managed"], var.channel)
    error_message = "channel must be self-managed, marketplace, or managed."
  }
}

variable "cell_id" {
  type        = string
  default     = ""
  description = "Managed cell id (e.g. us-east-1-a). Required when channel=managed; empty otherwise."

  validation {
    condition     = var.channel != "managed" || length(var.cell_id) > 0
    error_message = "cell_id is required when channel=managed."
  }
}

variable "vpc_cidr" {
  type        = string
  default     = "10.40.0.0/16"
  description = "VPC CIDR for this install. Isolated non-peered installs may reuse the default; assign unique CIDRs if Transit Gateway / peering is planned."
}

variable "api_image" {
  type        = string
  description = "Fully qualified API image (repo:tag). Go distroless builds from deploy/Dockerfile (CMD=api)."
}

variable "worker_image" {
  type        = string
  description = "Fully qualified worker image (repo:tag). Go distroless builds from deploy/Dockerfile (CMD=worker)."
}

variable "api_desired_count" {
  type    = number
  default = 2
}

variable "worker_desired_count" {
  type    = number
  default = 1
}

variable "api_cpu" {
  type    = number
  default = 512
}

variable "api_memory" {
  type    = number
  default = 1024
}

variable "worker_cpu" {
  type    = number
  default = 512
}

variable "worker_memory" {
  type    = number
  default = 1024
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.medium"
}

variable "db_name" {
  type    = string
  default = "one"
}

variable "db_username" {
  type    = string
  default = "one"
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "api_keys" {
  type        = string
  sensitive   = true
  description = "Comma-separated API_KEYS for machine principals"
}

variable "feature_flags" {
  type        = string
  default     = ""
  description = "Comma-separated FEATURE_FLAGS. Keep empty / omit agents until BP-006 tool allowlists ship."
}

variable "deploy_peer_mode" {
  type        = string
  default     = "allowlist"
  description = "DEPLOY_PEER_MODE: allowlist (recommended) or customer"
}

variable "deploy_share_secret" {
  type        = string
  default     = ""
  sensitive   = true
  description = "Optional override for DEPLOY_SHARE_SECRET. Empty generates a random secret."
}

variable "certificate_arn" {
  type        = string
  default     = ""
  description = "ACM cert ARN for HTTPS on the ALB. Required unless allow_http=true (dev only)."
}

variable "allow_http" {
  type        = bool
  default     = false
  description = "Dev-only: allow plaintext HTTP:80 when certificate_arn is empty. Production must use TLS."

  validation {
    condition     = var.allow_http || var.certificate_arn != ""
    error_message = "certificate_arn is required unless allow_http=true (development escape hatch only)."
  }
}

variable "cognito_callback_urls" {
  type        = list(string)
  default     = ["https://localhost/callback"]
  description = "OAuth callback URLs for the Admin/Client UI Cognito app client"
}

variable "cognito_logout_urls" {
  type        = list(string)
  default     = ["https://localhost/logout"]
  description = "OAuth logout URLs for the Admin/Client UI Cognito app client"
}

variable "enable_container_insights" {
  type    = bool
  default = true
}
