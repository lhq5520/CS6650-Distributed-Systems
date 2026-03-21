# ---------- HW5 original ----------
variable "service_name" {
  type = string
}

variable "image" {
  type = string
}

variable "container_port" {
  type = number
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_ids" {
  type = list(string)
}

variable "execution_role_arn" {
  type = string
}

variable "task_role_arn" {
  type = string
}

variable "log_group_name" {
  type = string
}

variable "ecs_count" {
  type    = number
  default = 1
}

variable "region" {
  type = string
}

variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}

# ---------- HW8 new: database ----------
variable "db_backend" {
  type    = string
  default = "mysql"
}

variable "db_host" {
  type    = string
  default = ""
}

variable "db_port" {
  type    = string
  default = "3306"
}

variable "db_name" {
  type    = string
  default = "shopping_cart_db"
}

variable "db_user" {
  type    = string
  default = "admin"
}

variable "db_password" {
  type      = string
  default   = ""
  sensitive = true
}

variable "dynamodb_table" {
  type    = string
  default = ""
}