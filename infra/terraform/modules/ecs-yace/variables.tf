variable "cluster_name" {
  type = string
}

variable "amp_workspace_arn" {
  type = string
}

variable "amp_remote_write_url" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "task_security_group_id" {
  type = string
}