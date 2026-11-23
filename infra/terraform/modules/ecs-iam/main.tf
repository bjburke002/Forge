data "aws_caller_identity" "current" {}
data "aws_region" "current" {}
resource "aws_iam_role" "task_execution_role" {
  name = "${var.cluster_name}-ecs-task-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
        ]
        Effect = "Allow",
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "task_execution_role_attachment" {
  role       = aws_iam_role.task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "ecs_forge_gitops_deploy_role" {
  name = "${var.cluster_name}-ecs-gitops-deploy-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Principal = {
          Federated = var.github_oidc_provider_arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
            "token.actions.githubusercontent.com:sub" = "repo:${var.gitops_repository}:ref:refs/heads/${var.allowed_branch}"
          }
        }
      }
    ]
  })
}

resource "aws_iam_role_policy" "ecs_forge_gitops_deploy_policy" {
  name = "${var.cluster_name}-ecs-deploy"
  role = aws_iam_role.ecs_forge_gitops_deploy_role.id

  policy = templatefile("${path.module}/policies/deploy_policy.json", {
    account_id              = data.aws_caller_identity.current.account_id
    region                  = data.aws_region.current.region
    cloudformation_role_arn = aws_iam_role.forge_ecs_cloudformation_role.arn
  })
}

resource "aws_iam_role" "forge_ecs_cloudformation_role" {
  name = "${var.cluster_name}-ecs-cloudformation-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole"
        ]
        Effect = "Allow",
        Principal = {
          Service = "cloudformation.amazonaws.com"
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

resource "aws_iam_role_policy" "forge_ecs_cloudformation_policy" {
  name = "${var.cluster_name}-ecs-cloudformation-policy"
  role = aws_iam_role.forge_ecs_cloudformation_role.id

  policy = templatefile("${path.module}/policies/cloudformation_policy.json", {
    account_id                  = data.aws_caller_identity.current.account_id
    region                      = data.aws_region.current.region
    hosted_zone_id              = var.hosted_zone_id
    execution_role_arn          = aws_iam_role.task_execution_role.arn
    ecs_infrastructure_role_arn = aws_iam_role.ecs_infrastructure_role.arn
    hook_invoke_role_arn        = aws_iam_role.hook_invoke_role.arn
  })
}

resource "aws_iam_role" "ecs_infrastructure_role" {
  name = "${var.cluster_name}-ecs-infrastructure-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole"
        ],
        Effect = "Allow",
        Principal = {
          Service = "ecs.amazonaws.com"
        }
      }
    ]

  })
}

resource "aws_iam_role_policy_attachment" "ecs_infrastructure_role" {
  role       = aws_iam_role.ecs_infrastructure_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonECSInfrastructureRolePolicyForLoadBalancers"
}

resource "aws_iam_role" "hook_invoke_role" {
  name = "${var.cluster_name}-ecs-hook-invoke-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
        ],
        Effect = "Allow"
        Principal = {
          Service = "ecs.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy" "hook_invoke" {
  name = "${var.cluster_name}-ecs-hook-invoke-policy"
  role = aws_iam_role.hook_invoke_role.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "lambda:InvokeFunction"
      Resource = "arn:aws:lambda:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:function:forge-ecs-canary-hook"
    }]
  })
}

resource "aws_iam_role" "lambda_execution_role" {
  name = "${var.cluster_name}-ecs-lambda-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
          "sts:AssumeRole"
        ],
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_execution_role" {
  role       = aws_iam_role.lambda_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}