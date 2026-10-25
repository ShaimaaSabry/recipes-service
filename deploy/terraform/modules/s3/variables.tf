variable "project" {
  description = "Project name used in the bucket name."
  type        = string
}

variable "environment" {
  description = "Deployment environment used in the bucket name."
  type        = string
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
