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

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for the public subnets."
  type        = list(string)
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for the private subnets."
  type        = list(string)
}

variable "availability_zones" {
  description = "Availability zones for the subnets."
  type        = list(string)
}

variable "db_name" {
  description = "Database the service uses."
  type        = string
}

variable "db_username" {
  description = "RDS master username."
  type        = string
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
}

variable "db_allocated_storage" {
  description = "RDS storage in GiB."
  type        = number
}

variable "backup_retention_period" {
  description = "Days of automated RDS backups to keep."
  type        = number
}

variable "admin_cidr_blocks" {
  description = "CIDRs allowed to connect to RDS directly (e.g. your home IP as x.x.x.x/32). Empty keeps it private."
  type        = list(string)
  default     = []
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
