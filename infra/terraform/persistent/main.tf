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

# Generating a random Secrets Manager key to use for Grafana-admin
ephemeral "random_password" "grafana_admin" {
  length           = 24
  special          = true
  override_special = "!@#$%&*()-_=+[]{}<>:?"
}

resource "aws_secretsmanager_secret" "grafana_admin" {
  name = "${local.cluster_name}/grafana-admin"
}

resource "aws_secretsmanager_secret_version" "grafana_admin" {
  secret_id = aws_secretsmanager_secret.grafana_admin.id
  secret_string_wo = jsonencode({
    "admin-user"     = "admin"
    "admin-password" = ephemeral.random_password.grafana_admin.result
  })

  secret_string_wo_version = 1
}