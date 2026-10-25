# Infrastructure

Terraform for running the service on AWS (us-east-1), laid out like shomoa-clinics-service's `infra/terraform`:

```
deploy/terraform/
  modules/        networking, ecr, rds, ecs, s3, acm
  environments/
    shared/       VPC, ECR, RDS (+ its DATABASE_URL secret), GitHub deploy role, resource group, budget
    prod/         ECS Fargate Spot service + ALB, photos bucket + CloudFront, HTTPS cert + DNS (once a domain is set)
```

Local development doesn't use any of this; it runs on Docker Compose (see the main README).

## Tags: what belongs to the project and what it costs

Every taggable resource is tagged by the provider's `default_tags`:

| Tag | Value |
|---|---|
| `Project` | `mamie-nellie` |
| `Environment` | `shared` or `prod` |
| `ManagedBy` | `terraform` |
| `Repository` | `ShaimaaSabry/recipes-service` |

ECS tasks get the service's tags (`propagate_tags`), and RDS snapshots the instance's.

- **Everything the project has:** Console → Resource Groups & Tag Editor → Saved resource groups → `mamie-nellie`.
- **What it costs:** Billing → Cost Explorer → Group by: Tag → `Project`, then pick `mamie-nellie`
  (shomoa-clinics shows as `shomoa-clinics` under the same tag).
- **Budget:** `mamie-nellie-monthly` (50 USD) in Billing → Budgets. Alerts are emailed at 80% of the
  forecast and 100% of the actual, once `budget_alert_emails` is set as a variable in the
  `mamie-nellie-shared` workspace.

`Project` must be an **active cost allocation tag** (Billing → Cost allocation tags) for Cost Explorer
and the budget to see it. It's account-wide, so if shomoa-clinics already activated it there is
nothing to do. Tags only apply to costs from the day they're active. A few charges can't carry tags
at all (data transfer, and the hourly public IPv4 charge is billed per IP), so expect a small amount
under "No tag key".

### Roughly what it costs (us-east-1, per month)

| | |
|---|---|
| ALB | ~$17 |
| RDS db.t4g.micro + 20 GB gp3 | ~$14 |
| Public IPv4 addresses (ALB ×2, task ×1) | ~$11 |
| Fargate Spot, 0.25 vCPU / 0.5 GB, one task | ~$3 |
| Secrets Manager, CloudWatch Logs, ECR, S3, CloudFront | ~$2 |
| **Total** | **~$45–50** |

## First-time setup

1. **Terraform Cloud:** create the organization `mamie-nellie` and two workspaces using the
   **Version Control workflow**, connected to this GitHub repo, with remote execution:
   - `mamie-nellie-shared`: working directory `deploy/terraform/environments/shared`, auto apply on.
   - `mamie-nellie-prod`: working directory `deploy/terraform/environments/prod`, auto apply off, so
     you confirm each prod change in the TFC UI.

   In each, set the run trigger to "only when files in specified paths change": its own
   environment folder and `deploy/terraform/modules/`, so Go-only pushes don't start Terraform runs.
   Also add the sensitive environment variables `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
   and `AWS_DEFAULT_REGION=us-east-1`.
2. **Push to main.** Every push that changes `deploy/terraform/` starts the runs by itself. Shared
   must be applied before prod, which looks shared's resources up: the first time, confirm (or let
   auto apply finish) the shared run, then start a new run in the prod workspace and confirm it.
3. **GitHub secret:** set shared's `github_actions_role_arn` output as `AWS_DEPLOY_ROLE_ARN` in
   this repo.
4. **Prod's single task can't start until an image exists**, so ECS keeps retrying until the next step.
5. **First deploy:** Actions → Deploy to Prod. It pushes the image and starts the service.
6. **GitHub environment** `prod` (Settings → Environments): add yourself as a required reviewer if
   you want to approve each deploy.

From then on, deploy with Actions → **Deploy to Prod**: checks → build → scan → deploy → health check.
If a new version keeps failing its health check, ECS rolls back to the previous one by itself.

## Domain (needed before the app uses prod)

Until `domain_name` and `hosted_zone_id` are set in `environments/prod/terraform.tfvars`, the ALB
serves plain HTTP on its AWS hostname (the `api_url` output). The app's Android build blocks
cleartext HTTP, so buy or pick a domain, put its Route 53 hosted zone ID there and apply prod: that
adds the HTTPS certificate, redirects HTTP to HTTPS and points `api.<domain>` at the ALB.

## Database access

The database is private. To connect from pgAdmin, set `admin_cidr_blocks = ["<your IP>/32"]` in
`environments/shared/terraform.tfvars` and apply shared; the credentials are in the
`mamie-nellie-database` secret. The instance has deletion protection and keeps a final snapshot.

## Checks

CI runs `terraform fmt -check`, `terraform validate` for both environments and TFLint (with the AWS
ruleset) on every push. Locally, without installing Terraform:

```bash
docker run --rm -v "$PWD/deploy/terraform:/tf" -w /tf hashicorp/terraform fmt -recursive
```
