variable "aws_region" {
  default = "us-west-2"
}

variable "az" {
  default = "us-west-2a"
}

variable "db_username" {
  default = "postgres"
}

variable "db_password" {
  description = "RDS master password"
  type        = string
  sensitive   = true
}

variable "key_name" {
  description = "EC2 SSH key pair name (must already exist in AWS)"
  type        = string
}

variable "instance_type" {
  default = "c6i.xlarge"
}

variable "db_instance_class" {
  default = "db.t4g.micro"
}
