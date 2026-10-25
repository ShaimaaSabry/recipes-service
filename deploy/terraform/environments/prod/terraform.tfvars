# ---------------------------------------------------------------------------
# Prod environment: non-sensitive settings. Do NOT add secrets here.
# The VPC, subnets, ECR repository and database secret are looked up from
# environments/shared by name and tag.
# ---------------------------------------------------------------------------
aws_region        = "us-east-1"
project           = "mamie-nellie"
environment       = "prod"
github_repository = "ShaimaaSabry/recipes-service"

# Go needs little: 0.25 vCPU / 512 MB.
cpu    = "256"
memory = "512"

# TODO: the API's domain. Until it's set, the ALB serves plain HTTP on its AWS
# name, which the app's prod build can't use (Android blocks cleartext).
domain_name    = ""
hosted_zone_id = ""
