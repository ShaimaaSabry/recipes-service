output "api_url" {
  description = "Base URL of the API, for the app's prod EXPO_PUBLIC_API_URL."
  value       = local.has_domain ? "https://${var.domain_name}" : "http://${module.ecs.alb_dns_name}"
}

output "cluster_name" {
  description = "ECS cluster name."
  value       = module.ecs.cluster_name
}

output "service_name" {
  description = "ECS service name."
  value       = module.ecs.service_name
}

output "task_definition_family" {
  description = "Task definition family."
  value       = module.ecs.task_definition_family
}

output "photos_bucket_name" {
  description = "Photos bucket."
  value       = module.s3.bucket_name
}

output "photos_url" {
  description = "CloudFront URL photos are served from."
  value       = module.s3.photos_url
}
