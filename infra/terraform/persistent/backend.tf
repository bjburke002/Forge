terraform {
  backend "s3" {
    bucket       = "forge-tfstate-241197444395-us-east-1"
    key          = "forge/terraform.persistent.tfstate"
    region       = "us-east-1"
    encrypt      = true
    use_lockfile = true
  }
}