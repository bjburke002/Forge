output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vpc_private_subnet_ids" {
  value = module.vpc.private_subnet_ids
}

output "alb_https_listener_arn" {
  value = module.ecs-alb.alb_https_listener_arn
}

output "alb_dns_name" {
  value = module.ecs-alb.alb_dns_name
}

output "alb_hosted_zone_id" {
  value = module.ecs-alb.alb_hosted_zone_id
}

output "task_security_group_id" {
  value = module.ecs-alb.task_security_group_id
}

output "ecs_cluster_name" {
  value = module.ecs.cluster_name
}

output "ecs_cluster_arn" {
  value = module.ecs.cluster_arn
}