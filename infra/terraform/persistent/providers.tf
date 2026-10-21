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
