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
  description = "Deployment environment; also the service's APP_ENV."
  type        = string
  default     = "prod"
}

variable "github_repository" {
  description = "GitHub repository (owner/name), recorded in the Repository tag."
  type        = string
}

variable "image_tag" {
  description = "Image for the first task definition. Later deploys come from GitHub Actions."
  type        = string
  default     = "latest"
}

variable "cpu" {
  description = "Task CPU units (256 = 0.25 vCPU)."
  type        = string
}

variable "memory" {
  description = "Task memory in MiB."
  type        = string
}

variable "domain_name" {
  description = "API hostname (e.g. api.mamienellie.com). Empty serves plain HTTP on the ALB's own name."
  type        = string
  default     = ""
}

variable "hosted_zone_id" {
  description = "Route 53 hosted zone of the domain. Required when domain_name is set."
  type        = string
  default     = ""

  validation {
    condition     = var.domain_name == "" || var.hosted_zone_id != ""
    error_message = "hosted_zone_id is required when domain_name is set."
  }
}
