variable "cluster_name" {
  type        = string
  description = "Name of the EKS cluster."
}

variable "domain_name" {
  type        = string
  description = "Domain name for the Route53 Lookup"
}