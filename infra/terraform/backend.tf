terraform {
  backend "s3" {
    bucket       = "forge-tf-backend-985504337819-us-east-1-an"
    key          = "forge/terraform.tfstate"
    region       = "us-east-1"
    encrypt      = true
    use_lockfile = true
  }
}