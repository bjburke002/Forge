output "cluster_name" {
  value = aws_ecs_cluster_capacity_providers.this.cluster_name
}

output "cluster_arn" {
  value = aws_ecs_cluster.this.arn
}