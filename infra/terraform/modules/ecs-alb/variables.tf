variable "cluster_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "vpc_subnets" {
  type = list(string)
}

variable "certificate_arn" {
  type = string
}

variable "nat_public_ip" {
  type = string
}