# IAM for Karpenter Controller Role
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}
data "aws_partition" "current" {}


resource "aws_iam_role" "controller" {
  name = "${var.cluster_name}-karpenter-controller-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Principal = {
          Service = "pods.eks.amazonaws.com"
        },
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
      }
    ]
  })
}

resource "aws_iam_policy" "controller" {
  name        = "${var.cluster_name}-karpenter-controller-policy"
  description = "Karpenter v1.15.0 controller permissions"
  policy = templatefile("${path.module}/policies/karpenter-controller-v1.15.0.json.tftpl", {
    partition              = data.aws_partition.current.partition
    region                 = data.aws_region.current.region
    account_id             = data.aws_caller_identity.current.account_id
    cluster_name           = var.cluster_name
    node_role_arn          = aws_iam_role.node.arn
    interruption_queue_arn = aws_sqs_queue.interruption.arn
  })
}

resource "aws_iam_role_policy_attachment" "controller" {
  role       = aws_iam_role.controller.name
  policy_arn = aws_iam_policy.controller.arn
}

# IAM for Karpenter Node Role
resource "aws_iam_role" "node" {
  name = "${var.cluster_name}-karpenter-node-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Principal = {
          Service = "ec2.amazonaws.com"
        },
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "node_AmazonEKSWorkerNodePolicy" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "node_AmazonEC2ContainerRegistryPullOnly" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly"
}

resource "aws_iam_role_policy_attachment" "node_AmazonSSMManagedInstanceCore" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# SQS Interruption Queue and EventBridge rules
resource "aws_sqs_queue" "interruption" {
  name                      = var.cluster_name
  message_retention_seconds = var.message_retention_seconds
  sqs_managed_sse_enabled   = true

}

# SQS EventBridge queue policy
resource "aws_sqs_queue_policy" "interruption" {
  queue_url = aws_sqs_queue.interruption.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "SQSSendMessage"
        Effect = "Allow"
        Principal = {
          Service = ["events.amazonaws.com", "sqs.amazonaws.com"]
        }
        Action   = "sqs:SendMessage"
        Resource = aws_sqs_queue.interruption.arn
      },
      {
        Sid       = "SQSDenyInsecureTransport"
        Effect    = "Deny"
        Principal = "*"
        Action    = "sqs:*"
        Resource  = aws_sqs_queue.interruption.arn
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      }
    ]
  })
}

# Mapping EventBridge names to rule patterns
locals {
  interruption_events = {
    scheduled-change = {
      source      = ["aws.health"]
      detail-type = ["AWS Health Event"]
    },
    spot-interruption = {
      source      = ["aws.ec2"]
      detail-type = ["EC2 Spot Instance Interruption Warning"]
    },
    rebalance-recommendation = {
      source      = ["aws.ec2"]
      detail-type = ["EC2 Instance Rebalance Recommendation"]
    },
    instance-state-change = {
      source      = ["aws.ec2"]
      detail-type = ["EC2 Instance State-change Notification"]
    },
    capacity-reservation-interruption = {
      source      = ["aws.ec2"]
      detail-type = ["EC2 Capacity Reservation Instance Interruption Warning"]
    }
  }
}

# EventBridge rules
resource "aws_cloudwatch_event_rule" "karpenter" {
  for_each      = local.interruption_events
  name          = "${var.cluster_name}-karpenter-${each.key}"
  event_pattern = jsonencode(each.value)
}

# EventBridge rule targets aka attachments
resource "aws_cloudwatch_event_target" "karpenter" {
  for_each = aws_cloudwatch_event_rule.karpenter
  rule     = each.value.name
  arn      = aws_sqs_queue.interruption.arn
}

# EKS Pod Identity association for Karpenter
resource "aws_eks_pod_identity_association" "karpenter" {
  cluster_name    = var.cluster_name
  namespace       = "kube-system"
  service_account = "karpenter"
  role_arn        = aws_iam_role.controller.arn
}

# EKS Access Entry for Karpenter 
resource "aws_eks_access_entry" "karpenter" {
  cluster_name  = var.cluster_name
  principal_arn = aws_iam_role.node.arn
  type          = "EC2_LINUX"
}