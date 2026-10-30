# ---------------------------------------------------------------------------
# Prod environment: non-sensitive settings. Do NOT add secrets here.
# The ECR repository is looked up from environments/shared; the VPC and ALB
# from shomoa-clinics-service, which owns them.
# ---------------------------------------------------------------------------
aws_region        = "us-east-1"
project           = "mamie-nellie"
environment       = "prod"
github_repository = "ShaimaaSabry/recipes-service"

# Go needs little: 0.25 vCPU / 512 MB.
cpu    = "256"
memory = "512"

# The API is served on HTTPS at this name; the zone is mamienellie.com in Route 53.
domain_name    = "api.mamienellie.com"
hosted_zone_id = "Z0721032PXVLPVY3S3AI"

# Shared with the other projects, owned by shomoa-clinics-service's
# environments/shared, whose terraform.tfvars lists the listener priorities.
shared_vpc_name        = "shomoa-clinics-vpc"
shared_alb_name        = "shared-alb"
listener_rule_priority = 200

# Sign-in: the app's IDs with Apple and Google. Not secret (they're inside the app), but required:
# the service refuses to start in prod without at least one of each.
apple_client_ids  = [] # e.g. ["com.mamienellie.app"]
google_client_ids = [] # e.g. ["<id>.apps.googleusercontent.com", ...] for iOS, Android and web
