resource "aws_eks_cluster" "this" {
  name = var.cluster_name

  access_config {
    authentication_mode                         = "API"
    bootstrap_cluster_creator_admin_permissions = false
  }

  role_arn = aws_iam_role.cluster.arn
  version  = "1.36"

  vpc_config {
    subnet_ids              = var.subnet_ids
    endpoint_public_access  = true
    endpoint_private_access = true
    public_access_cidrs     = var.allowed_ip_addresses
  }

  upgrade_policy {
    support_type = "STANDARD"
  }

  depends_on = [
    aws_iam_role_policy_attachment.cluster_AmazonEKSClusterPolicy
  ]
}

resource "aws_eks_node_group" "this" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "${var.cluster_name}-node-group"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.subnet_ids

  ami_type       = "AL2023_x86_64_STANDARD"
  instance_types = ["t3a.small"]
  scaling_config {
    desired_size = 2
    max_size     = 2
    min_size     = 2
  }

  depends_on = [
    aws_iam_role_policy_attachment.node_AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node_AmazonEC2ContainerRegistryPullOnly,
    aws_eks_addon.vpc_cni,
  ]
}

resource "aws_eks_addon" "vpc_cni" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "vpc-cni"

  configuration_values = jsonencode({
    enableNetworkPolicy = "true"
    env = {
      "ENABLE_PREFIX_DELEGATION" = "true"
      "WARM_PREFIX_TARGET"       = "1"
    }
  })

  pod_identity_association {
    role_arn        = aws_iam_role.vpc_cni.arn
    service_account = "aws-node"
  }

  depends_on = [
    aws_eks_addon.eks_pod_identity_agent,
    aws_iam_role_policy_attachment.vpc_cni_AmazonEKS_CNI_Policy
  ]
}

resource "aws_iam_role" "vpc_cni" {
  name = "${var.cluster_name}-cni-pod-identity-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "vpc_cni_AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.vpc_cni.name
}
resource "aws_iam_role" "yace" {
  name = "${var.cluster_name}-yace-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy" "yace" {
  name        = "${var.cluster_name}-yace-policy"
  path        = "/"
  description = "Yet Another CloudWatch Exporter (YACE) Policy for ALB metrics"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "tag:GetResources",
          "cloudwatch:GetMetricData",
          "cloudwatch:ListMetrics"
        ],
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "yace" {
  role       = aws_iam_role.yace.name
  policy_arn = aws_iam_policy.yace.arn
}

resource "aws_eks_pod_identity_association" "yace" {
  cluster_name    = var.cluster_name
  namespace       = "kube-prometheus"
  service_account = "yace-sa"
  role_arn        = aws_iam_role.yace.arn

  depends_on = [
    aws_eks_cluster.this,
    aws_iam_role_policy_attachment.yace,
    aws_eks_addon.eks_pod_identity_agent
  ]
}

# AWS Managed Workspace info
data "aws_prometheus_workspaces" "amp" {
  alias_prefix = "forge-amp"
}

resource "aws_iam_role" "prometheus" {
  name = "${var.cluster_name}-prometheus-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy" "prometheus" {
  name        = "${var.cluster_name}-prometheus-policy"
  path        = "/"
  description = "Pod identity policy for Prometheus AMP"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "aps:RemoteWrite",
        ]
        Resource = one(data.aws_prometheus_workspaces.amp.arns) # data could potentially pass more than one arn here so making sure
      }
    ]
  })
}

resource "aws_eks_pod_identity_association" "prometheus" {
  cluster_name    = var.cluster_name
  namespace       = "kube-prometheus"
  service_account = "prometheus-sa"
  role_arn        = aws_iam_role.prometheus.arn

  depends_on = [
    aws_eks_cluster.this,
    aws_iam_role_policy_attachment.prometheus,
    aws_eks_addon.eks_pod_identity_agent
  ]
}
resource "aws_iam_role_policy_attachment" "prometheus" {
  role       = aws_iam_role.prometheus.name
  policy_arn = aws_iam_policy.prometheus.arn
}

