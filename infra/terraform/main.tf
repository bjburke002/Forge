module "vpc" {
  source = "./modules/vpc"

  vpc_cidr_block = "10.20.0.0/16"
  number_of_azs  = 2
  cluster_name   = "forge"
}

module "eks" {
  source = "./modules/eks"

  cluster_name             = "forge"
  subnet_ids               = module.vpc.private_subnet_ids
  allowed_IP_addresses     = var.allowed_IP_addresses
  forge_admin_iam_role_arn = var.forge_admin_iam_role_arn
}

module "iam" {
  source = "./modules/iam"

  cluster_name = module.eks.cluster_name
  domain_name  = "forgeappservices.com"
}