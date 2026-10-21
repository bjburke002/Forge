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

provider "helm" {
  kubernetes = {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_ca_certificate)

    exec = {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", "forge", "--region", "us-east-1"]
    }
  }
}