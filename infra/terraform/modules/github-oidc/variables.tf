variable "github_repository" {
  type        = string
  description = "The GitHub repository name for the OIDC provider."
}

variable "allowed_branch" {
  type        = string
  description = "The allowed branch for the OIDC provider."
}

variable "ecr_repository_arn_pattern" {
  type        = string
  description = "The ARN pattern for the ECR repository that the OIDC provider will have access to."
}

variable "cluster_name" {
  type        = string
  description = "EKS cluster name."
}