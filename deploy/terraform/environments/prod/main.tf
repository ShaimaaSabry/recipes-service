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
  has_domain = var.domain_name != ""
}

# ---------------------------------------------------------------------------
# What environments/shared created, looked up by name and tag, so nothing has
# to be copied from its outputs by hand.
# ---------------------------------------------------------------------------
data "aws_vpc" "shared" {
  tags = { Project = var.project, Name = "${var.project}-vpc" }
}

data "aws_subnets" "public" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.shared.id]
  }
  tags = { Tier = "public" }
}

data "aws_ecr_repository" "service" {
  name = var.project
}

data "aws_secretsmanager_secret" "database" {
  name = "${var.project}-database"
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
# HTTPS certificate and DNS name, once a domain is set in terraform.tfvars.
# ---------------------------------------------------------------------------
module "acm" {
  source = "../../modules/acm"
  count  = local.has_domain ? 1 : 0

  domain_name    = var.domain_name
  hosted_zone_id = var.hosted_zone_id
}

# ---------------------------------------------------------------------------
# ECS Fargate Spot service behind an ALB
# ---------------------------------------------------------------------------
module "ecs" {
  source = "../../modules/ecs"

  project     = var.project
  environment = var.environment

  ecr_repository_url = data.aws_ecr_repository.service.repository_url
  image_tag          = var.image_tag

  vpc_id            = data.aws_vpc.shared.id
  public_subnet_ids = data.aws_subnets.public.ids

  database_secret_arn = data.aws_secretsmanager_secret.database.arn
  photos_bucket_name  = module.s3.bucket_name
  photos_url          = module.s3.photos_url

  certificate_arn = local.has_domain ? module.acm[0].certificate_arn : null

  cpu    = var.cpu
  memory = var.memory
}

resource "aws_route53_record" "api" {
  count = local.has_domain ? 1 : 0

  zone_id = var.hosted_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = module.ecs.alb_dns_name
    zone_id                = module.ecs.alb_zone_id
    evaluate_target_health = true
  }
}
