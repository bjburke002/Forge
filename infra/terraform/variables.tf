variable "aws_region" {
  type        = string
  default     = "us-east-1"
  description = "The AWS region to deploy resources in."
}

variable "allowed_ip_addresses" {
  type        = list(string)
  description = "List of allowed IP addresses for the EKS API."
}

variable "forge_admin_iam_role_arn" {
  type        = string
  description = "The ARN of the IAM role that will have admin access to the EKS cluster."
}

variable "repo_url" {
  type        = string
  description = "URL of the repo that where the application lives."
}

variable "path" {
  type        = string
  description = "Path to the manifests"
}

variable "revision" {
  type        = string
  description = "Branch, e.g. main"
}