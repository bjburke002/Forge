output "ci_role_arn" {
  value = module.github_oidc.ci_oidc_role_arn
}

output "ecr_registry_url" {
  value = module.ecr.ecr_registry_url
}