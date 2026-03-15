output "alb_dns_name" {
  description = "ALB DNS name — use this as your Locust host"
  value       = "http://${aws_lb.main.dns_name}"
}

output "sns_topic_arn" {
  description = "SNS topic ARN"
  value       = aws_sns_topic.orders.arn
}

output "sqs_queue_url" {
  description = "SQS queue URL"
  value       = aws_sqs_queue.orders.url
}

output "ecr_push_commands" {
  description = "Commands to build and push Docker image"
  value       = <<-EOT
    aws ecr create-repository --repository-name ${var.project_name}-order-service
    aws ecr get-login-password | docker login --username AWS --password-stdin ${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com
    docker build -t ${var.project_name}-order-service ./order-service
    docker tag ${var.project_name}-order-service:latest ${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${var.project_name}-order-service:latest
    docker push ${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${var.project_name}-order-service:latest
  EOT
}
