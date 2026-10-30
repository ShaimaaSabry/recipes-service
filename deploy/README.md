# Infrastructure

Terraform for running the service on AWS (us-east-1), laid out like shomoa-clinics-service's `infra/terraform`:

```
deploy/terraform/
  modules/        ecr, ecs, s3, acm
  environments/
    shared/       ECR, GitHub deploy role, resource group, budget
    prod/         ECS Fargate Spot service, DATABASE_URL and auth secrets, photos bucket + CloudFront, HTTPS cert + DNS
```

The VPC, the ALB and the Postgres instance are shared with shomoa-clinics and data-magic, and owned
by shomoa-clinics-service's Terraform (`shomoa-clinics-vpc`, `shared-alb`, `shomoa-clinics-db`).
Prod looks them up by name and adds only its own pieces: a host-header rule (priority 200) and its
certificate on the ALB's HTTPS listener, a target group and the tasks' security group. Changes to
the shared resources themselves are made in shomoa-clinics-service.

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
| Public IPv4 address (task ×1) | ~$4 |
| Fargate Spot, 0.25 vCPU / 0.5 GB, one task | ~$3 |
| Secrets Manager, CloudWatch Logs, ECR, S3, CloudFront | ~$2 |
| **Total** | **~$9** |

The ALB and the database are shared, so they're billed under `shomoa-clinics`.

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
   must be applied before prod, which looks shared's resources up (and the shared VPC and ALB
   must exist in shomoa-clinics-service): the first time, confirm (or let
   auto apply finish) the shared run, then start a new run in the prod workspace and confirm it.
3. **GitHub secret:** set shared's `github_actions_role_arn` output as `AWS_DEPLOY_ROLE_ARN` in
   this repo.
4. **Database:** on the shared Postgres instance, create the `mamie_nellie` database and its own
   user (not the master user), then set the `mamie-nellie-database` secret's value to
   `{"DATABASE_URL": "postgres://<user>:<url-encoded password>@<host>:5432/mamie_nellie?sslmode=require"}`.
   Terraform creates the secret empty and never touches its value.
   Do the same for `mamie-nellie-auth`: `{"JWT_SIGNING_KEY": "<openssl rand -base64 48>"}`. Changing it
   later only signs everyone out.
   Also set `apple_client_ids` and `google_client_ids` in `environments/prod/terraform.tfvars`: the
   service refuses to start in prod without them.
5. **Prod's single task can't start until an image exists**, so ECS keeps retrying until the next step.
6. **First deploy:** Actions → Deploy to Prod. It pushes the image and starts the service.
7. **GitHub environment** `prod` (Settings → Environments): add yourself as a required reviewer if
   you want to approve each deploy.

From then on, deploy with Actions → **Deploy to Prod**: checks → build → scan → deploy → health check.
If a new version keeps failing its health check, ECS rolls back to the previous one by itself.

## Domain

The API is served at `https://api.mamienellie.com`. The domain is registered in Route 53 in this
account, and prod's `terraform.tfvars` names its hosted zone. Applying prod issues the HTTPS
certificate (validated by DNS in that zone), redirects HTTP to HTTPS and points
`api.mamienellie.com` at the shared ALB, which routes that hostname to this service. The app's Android build blocks plain HTTP, so it must use the
HTTPS name, not the ALB's own hostname.

## Database access

The database lives on the shared instance `shomoa-clinics-db`. Who may connect to it directly
(`admin_cidr_blocks`) is set in shomoa-clinics-service's Terraform; the service's own connection
string is in the `mamie-nellie-database` secret.

## Checks

CI runs `terraform fmt -check`, `terraform validate` for both environments and TFLint (with the AWS
ruleset) on every push. Locally, without installing Terraform:

```bash
docker run --rm -v "$PWD/deploy/terraform:/tf" -w /tf hashicorp/terraform fmt -recursive
```
