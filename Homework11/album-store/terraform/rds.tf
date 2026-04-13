# RDS subnet group (needs 2 AZs)
resource "aws_db_subnet_group" "main" {
  name = "album-store-db-subnet"
  subnet_ids = [
    aws_subnet.private_a.id,
    aws_subnet.private_b.id,
  ]

  tags = { Name = "album-store-db-subnet" }
}

resource "aws_db_instance" "postgres" {
  identifier     = "album-store-db"
  engine         = "postgres"
  engine_version = "16"

  instance_class        = var.db_instance_class
  allocated_storage     = 20
  storage_type          = "gp3"
  db_name               = "albumstore"
  username              = var.db_username
  password              = var.db_password
  parameter_group_name  = "default.postgres16"
  publicly_accessible   = false
  skip_final_snapshot   = true
  availability_zone     = var.az
  db_subnet_group_name  = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  tags = { Name = "album-store-db" }
}
