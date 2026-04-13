# EC2 Security Group
resource "aws_security_group" "ec2" {
  name   = "album-store-ec2"
  vpc_id = aws_vpc.main.id

  # HTTP from anywhere (ChaosArena needs to reach us)
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # SSH for deployment
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # All outbound (S3, RDS, internet)
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "album-store-ec2-sg" }
}

# RDS Security Group — only allow EC2 to connect
resource "aws_security_group" "rds" {
  name   = "album-store-rds"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ec2.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "album-store-rds-sg" }
}
