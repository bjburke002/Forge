resource "aws_iam_role" "lb_pod_identity" {
  name = "${var.cluster_name}-lb-pod-identity-role"

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

resource "aws_iam_policy" "lb_pod_identity_role_policy" {
  name   = "${var.cluster_name}-lb-pod-identity-policy"
  policy = file("${path.module}/policies/aws-load-balancer-controller-v3.6.0.json")

  description = "AWS Load Balancer Controller v3.6.0 permissions"
}

resource "aws_iam_role_policy_attachment" "lb_pod_identity_role_attachment" {
  role       = aws_iam_role.lb_pod_identity.name
  policy_arn = aws_iam_policy.lb_pod_identity_role_policy.arn
}

resource "aws_eks_pod_identity_association" "lb_pod_identity_association" {
  cluster_name    = var.cluster_name
  namespace       = "kube-system"
  service_account = "aws-load-balancer-controller"
  role_arn        = aws_iam_role.lb_pod_identity.arn
}

data "aws_route53_zone" "forge" {
  name         = var.domain_name
  private_zone = false
}

resource "aws_iam_role" "external_dns" {
  name = "${var.cluster_name}-external-dns-pod-identity-role"

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

resource "aws_iam_policy" "external_dns" {
  name = "${var.cluster_name}-external-dns-pod-identity-policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ChangeRecordsInForgeZone"
        Effect   = "Allow"
        Action   = ["route53:ChangeResourceRecordSets"]
        Resource = [data.aws_route53_zone.forge.arn]
      },
      {
        Sid    = "DiscoverZonesAndRecords"
        Effect = "Allow"
        Action = [
          "route53:ListHostedZones",
          "route53:ListResourceRecordSets",
          "route53:ListTagsForResources",
        ]
        Resource = ["*"]
      }
    ]
  })

  description = "ExternalDNS permissions for the forgeappservices.com zone"
}

resource "aws_iam_role_policy_attachment" "external_dns" {
  role       = aws_iam_role.external_dns.name
  policy_arn = aws_iam_policy.external_dns.arn
}

resource "aws_eks_pod_identity_association" "external_dns" {
  cluster_name    = var.cluster_name
  namespace       = "external-dns"
  service_account = "external-dns"
  role_arn        = aws_iam_role.external_dns.arn
}