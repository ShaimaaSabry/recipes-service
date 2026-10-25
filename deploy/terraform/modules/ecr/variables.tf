variable "project" {
  description = "Project name, used as the repository name."
  type        = string
}

variable "keep_last_n_images" {
  description = "Number of recent images to keep, not counting prod-* tagged ones."
  type        = number
  default     = 10
}

variable "keep_last_n_prod_images" {
  description = "Number of prod-deployed (prod-* tagged) images to keep as rollback targets."
  type        = number
  default     = 20
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
