output "vpc_id" {
  description = "ID of the VPC."
  value       = module.networking.vpc_id
}

output "public_subnet_ids" {
  description = "Public subnet IDs."
  value       = module.networking.public_subnet_ids
}

output "ecr_repository_url" {
  description = "ECR repository URL."
  value       = module.ecr.repository_url
}

output "db_instance_address" {
  description = "RDS hostname."
  value       = module.rds.db_instance_address
}

output "database_secret_name" {
  description = "Secret holding DATABASE_URL."
  value       = module.rds.database_secret_name
}

output "github_actions_role_arn" {
  description = "Set as the AWS_DEPLOY_ROLE_ARN secret in this repo."
  value       = aws_iam_role.github_actions.arn
}

output "resource_group_name" {
  description = "Console → Resource Groups → this name lists every project resource."
  value       = aws_resourcegroups_group.project.name
}
