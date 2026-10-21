variable "aws_region" {
  type        = string
  default     = "us-east-1"
  description = "The AWS region to deploy resources in."
}

variable "github_repository" {
  type        = string
  description = "The GitHub repository name for the OIDC provider."
}

variable "allowed_branch" {
  type        = string
  description = "The allowed branch for the OIDC provider."
}
