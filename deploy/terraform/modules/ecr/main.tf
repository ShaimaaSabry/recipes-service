resource "aws_ecr_repository" "this" {
  name                 = var.project
  image_tag_mutability = "MUTABLE" # "latest" is re-pointed on every build; deploys pin by digest
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = merge(var.tags, { Name = var.project })
}

# ---------------------------------------------------------------------------
# Lifecycle policy
#   1. Images tagged prod-* (added by the deploy workflow once an image is live
#      and healthy) are the rollback targets: keep the last keep_last_n_prod_images.
#   2. Untagged images expire after a week.
#   3. Everything else keeps only the last keep_last_n_images.
# ECR never lets a lower-priority rule expire an image a higher one selected.
# ---------------------------------------------------------------------------
resource "aws_ecr_lifecycle_policy" "this" {
  repository = aws_ecr_repository.this.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the last ${var.keep_last_n_prod_images} images deployed to prod"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["prod-"]
          countType     = "imageCountMoreThan"
          countNumber   = var.keep_last_n_prod_images
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 2
        description  = "Expire untagged images after 7 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 7
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 3
        description  = "Keep only the last ${var.keep_last_n_images} other images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.keep_last_n_images
        }
        action = { type = "expire" }
      }
    ]
  })
}
