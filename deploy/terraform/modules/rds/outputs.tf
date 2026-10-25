output "db_instance_address" {
  description = "Hostname of the RDS instance."
  value       = aws_db_instance.this.address
}

output "db_instance_port" {
  description = "Port of the RDS instance."
  value       = aws_db_instance.this.port
}

output "database_secret_arn" {
  description = "ARN of the secret holding DATABASE_URL and the connection details."
  value       = aws_secretsmanager_secret.database.arn
}

output "database_secret_name" {
  description = "Name of the secret holding DATABASE_URL and the connection details."
  value       = aws_secretsmanager_secret.database.name
}
