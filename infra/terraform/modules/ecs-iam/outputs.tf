output "task_execution_role_arn" {
  value = aws_iam_role.task_execution_role.arn
}

output "gitops_deploy_role_arn" {
  value = aws_iam_role.ecs_forge_gitops_deploy_role.arn
}

output "cloudformation_role_arn" {
  value = aws_iam_role.forge_ecs_cloudformation_role.arn
}