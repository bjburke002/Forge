output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "cluster_security_group_id" {
  value       = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id
  description = "ID for the EKS-created security group carrying Karpenter tags."
}

output "node_role_arn" {
  value = aws_iam_role.node.arn
}

output "cluster_ca_certificate" {
  value       = aws_eks_cluster.this.certificate_authority[0].data
  description = "Base64 encoded CA certificate. Will need to wrap this with base64decode(...)"
}

output "cluster_endpoint" {
  value = aws_eks_cluster.this.endpoint
}