variable "project" {
  description = "Project name used for resource names."
  type        = string
}

variable "vpc_id" {
  description = "VPC for the database and its security group."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR of the VPC, allowed to reach Postgres."
  type        = string
}

variable "subnet_ids" {
  description = "Private subnet IDs for the DB subnet group (at least two availability zones)."
  type        = list(string)
}

variable "db_identifier" {
  description = "RDS instance identifier, unique per region in the account."
  type        = string
}

variable "db_engine_version" {
  description = "PostgreSQL major version."
  type        = string
  default     = "16"
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage" {
  description = "Storage in GiB."
  type        = number
  default     = 20
}

variable "db_name" {
  description = "Database the service uses."
  type        = string
}

variable "db_username" {
  description = "Master username."
  type        = string
}

variable "deletion_protection" {
  description = "Block deleting the instance. Keep on while it holds real data."
  type        = bool
  default     = true
}

variable "backup_retention_period" {
  description = "Days of automated backups to keep."
  type        = number
  default     = 7
}

variable "admin_cidr_blocks" {
  description = "CIDRs allowed to connect directly (e.g. your home IP as x.x.x.x/32). Non-empty makes the instance publicly reachable from them."
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
