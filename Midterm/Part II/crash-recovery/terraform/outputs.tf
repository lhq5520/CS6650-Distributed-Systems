output "alb_dns" {
  description = "ALB DNS name — use this as Locust host"
  value       = "http://${aws_lb.search_alb.dns_name}"
}

output "search_service_url" {
  description = "Search endpoint"
  value       = "http://${aws_lb.search_alb.dns_name}/products/search?q=alpha"
}

output "metrics_url" {
  description = "Metrics endpoint"
  value       = "http://${aws_lb.search_alb.dns_name}/metrics"
}

output "cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "resilience_enabled" {
  value = var.resilience_enabled
}
