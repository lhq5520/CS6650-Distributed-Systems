variable "service_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "ecs_security_group_ids" {
  type        = list(string)
  description = "SG IDs of ECS tasks (for ingress)"
}

variable "db_name" {
  type    = string
  default = "shopping_cart_db"
}

variable "db_username" {
  type    = string
  default = "admin"
}

variable "db_password" {
  type      = string
  sensitive = true
}
