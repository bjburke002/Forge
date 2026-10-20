# ECR supports creating repos on push, so we can set up a template policy here to spin them up as needed:
# https://aws.amazon.com/about-aws/whats-new/2025/12/amazon-ecr-creating-repositories-on-push/

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  lifecycle_policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images."
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = var.untagged_expiry_days
        }
        action = {
          type = "expire"
        }
      },
      {
        rulePriority = 2
        description  = "Keep on the most recent build images."
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["sha-"]
          countType     = "imageCountMoreThan"
          countNumber   = var.max_tagged_images
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

resource "aws_ecr_repository_creation_template" "ecr_template" {
  prefix               = var.prefix
  description          = "Settings for every Forge service repository."
  applied_for          = ["CREATE_ON_PUSH"]
  image_tag_mutability = "IMMUTABLE"
  lifecycle_policy     = local.lifecycle_policy

  encryption_configuration {
    encryption_type = "AES256"
  }
}

# Scan on push is a registry setting, not a template setting
resource "aws_ecr_registry_scanning_configuration" "ecr_scan_on_push" {
  scan_type = "BASIC"

  rule {
    scan_frequency = "SCAN_ON_PUSH"
    repository_filter {
      filter      = "${var.prefix}/*"
      filter_type = "WILDCARD"
    }
  }
}