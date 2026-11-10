output "forge_amp_arn" {
  value = aws_prometheus_workspace.amp.arn
}

output "forge_amp_id" {
  value = aws_prometheus_workspace.amp.id
}

output "forge_amp_endpoint" {
  value = aws_prometheus_workspace.amp.prometheus_endpoint
}