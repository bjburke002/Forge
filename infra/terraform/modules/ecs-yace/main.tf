data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "aws_ssm_parameter" "forge_task_execution_role" {
  name = "/forge/shared/ecs-task-execution-role-arn"
}

resource "aws_iam_role" "yace_task_role" {
  name = "${var.cluster_name}-yace-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole"
        ]
        Effect = "Allow",
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
        }
      }
    ]
  })

}

resource "aws_iam_role_policy" "yace_role_policy" {
  name = "${var.cluster_name}-yace-policy"
  role = aws_iam_role.yace_task_role.id

  policy = templatefile("${path.module}/policies/yace.json", {
    aws_amp_arn = var.amp_workspace_arn
  })
}

resource "aws_cloudwatch_log_group" "yace" {
  name              = "platform-yace"
  retention_in_days = 7
}

# YACE container config, coming from the EKS config
locals {
  yace_config = <<-EOT
    apiVersion: v1alpha1
    sts-region: ${data.aws_region.current.region}
    discovery:
      exportedTagsOnMetrics:
        AWS/ApplicationELB:
          - Service
          - Environment
      jobs:
        - type: AWS/ApplicationELB
          regions:
            - ${data.aws_region.current.region}
          dimensionNameRequirements:
            - LoadBalancer
            - TargetGroup
          searchTags:
            - key: Project
              value: forge
            - key: Backend
              value: ecs
          delay: 120
          metrics:
            - name: RequestCount
              statistics: [Sum]
              period: 60
              length: 180
              nilToZero: true
            - name: HTTPCode_Target_5XX_Count
              statistics: [Sum]
              period: 60
              length: 180
              nilToZero: true
            - name: HealthyHostCount
              statistics: [Minimum]
              period: 60
              length: 180
  EOT
  adot_config = <<-EOT
    extensions:
      sigv4auth:
        region: ${data.aws_region.current.region}
        service: aps

    receivers:
      prometheus:
        config:
          global:
            scrape_interval: 60s
          scrape_configs:
            - job_name: yace
              static_configs:
                - targets: ["localhost:5000"]

    exporters:
      prometheusremotewrite:
        endpoint: ${var.amp_remote_write_url}
        auth:
          authenticator: sigv4auth

    service:
      extensions: [sigv4auth]
      pipelines:
        metrics:
          receivers: [prometheus]
          exporters: [prometheusremotewrite]
  EOT
}

resource "aws_ecs_task_definition" "yace" {
  family                   = "platform-yace"
  execution_role_arn       = data.aws_ssm_parameter.forge_task_execution_role.value
  task_role_arn            = aws_iam_role.yace_task_role.arn
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  container_definitions = jsonencode([{
    name                   = "yace"
    image                  = "quay.io/prometheuscommunity/yet-another-cloudwatch-exporter:v0.68.0"
    essential              = true
    user                   = "65532"
    readonlyRootFilesystem = true
    entryPoint             = ["sh", "-c"]
    command = [
      "printf '%s' \"$YACE_CONFIG\" | exec yace --config.file=/dev/stdin --scraping-interval=60"
    ]
    environment = [
      { name = "YACE_CONFIG", value = local.yace_config }
    ]
    portMappings = [{ containerPort = 5000, protocol = "tcp" }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.yace.name
        "awslogs-region"        = data.aws_region.current.region
        "awslogs-stream-prefix" = "yace"
      }
    }
    },
    {
      name                   = "adot"
      image                  = "public.ecr.aws/aws-observability/aws-otel-collector:v0.51.0"
      essential              = true
      readonlyRootFilesystem = true
      environment = [
        { name = "AOT_CONFIG_CONTENT", value = local.adot_config }
      ]
      dependsOn = [{ containerName = "yace", condition = "START" }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.yace.name
          "awslogs-region"        = data.aws_region.current.region
          "awslogs-stream-prefix" = "adot"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "yace" {
  name            = "platform-yace"
  cluster         = var.cluster_name
  task_definition = aws_ecs_task_definition.yace.arn
  desired_count   = 1
  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 1
  }
  network_configuration {
    subnets         = var.private_subnet_ids
    security_groups = [var.task_security_group_id]
  }
}