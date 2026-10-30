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
  description = "API hostname (e.g. api.mamienellie.com), routed to this service on the shared ALB."
  type        = string
}

variable "hosted_zone_id" {
  description = "Route 53 hosted zone of the domain."
  type        = string
}

variable "shared_vpc_name" {
  description = "Name tag of the shared VPC (owned by shomoa-clinics-service)."
  type        = string
}

variable "shared_alb_name" {
  description = "Name of the shared ALB (owned by shomoa-clinics-service)."
  type        = string
}

variable "listener_rule_priority" {
  description = "Priority of this service's rule on the shared HTTPS listener. Unique on that listener."
  type        = number
}

variable "apple_client_ids" {
  description = "Bundle IDs Sign in with Apple tokens must be issued for. The service won't start without one."
  type        = list(string)
}

variable "google_client_ids" {
  description = "Google OAuth client IDs (iOS, Android, web) Google Sign-In tokens must be issued for. The service won't start without one."
  type        = list(string)
}
