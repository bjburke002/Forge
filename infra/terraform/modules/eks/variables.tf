variable "cluster_name" {
  type        = string
  description = "EKS cluster name."
}

variable "subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for the control plane and nodes."
}

variable "allowed_IP_addresses" {
  type        = list(string)
  description = "List of allowed IP addresses for the EKS cluster security group."
}

variable "forge_admin_iam_role_arn" {
  type        = string
  description = "The ARN of the IAM role that will have admin access to the EKS cluster."
}