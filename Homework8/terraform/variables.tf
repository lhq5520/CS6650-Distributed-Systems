# ---------- HW5 original variables ----------

variable "aws_region" {
  type    = string
  default = "us-west-2"
}

variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650HW8"
}

variable "container_port" {
  type    = number
  default = 5173
}

variable "ecs_count" {
  type    = number
  default = 1
}

variable "log_retention_days" {
  type    = number
  default = 7
}

# ---------- HW8 new variables ----------

variable "db_password" {
  description = "RDS MySQL master password"
  type        = string
  sensitive   = true
  default     = "ChangeMe123!"
}

variable "db_backend" {
  description = "Which database backend to use: mysql or dynamodb"
  type        = string
  default     = "mysql"
}
