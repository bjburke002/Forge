variable "amp_alias" {
  type        = string
  description = "Alias for the AWS Managed Prometheus Workspace"
}

variable "slo_rules" {
  type        = string
  description = "Prometheus rule-group YAML for the SLO rules"
}

variable "ecs_sli_rules" {
  type        = string
  description = "SLI rules for ECS services"
}