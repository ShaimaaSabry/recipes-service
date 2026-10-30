variable "aws_region" {
  description = "AWS region for every resource."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Project name: the Project tag on every resource, and the prefix of their names."
  type        = string
  default     = "mamie-nellie"
}

variable "environment" {
  description = "Environment tag for these shared resources."
  type        = string
  default     = "shared"
}

variable "github_repository" {
  description = "GitHub repository (owner/name) allowed to deploy."
  type        = string
}

variable "monthly_budget_usd" {
  description = "Monthly budget for everything tagged with the project."
  type        = number
}

variable "budget_alert_emails" {
  description = "Addresses that get budget alerts. Set as a TFC workspace variable rather than committing them."
  type        = list(string)
  default     = []
}
