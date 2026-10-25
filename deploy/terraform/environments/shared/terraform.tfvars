# ---------------------------------------------------------------------------
# Shared environment: non-sensitive settings. Do NOT add secrets here.
# ---------------------------------------------------------------------------
aws_region        = "us-east-1"
project           = "mamie-nellie"
github_repository = "ShaimaaSabry/recipes-service"

# Networking: a different range from shomoa-clinics (10.0.0.0/16), so the two
# VPCs could be peered one day.
vpc_cidr             = "10.20.0.0/16"
public_subnet_cidrs  = ["10.20.1.0/24", "10.20.2.0/24"]
private_subnet_cidrs = ["10.20.11.0/24", "10.20.12.0/24"]
availability_zones   = ["us-east-1a", "us-east-1b"]

# RDS
db_name                 = "mamie_nellie"
db_username             = "mamie_nellie_admin"
db_instance_class       = "db.t4g.micro"
db_allocated_storage    = 20
backup_retention_period = 7

# Set to ["<your IP>/32"] to connect from pgAdmin; empty keeps the database private.
admin_cidr_blocks = []

# Cost alerts. Set budget_alert_emails as a workspace variable in TFC.
monthly_budget_usd = 50
