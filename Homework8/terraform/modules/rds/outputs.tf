output "endpoint" {
  value = aws_db_instance.mysql.endpoint
}

output "hostname" {
  value = aws_db_instance.mysql.address
}

output "port" {
  value = aws_db_instance.mysql.port
}

output "db_name" {
  value = aws_db_instance.mysql.db_name
}
