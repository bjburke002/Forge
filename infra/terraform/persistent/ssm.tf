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

resource "aws_ssm_parameter" "ecs_cloudformation_role_arn" {
  name        = "/forge/shared/ecs-cloudformation-role-arn"
  description = "The CloudFormation Service role ARN"
  type        = "String"
  value       = module.ecs-iam.cloudformation_role_arn
}

resource "aws_ssm_parameter" "ecs_infrastructure_role_arn" {
  name        = "/forge/shared/ecs-infrastructure-role-arn"
  description = "The ECS Infrastructure Role ARN"
  type        = "String"
  value       = module.ecs-iam.ecs_infrastructure_role_arn
}

resource "aws_ssm_parameter" "hook_invoke_role_arn" {
  name        = "/forge/shared/ecs-hook-invoke-role-arn"
  description = "The Lambda Hook Invocation role ARN"
  type        = "String"
  value       = module.ecs-iam.hook_invoke_role_arn
}

resource "aws_ssm_parameter" "lambda_execution_role_arn" {
  name        = "/forge/shared/ecs-canary-hook-role-arn"
  description = "The Lambda canary hook execution role ARN"
  type        = "String"
  value       = module.ecs-iam.lambda_execution_role_arn
}