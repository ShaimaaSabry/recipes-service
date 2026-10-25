# ---------------------------------------------------------------------------
# Remote state: Terraform Cloud, remote execution (Terraform runs inside TFC).
# The workspace is connected to this GitHub repo, so a push to main that
# touches this environment or the modules starts a run by itself.
#
# One-time setup:
#   1. In https://app.terraform.io create the organization "mamie-nellie" and
#      the workspace "mamie-nellie-shared" (Version Control workflow → GitHub →
#      ShaimaaSabry/recipes-service, Execution Mode: Remote):
#        - Terraform Working Directory: deploy/terraform/environments/shared
#        - Run triggers: "Only trigger runs when files in specified paths
#          change": deploy/terraform/environments/shared/ and deploy/terraform/modules/
#        - Auto apply: ON
#   2. Add AWS credentials as sensitive workspace environment variables:
#        AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION=us-east-1
#
# Commit .terraform.lock.hcl for reproducible provider versions.
# ---------------------------------------------------------------------------
terraform {
  cloud {
    organization = "mamie-nellie"

    workspaces {
      name = "mamie-nellie-shared"
    }
  }
}
