output "node_role_name" {
  value = aws_iam_role.node.name
}

output "controller_role_arn" {
  value = aws_iam_role.controller.arn
}

output "interruption_queue_name" {
  value = aws_sqs_queue.interruption.name
}

