locals {
  cluster_name = "forge"
}

module "ecr" {
  source = "../modules/ecr"

  prefix = local.cluster_name
}

module "github_oidc" {
  source = "../modules/github-oidc"

  cluster_name               = local.cluster_name
  github_repository          = var.github_repository
  allowed_branch             = var.allowed_branch
  ecr_repository_arn_pattern = module.ecr.ecr_repository_arn_pattern
}

resource "aws_iam_service_linked_role" "spot" {
  aws_service_name = "spot.amazonaws.com"
}

data "aws_route53_zone" "forge" {
  name         = "forgeappservices.com"
  private_zone = false
}

module "acm" {
  source = "../modules/acm"

  domain_name     = "forgeappservices.com"
  route53_zone_id = data.aws_route53_zone.forge.zone_id
}