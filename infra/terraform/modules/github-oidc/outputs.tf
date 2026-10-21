output "github_oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.github.arn
}

output "ci_oidc_role_arn" {
  value       = aws_iam_role.ci_oidc_role.arn
  description = "Value for the role-to-assume in GitHut Cations workflows."
}