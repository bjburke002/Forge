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
  source          = "../modules/ecs-alb"
  cluster_name    = local.cluster_name
  vpc_subnets     = module.vpc.public_subnet_ids
  vpc_id          = module.vpc.vpc_id
  certificate_arn = data.aws_acm_certificate.wildcard.arn
}