variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "cell_id" {
  type        = string
  description = "Managed cell identifier (e.g. us-east-1-a). One apply of this overlay per cell."
}

variable "cognito_user_pools_quota" {
  type        = number
  default     = 1000
  description = "Effective User pools per Region quota for this account (default soft limit 1000; raise after Service Quotas increase)."

  validation {
    condition     = var.cognito_user_pools_quota > 0
    error_message = "cognito_user_pools_quota must be positive."
  }
}

variable "alarm_email" {
  type        = string
  default     = ""
  description = "Optional email subscribed to the fleet SNS topic for quota alarms."
}

variable "breakglass_mfa_age_seconds" {
  type        = number
  default     = 3600
  description = "Max MFA auth age for OneManagedBreakglass (STS condition aws:MultiFactorAuthAge)."
}

variable "enable_fleet_roles" {
  type        = bool
  default     = true
  description = "Create FleetOps + Breakglass IAM roles for this cell."
}
