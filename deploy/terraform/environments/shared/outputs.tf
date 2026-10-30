output "ecr_repository_url" {
  description = "ECR repository URL."
  value       = module.ecr.repository_url
}

output "github_actions_role_arn" {
  description = "Set as the AWS_DEPLOY_ROLE_ARN secret in this repo."
  value       = aws_iam_role.github_actions.arn
}

output "resource_group_name" {
  description = "Console → Resource Groups → this name lists every project resource."
  value       = aws_resourcegroups_group.project.name
}
