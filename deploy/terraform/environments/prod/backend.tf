# ---------------------------------------------------------------------------
# Remote state: Terraform Cloud, remote execution (Terraform runs inside TFC).
# The workspace is connected to this GitHub repo, so a push to main that
# touches this environment or the modules starts a run by itself.
#
# One-time setup:
#   1. In https://app.terraform.io create the organization "mamie-nellie" and
#      the workspace "mamie-nellie-prod" (Version Control workflow → GitHub →
#      ShaimaaSabry/recipes-service, Execution Mode: Remote):
#        - Terraform Working Directory: deploy/terraform/environments/prod
#        - Run triggers: "Only trigger runs when files in specified paths
#          change": deploy/terraform/environments/prod/ and deploy/terraform/modules/
#        - Auto apply: OFF, so each prod change is confirmed in the TFC UI
#   2. Add AWS credentials as sensitive workspace environment variables:
#        AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION=us-east-1
#   3. Apply environments/shared first: prod looks its VPC, subnets, ECR
#      repository and database secret up by name and tag.
#
# Commit .terraform.lock.hcl for reproducible provider versions.
# ---------------------------------------------------------------------------
terraform {
  cloud {
    organization = "mamie-nellie"

    workspaces {
      name = "mamie-nellie-prod"
    }
  }
}
