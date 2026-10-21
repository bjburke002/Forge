output "ecr_registry_url" {
  description = "Push to <registry_url>/<service>:<tag>"
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.region}.amazonaws.com/${var.prefix}"
}

output "ecr_repository_arn_pattern" {
  description = "For scoping the CI role's permissions."
  value       = "arn:aws:ecr:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:repository/${var.prefix}/*"
}