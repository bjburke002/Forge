resource "aws_ssm_parameter" "ecs_cluster_name" {
  name        = "/forge/ecs/cluster-name"
  description = "The name of the ECS cluster"
  type        = "String"
  value       = module.ecs.cluster_name
}

resource "aws_ssm_parameter" "alb_https_listener_arn" {
  name        = "/forge/ecs/https-listener-arn"
  description = "The ARN of the ECS cluster's ALB HTTPS listener"
  type        = "String"
  value       = module.ecs-alb.alb_https_listener_arn
}

resource "aws_ssm_parameter" "task_security_group_id" {
  name        = "/forge/ecs/task-security-group-id"
  description = "The ECS Task Security Group ID"
  type        = "String"
  value       = module.ecs-alb.task_security_group_id
}

resource "aws_ssm_parameter" "vpc_private_subnet_ids" {
  name        = "/forge/ecs/private-subnet-ids"
  description = "The subnet IDs for ECS private subnets"
  type        = "StringList"
  value       = join(",", module.vpc.private_subnet_ids)
}

resource "aws_ssm_parameter" "alb_dns_name" {
  name        = "/forge/ecs/alb-dns-name"
  description = "DNS name of the ECS ALB"
  type        = "String"
  value       = module.ecs-alb.alb_dns_name
}

resource "aws_ssm_parameter" "alb_zone_id" {
  name        = "/forge/ecs/alb-zone-id"
  description = "The ALB's Hosted Zone ID, needed for the Route53 alias"
  type        = "String"
  value       = module.ecs-alb.alb_hosted_zone_id
}

resource "aws_ssm_parameter" "ecs_vpc_id" {
  name        = "/forge/ecs/vpc-id"
  description = "The ECS VPC ID"
  type        = "String"
  value       = module.vpc.vpc_id
}
resource "aws_ssm_parameter" "canary_listener_rule_arn" {
  name        = "/forge/ecs/test-listener-arn"
  description = "ARN of the ALB listener used for canary tests."
  type        = "String"
  value       = module.ecs-alb.canary_listener_arn
}

resource "aws_ssm_parameter" "lambda_function_arn" {
  name        = "/forge/ecs/canary-hook-arn"
  description = "Canary hook Lambda function ARN"
  type        = "String"
  value       = module.ecs-canary.lambda_function_arn
}