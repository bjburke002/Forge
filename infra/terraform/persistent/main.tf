terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
  required_version = ">= 1.10.0"
}

# Configure the provider
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = "forge"
      ManagedBy = "terraform"
    }
  }
}

module "ecr" {
  source = "../modules/ecr"

  prefix = "forge"
}

module "github_oidc" {
  source = "../modules/github-oidc"

  github_repository          = var.github_repository
  allowed_branch             = var.allowed_branch
  ecr_repository_arn_pattern = module.ecr.ecr_repository_arn_pattern
}
