locals {
  function_name = "${var.cluster_name}-canary-hook"
}

resource "aws_security_group" "lambda" {
  name        = "${var.cluster_name}-ecs-lambda-sg"
  description = "Allow outbound traffic"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${var.cluster_name}-ecs-lambda-sg"
  }
}

resource "aws_vpc_security_group_egress_rule" "lambda_outbound_443" {
  security_group_id = aws_security_group.lambda.id
  ip_protocol       = "tcp"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
}

resource "aws_vpc_security_group_egress_rule" "lambda_outbound_8443" {
  security_group_id = aws_security_group.lambda.id
  ip_protocol       = "tcp"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 8443
  to_port           = 8443
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = 7
}

data "archive_file" "canary_hook" {
  type        = "zip"
  source_file = "${path.module}/src/lambda_function.py"
  output_path = "${path.module}/build/canary_hook.zip"
}

resource "aws_lambda_function" "canary" {
  filename      = data.archive_file.canary_hook.output_path
  function_name = local.function_name
  role          = var.lambda_execution_role_arn
  handler       = "lambda_function.lambda_handler"
  code_sha256   = data.archive_file.canary_hook.output_base64sha256
  timeout       = 30

  runtime = "python3.15"

  vpc_config {
    subnet_ids         = var.private_subnet_ids
    security_group_ids = [aws_security_group.lambda.id]
  }

  depends_on = [aws_cloudwatch_log_group.lambda]
}