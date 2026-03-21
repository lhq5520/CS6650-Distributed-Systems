# =============================================================================
# RDS MySQL Module - HW8 STEP I
# Adapted for Default VPC (public subnets) + AWS Academy LabRole
# =============================================================================

# Security group: only ECS tasks can reach MySQL on 3306
resource "aws_security_group" "rds" {
  name        = lower("${var.service_name}-rds-sg")
  description = "Allow MySQL access from ECS tasks only"
  vpc_id      = var.vpc_id

  ingress {
    description     = "MySQL from ECS"
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = var.ecs_security_group_ids
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.service_name}-rds-sg" }
}

# DB subnet group - uses default VPC subnets (needs ≥2 AZs)
resource "aws_db_subnet_group" "main" {
  name       = lower("${var.service_name}-db-subnet")
  subnet_ids = var.subnet_ids

  tags = { Name = "${var.service_name}-db-subnet" }
}

# RDS MySQL instance
resource "aws_db_instance" "mysql" {
  identifier = lower("${var.service_name}-mysql")

  engine               = "mysql"
  engine_version       = "8.0"
  instance_class       = "db.t3.micro"
  allocated_storage    = 20
  storage_type         = "gp2"

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password
  port     = 3306

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false

  # Assignment requirements
  skip_final_snapshot = true
  deletion_protection = false

  # Minimal settings for assignment
  multi_az                = false
  backup_retention_period = 0
  apply_immediately       = true

  tags = {
    Name = "${var.service_name}-mysql"
    HW   = "8"
  }
}
