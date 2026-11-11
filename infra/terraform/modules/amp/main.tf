resource "aws_prometheus_workspace" "amp" {
  alias = var.amp_alias

  logging_configuration {
    log_group_arn = "${aws_cloudwatch_log_group.amp.arn}:*"
  }
}

resource "aws_prometheus_rule_group_namespace" "amp" {
  name         = "${var.amp_alias}-slo"
  workspace_id = aws_prometheus_workspace.amp.id
  data         = var.slo_rules
}

resource "aws_cloudwatch_log_group" "amp" {
  name              = "/aws/vendedlogs/prometheus/${var.amp_alias}"
  retention_in_days = 7
}

resource "aws_prometheus_query_logging_configuration" "amp" {
  workspace_id = aws_prometheus_workspace.amp.id

  destination {
    cloudwatch_logs {
      log_group_arn = "${aws_cloudwatch_log_group.amp.arn}:*"
    }

    filters {
      qsp_threshold = 1000000
    }
  }
}