# Policy info and Pod Identity association for Grafana
resource "aws_iam_role" "grafana" {
  name = "${var.cluster_name}-grafana-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy" "grafana" {
  name        = "${var.cluster_name}-grafana-policy"
  path        = "/"
  description = "Pod Identity Policy for Grafana"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "aps:QueryMetrics",
          "aps:GetSeries",
          "aps:GetLabels",
          "aps:GetMetricMetadata"
        ]
        Resource = one(data.aws_prometheus_workspaces.amp.arns)
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "grafana" {
  role       = aws_iam_role.grafana.name
  policy_arn = aws_iam_policy.grafana.arn
}

resource "aws_eks_pod_identity_association" "grafana" {
  cluster_name    = var.cluster_name
  namespace       = "kube-prometheus"
  service_account = "grafana-sa"
  role_arn        = aws_iam_role.grafana.arn

  depends_on = [
    aws_eks_cluster.this,
    aws_iam_role_policy_attachment.grafana,
    aws_eks_addon.eks_pod_identity_agent
  ]
}
resource "aws_iam_role" "kyverno" {
  name = "${var.cluster_name}-kyverno-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      },
    ]
  })

}

resource "aws_iam_role_policy_attachment" "kyverno_AmazonEC2ContainerRegistryReadOnly" {
  role       = aws_iam_role.kyverno.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_eks_pod_identity_association" "kyverno_admission_controller" {
  cluster_name    = var.cluster_name
  namespace       = "kyverno"
  service_account = "kyverno-admission-controller"
  role_arn        = aws_iam_role.kyverno.arn

  depends_on = [
    aws_eks_cluster.this,
    aws_iam_role_policy_attachment.kyverno_AmazonEC2ContainerRegistryReadOnly,
    aws_eks_addon.eks_pod_identity_agent
  ]
}

resource "aws_eks_pod_identity_association" "kyverno_reports_controller" {
  cluster_name    = var.cluster_name
  namespace       = "kyverno"
  service_account = "kyverno-reports-controller"
  role_arn        = aws_iam_role.kyverno.arn

  depends_on = [
    aws_eks_cluster.this,
    aws_iam_role_policy_attachment.kyverno_AmazonEC2ContainerRegistryReadOnly,
    aws_eks_addon.eks_pod_identity_agent
  ]
}
resource "aws_eks_addon" "kube_proxy" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "kube-proxy"
}

resource "aws_eks_addon" "core_dns" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "coredns"

  depends_on = [
    aws_eks_node_group.this
  ]
}

resource "aws_eks_addon" "eks_pod_identity_agent" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "eks-pod-identity-agent"
}

resource "aws_eks_addon" "aws_ebs_csi_driver" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "aws-ebs-csi-driver"

  depends_on = [
    aws_eks_node_group.this,
    aws_eks_addon.eks_pod_identity_agent,
    aws_iam_role_policy_attachment.ebs_csi_AmazonEBSCSIDriverPolicy
  ]

  pod_identity_association {
    role_arn        = aws_iam_role.ebs_csi.arn
    service_account = "ebs-csi-controller-sa"
  }
}

resource "aws_eks_addon" "metrics_server" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "metrics-server"

  depends_on = [
    aws_eks_node_group.this
  ]
}

resource "aws_iam_role" "cluster" {
  name = "${var.cluster_name}-eks-cluster-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "cluster_AmazonEKSClusterPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.cluster.name
}

resource "aws_iam_role" "node" {
  name = "${var.cluster_name}-eks-node-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
        ]
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "node_AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node.name
}

resource "aws_iam_role_policy_attachment" "node_AmazonEC2ContainerRegistryPullOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly"
  role       = aws_iam_role.node.name
}

resource "aws_iam_role" "ebs_csi" {
  name = "${var.cluster_name}-ebs-csi-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
        Effect = "Allow"
        Principal = {
          Service = "pods.eks.amazonaws.com"
        }
      },
    ]
  })

}

resource "aws_iam_role_policy_attachment" "ebs_csi_AmazonEBSCSIDriverPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
  role       = aws_iam_role.ebs_csi.name
}

# Adding the Karpenter discovery tag to the cluster security group
resource "aws_ec2_tag" "karpenter_discovery" {
  resource_id = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id
  key         = "karpenter.sh/discovery"
  value       = var.cluster_name
}

# Granting forge-admin access to the cluster 
resource "aws_eks_access_entry" "forge_admin" {
  cluster_name  = aws_eks_cluster.this.name
  principal_arn = var.forge_admin_iam_role_arn
  type          = "STANDARD"
}

# Grant the role cluster-admin permission 
resource "aws_eks_access_policy_association" "forge_admin" {
  cluster_name  = aws_eks_cluster.this.name
  principal_arn = aws_eks_access_entry.forge_admin.principal_arn
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"

  access_scope {
    type = "cluster"
  }
}