locals {
  cluster_name = "forge"
}

module "vpc" {
  source = "./modules/vpc"

  vpc_cidr_block = "10.20.0.0/16"
  number_of_azs  = 2
  cluster_name   = local.cluster_name
}

module "eks" {
  source = "./modules/eks"

  cluster_name             = local.cluster_name
  subnet_ids               = module.vpc.private_subnet_ids
  allowed_ip_addresses     = var.allowed_ip_addresses
  forge_admin_iam_role_arn = var.forge_admin_iam_role_arn
}

module "iam" {
  source = "./modules/iam"

  cluster_name = module.eks.cluster_name
  domain_name  = "forgeappservices.com"
}

module "argocd" {
  source = "./modules/argocd"

  repo_url = var.repo_url
  revision = var.revision
  path     = var.path


  depends_on = [module.eks]
}

