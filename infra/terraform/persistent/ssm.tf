resource "aws_ssm_parameter" "ecs_task_execution_role_arn" {
  name        = "/forge/shared/ecs-task-execution-role-arn"
  description = "The ECS Task Execution Role's ARN"
  type        = "String"
  value       = module.ecs-iam.task_execution_role_arn
}

resource "aws_ssm_parameter" "hosted_zone_id" {
  name        = "/forge/shared/hosted-zone-id"
  description = "Route53 Hosted Zone ID for forgeappservices.com"
  type        = "String"
  value       = data.aws_route53_zone.forge.zone_id
}