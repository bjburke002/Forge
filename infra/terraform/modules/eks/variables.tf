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