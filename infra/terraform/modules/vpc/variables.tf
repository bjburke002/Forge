variable "vpc_cidr_block" {
  type        = string
  description = "The CIDR block for the VPC. Must be a /16."

  validation {
    condition     = can(cidrhost(var.vpc_cidr_block, 0)) && endswith(var.vpc_cidr_block, "/16")
    error_message = "The VPC CIDR block must be a valid /16 CIDR block."
  }
}

variable "number_of_azs" {
  type        = number
  default     = 2
  description = "The number of availability zones to use for the VPC."
}

variable "cluster_name" {
  type        = string
  description = "EKS cluster name used for the karpenter.sh/discovery subnet tag"
}