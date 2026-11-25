output "cluster_name" {
  value = aws_ecs_cluster_capacity_providers.this.cluster_name
}

output "cluster_arn" {
  value = aws_ecs_cluster.this.arn
}

output "dev_serviceconnect_namespace_arn" {
  value = aws_service_discovery_http_namespace.dev.arn
}

output "prod_serviceconnect_namespace_arn" {
  value = aws_service_discovery_http_namespace.prod.arn
}