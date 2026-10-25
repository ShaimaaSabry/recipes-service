variable "domain_name" {
  description = "Domain the certificate is for (e.g. api.example.com)."
  type        = string
}

variable "hosted_zone_id" {
  description = "Route 53 hosted zone the validation record goes in."
  type        = string
}

variable "tags" {
  description = "Tags merged onto every resource (the provider's default_tags already add Project)."
  type        = map(string)
  default     = {}
}
