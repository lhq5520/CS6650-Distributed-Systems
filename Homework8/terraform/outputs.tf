output "rds_endpoint" {
  description = "MySQL RDS endpoint"
  value       = module.rds.endpoint
}

output "dynamodb_table" {
  description = "DynamoDB table name"
  value       = module.dynamodb.table_name
}
