locals {
  cluster_name = "forge-ecs"
}

module "vpc" {
  source = "../modules/vpc"

  vpc_cidr_block = "10.30.0.0/16"
  number_of_azs  = 2
  cluster_name   = local.cluster_name
  eks_tags       = false
}

module "ecs" {
  source = "../modules/ecs"

  cluster_name = local.cluster_name
}

data "aws_acm_certificate" "wildcard" {
  domain      = "forgeappservices.com"
  statuses    = ["ISSUED"]
  most_recent = true
}

module "ecs-alb" {
  source = "../modules/ecs-alb"

  cluster_name    = local.cluster_name
  vpc_subnets     = module.vpc.public_subnet_ids
  vpc_id          = module.vpc.vpc_id
  certificate_arn = data.aws_acm_certificate.wildcard.arn
}

data "aws_prometheus_workspaces" "amp" {
  alias_prefix = "forge-amp"
}

data "aws_prometheus_workspace" "amp" {
  workspace_id = one(data.aws_prometheus_workspaces.amp.workspace_ids)
}

module "ecs-yace" {
  source = "../modules/ecs-yace"

  private_subnet_ids     = module.vpc.private_subnet_ids
  task_security_group_id = module.ecs-alb.task_security_group_id
  cluster_name           = module.ecs.cluster_name
  amp_workspace_arn      = data.aws_prometheus_workspace.amp.arn
  amp_remote_write_url   = "${data.aws_prometheus_workspace.amp.prometheus_endpoint}api/v1/remote_write"
}