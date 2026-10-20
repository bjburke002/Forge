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

module "vpc" {
  source = "./modules/vpc"

  vpc_cidr_block = "10.20.0.0/16"
  number_of_azs  = 2
  cluster_name   = "forge"
}

module "ecr" {
  source = "./modules/ecr"

  prefix = "forge"
}