variable "project" {
  description = "Project name used for resource names."
  type        = string
}

variable "environment" {
  description = "Deployment environment; also the service's APP_ENV."
  type        = string
}

variable "ecr_repository_url" {
  description = "ECR repository URL (without tag)."
  type        = string
}

variable "image_tag" {
  description = "Image tag for the first task definition. Later deploys come from GitHub Actions."
  type        = string
  default     = "latest"
}

variable "vpc_id" {
  description = "VPC the tasks run in."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnets for the tasks (they get public IPs; no NAT Gateway)."
  type        = list(string)
}

variable "database_secret_arn" {
  description = "Secrets Manager secret whose DATABASE_URL key is injected into the container."
  type        = string
}

variable "auth_secret_arn" {
  description = "Secrets Manager secret whose JWT_SIGNING_KEY key is injected into the container."
  type        = string
}

variable "apple_client_ids" {
  description = "Bundle IDs Sign in with Apple tokens must be issued for."
  type        = list(string)
}

variable "google_client_ids" {
  description = "Google OAuth client IDs (iOS, Android, web) Google Sign-In tokens must be issued for."
  type        = list(string)
}

variable "photos_bucket_name" {
  description = "S3 bucket for recipe photos."
  type        = string
}

variable "photos_url" {
  description = "Public base URL photos are served from (CloudFront)."
  type        = string
}

variable "https_listener_arn" {
  description = "HTTPS listener of the shared ALB this service is routed through."
  type        = string
}

variable "alb_security_group_id" {
  description = "Security group of the shared ALB, allowed to reach the tasks."
  type        = string
}

variable "host_header" {
  description = "Hostname the shared ALB routes to this service (e.g. api.mamienellie.com)."
  type        = string
}

variable "listener_rule_priority" {
  description = "Priority of this service's rule on the shared HTTPS listener; unique on that listener."
  type        = number
}

variable "certificate_arn" {
  description = "Validated ACM certificate for host_header, added to the shared HTTPS listener."
  type        = string
}

variable "container_port" {
  description = "Port the service listens on."
  type        = number
  default     = 8080
}

variable "health_check_path" {
  description = "Path the ALB and container health checks call."
  type        = string
  default     = "/healthz"
}

variable "cpu" {
  description = "Task CPU units (256 = 0.25 vCPU)."
  type        = string
  default     = "256"
}

variable "memory" {
  description = "Task memory in MiB."
  type        = string
  default     = "512"
}

variable "log_retention_days" {
  description = "Days to keep the service's logs."
  type        = number
  default     = 14
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
