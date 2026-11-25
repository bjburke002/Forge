resource "aws_ecs_cluster" "this" {
  name = var.cluster_name

  setting {
    name  = "containerInsights"
    value = "disabled"
  }
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name = aws_ecs_cluster.this.name

  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    weight            = 100
    capacity_provider = "FARGATE"
  }
}

resource "aws_service_discovery_http_namespace" "dev" {
  name        = "forge-dev"
  description = "ServiceConnect namespace for dev ECS resources"
}

resource "aws_service_discovery_http_namespace" "prod" {
  name        = "forge-prod"
  description = "ServiceConnect namespace for prod ECS resources"
}
