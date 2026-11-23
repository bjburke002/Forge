output "alb_https_listener_arn" {
  value = aws_lb_listener.https.arn
}

output "canary_listener_arn" {
  value = aws_lb_listener.canary.arn
}
output "alb_dns_name" {
  value = aws_lb.this.dns_name
}

output "alb_hosted_zone_id" {
  value = aws_lb.this.zone_id
}

output "task_security_group_id" {
  value = aws_security_group.task.id
}

output "alb_full_name" {
  value = aws_lb.this.arn_suffix
}