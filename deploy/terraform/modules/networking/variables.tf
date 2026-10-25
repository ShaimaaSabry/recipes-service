variable "project" {
  description = "Project name used for resource names."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for the public subnets, one per availability zone (ALB and ECS tasks)."
  type        = list(string)
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for the private subnets, one per availability zone. RDS needs at least two."
  type        = list(string)
}

variable "availability_zones" {
  description = "Availability zones for the subnets, in the same order as the CIDR lists."
  type        = list(string)
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
