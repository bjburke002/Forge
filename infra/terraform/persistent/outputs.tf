output "ci_role_arn" {
  value = module.github_oidc.ci_oidc_role_arn
}

output "task_execution_role_arn" {
  value = module.ecs-iam.task_execution_role_arn
}

output "gitops_deploy_role_arn" {
  value = module.ecs-iam.gitops_deploy_role_arn
}

output "ecr_registry_url" {
  value = module.ecr.ecr_registry_url
}

output "forge_amp_arn" {
  value = module.amp.forge_amp_arn
}

output "forge_amp_id" {
  value = module.amp.forge_amp_id
}

output "forge_amp_endpoint" {
  value = module.amp.forge_amp_endpoint
}