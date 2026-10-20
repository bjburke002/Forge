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
