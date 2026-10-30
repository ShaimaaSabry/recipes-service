# Every taggable resource gets these tags; Project is the cost allocation tag.
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

# ---------------------------------------------------------------------------
# Infrastructure shared with the other projects, owned by shomoa-clinics-service's
# Terraform and looked up here by name and tag: the VPC, the ALB and the
# Postgres instance (reached through the DATABASE_URL in the secret below).
# ---------------------------------------------------------------------------
data "aws_vpc" "shared" {
  tags = { Name = var.shared_vpc_name }
}

data "aws_subnets" "public" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.shared.id]
  }
  tags = { Tier = "public" }
}

data "aws_lb" "shared" {
  name = var.shared_alb_name
}

data "aws_lb_listener" "shared_https" {
  load_balancer_arn = data.aws_lb.shared.arn
  port              = 443
}

# environments/shared of this repo.
data "aws_ecr_repository" "service" {
  name = var.project
}

# ---------------------------------------------------------------------------
# Database connection. The database and its user are created by hand on the
# shared instance; Terraform only creates the secret, and its value is set by
# hand too, as {"DATABASE_URL": "postgres://...?sslmode=require"}. Terraform
# never reads or overwrites it. The service can't start until it's set.
# ---------------------------------------------------------------------------
resource "aws_secretsmanager_secret" "database" {
  name                    = "${var.project}-database"
  description             = "DATABASE_URL for ${var.project}. Created by Terraform; value set by hand."
  recovery_window_in_days = 0
}

# ---------------------------------------------------------------------------
# Photos bucket + CloudFront
# ---------------------------------------------------------------------------
module "s3" {
  source = "../../modules/s3"

  project     = var.project
  environment = var.environment
}

# ---------------------------------------------------------------------------
# HTTPS certificate for the API's hostname, added to the shared ALB's listener.
# ---------------------------------------------------------------------------
module "acm" {
  source = "../../modules/acm"

  domain_name    = var.domain_name
  hosted_zone_id = var.hosted_zone_id
}

# ---------------------------------------------------------------------------
# ECS Fargate Spot service behind the shared ALB
# ---------------------------------------------------------------------------
module "ecs" {
  source = "../../modules/ecs"

  project     = var.project
  environment = var.environment

  ecr_repository_url = data.aws_ecr_repository.service.repository_url
  image_tag          = var.image_tag

  vpc_id            = data.aws_vpc.shared.id
  public_subnet_ids = data.aws_subnets.public.ids

  https_listener_arn     = data.aws_lb_listener.shared_https.arn
  alb_security_group_id  = tolist(data.aws_lb.shared.security_groups)[0]
  host_header            = var.domain_name
  listener_rule_priority = var.listener_rule_priority
  certificate_arn        = module.acm.certificate_arn

  database_secret_arn = aws_secretsmanager_secret.database.arn
  photos_bucket_name  = module.s3.bucket_name
  photos_url          = module.s3.photos_url

  cpu    = var.cpu
  memory = var.memory
}

resource "aws_route53_record" "api" {
  zone_id = var.hosted_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = data.aws_lb.shared.dns_name
    zone_id                = data.aws_lb.shared.zone_id
    evaluate_target_health = true
  }
}
