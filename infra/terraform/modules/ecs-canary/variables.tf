variable "cluster_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "lambda_execution_role_arn" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}