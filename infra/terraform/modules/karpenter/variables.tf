variable "cluster_name" {
  type        = string
  description = "Name of the EKS cluster Karpenter will control."
}

variable "message_retention_seconds" {
  type        = number
  default     = 300
  description = "Message retention time for the Karpenter SQS queue. Default is 300"
}