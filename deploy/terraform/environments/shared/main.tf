# Every taggable resource gets these tags. Project is the cost allocation tag:
# Cost Explorer → Group by Tag "Project" → "mamie-nellie" shows what this costs.
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.tags
  }
}

locals {
  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Repository  = var.github_repository
  }
}

data "aws_caller_identity" "current" {}

# ---------------------------------------------------------------------------
# Networking: VPC, public subnets (ALB + ECS) and private subnets (RDS)
# ---------------------------------------------------------------------------
module "networking" {
  source = "../../modules/networking"

  project              = var.project
  vpc_cidr             = var.vpc_cidr
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  availability_zones   = var.availability_zones
}

# ---------------------------------------------------------------------------
# ECR: one repository; images are tagged with the git SHA by GitHub Actions
# ---------------------------------------------------------------------------
module "ecr" {
  source = "../../modules/ecr"

  project = var.project
}

# ---------------------------------------------------------------------------
# RDS: PostgreSQL 16 for prod. It lives here, not in prod, so tearing down and
# recreating the app never touches the data.
# ---------------------------------------------------------------------------
module "rds" {
  source = "../../modules/rds"

  project    = var.project
  vpc_id     = module.networking.vpc_id
  vpc_cidr   = module.networking.vpc_cidr
  subnet_ids = module.networking.private_subnet_ids

  db_identifier        = "${var.project}-db"
  db_name              = var.db_name
  db_username          = var.db_username
  db_instance_class    = var.db_instance_class
  db_allocated_storage = var.db_allocated_storage

  deletion_protection     = true
  backup_retention_period = var.backup_retention_period
  admin_cidr_blocks       = var.admin_cidr_blocks
}

# ---------------------------------------------------------------------------
# GitHub Actions deploy role, assumed through OIDC by this repo's workflows.
# The account's GitHub OIDC provider already exists (shomoa-clinics uses it).
# Permissions only reach mamie-nellie resources, so this repo can't touch
# another project's.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "github_actions" {
  name        = "github-${var.project}-service"
  description = "Assumed by ${var.github_repository} GitHub Actions via OIDC to deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/token.actions.githubusercontent.com"
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
          # Only main, and jobs running in the prod GitHub environment.
          "token.actions.githubusercontent.com:sub" = [
            "repo:${var.github_repository}:ref:refs/heads/main",
            "repo:${var.github_repository}:environment:prod",
          ]
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "github_actions_deploy" {
  name = "deploy"
  role = aws_iam_role.github_actions.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ECRLogin"
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Sid    = "ECRPushAndTag"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload",
          "ecr:PutImage",
          "ecr:DescribeImages",
        ]
        Resource = module.ecr.repository_arn
      },
      {
        # These ECS calls don't support resource-level permissions.
        Sid    = "ECSTaskDefinitions"
        Effect = "Allow"
        Action = [
          "ecs:DescribeTaskDefinition",
          "ecs:RegisterTaskDefinition",
          "ecs:ListTaskDefinitions",
          "ecs:ListTasks",
          "ecs:DescribeTasks",
        ]
        Resource = "*"
      },
      {
        Sid      = "ECSUpdateOwnService"
        Effect   = "Allow"
        Action   = ["ecs:UpdateService", "ecs:DescribeServices"]
        Resource = "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:service/${var.project}-*/*"
      },
      {
        Sid      = "ELBHealth"
        Effect   = "Allow"
        Action   = ["elasticloadbalancing:DescribeTargetHealth"]
        Resource = "*"
      },
      {
        Sid      = "PassOwnTaskRoles"
        Effect   = "Allow"
        Action   = ["iam:PassRole"]
        Resource = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${var.project}-*"
        Condition = {
          StringEquals = { "iam:PassedToService" = "ecs-tasks.amazonaws.com" }
        }
      },
    ]
  })
}

# ---------------------------------------------------------------------------
# Seeing what the project has and what it costs
# ---------------------------------------------------------------------------

# Console → Resource Groups → "mamie-nellie": every resource with the tag, in all
# environments.
resource "aws_resourcegroups_group" "project" {
  name        = var.project
  description = "Every resource tagged Project=${var.project}"

  resource_query {
    query = jsonencode({
      ResourceTypeFilters = ["AWS::AllSupported"]
      TagFilters          = [{ Key = "Project", Values = [var.project] }]
    })
  }
}

# A monthly budget over everything tagged with the project. Emails go out at 80%
# of the forecast and 100% of the actual spend, when addresses are given.
# Needs "Project" activated as a cost allocation tag (see deploy/README.md).
resource "aws_budgets_budget" "project" {
  name         = "${var.project}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  cost_filter {
    name   = "TagKeyValue"
    values = ["user:Project$${var.project}"]
  }

  dynamic "notification" {
    for_each = length(var.budget_alert_emails) > 0 ? [
      { type = "FORECASTED", threshold = 80 },
      { type = "ACTUAL", threshold = 100 },
    ] : []
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value.threshold
      threshold_type             = "PERCENTAGE"
      notification_type          = notification.value.type
      subscriber_email_addresses = var.budget_alert_emails
    }
  }
}
