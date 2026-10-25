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
  description = "VPC for the ALB and tasks."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnets for the ALB and tasks (tasks get public IPs; no NAT Gateway)."
  type        = list(string)
}

variable "database_secret_arn" {
  description = "Secrets Manager secret whose DATABASE_URL key is injected into the container."
  type        = string
}

variable "photos_bucket_name" {
  description = "S3 bucket for recipe photos."
  type        = string
}

variable "photos_url" {
  description = "Public base URL photos are served from (CloudFront)."
  type        = string
}

variable "certificate_arn" {
  description = "Validated ACM certificate for HTTPS. Null serves plain HTTP (only until a domain is set up)."
  type        = string
  default     = null
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
