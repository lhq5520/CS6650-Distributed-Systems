# IAM role so EC2 can upload/delete from S3
resource "aws_iam_role" "ec2" {
  name = "album-store-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "s3_access" {
  name = "album-store-s3-access"
  role = aws_iam_role.ec2.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:PutObject", "s3:DeleteObject", "s3:GetObject"]
      Resource = "${aws_s3_bucket.photos.arn}/*"
    }]
  })
}

resource "aws_iam_instance_profile" "ec2" {
  name = "album-store-ec2-profile"
  role = aws_iam_role.ec2.name
}

# Find latest Amazon Linux 2023 AMI
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "app" {
  ami                    = data.aws_ami.al2023.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.ec2.id]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name
  availability_zone      = var.az

  root_block_device {
    volume_size = 20
    volume_type = "gp3"
  }

  user_data = <<-EOF
    #!/bin/bash
    set -e

    # Create app directory
    mkdir -p /opt/album-store

    # Write environment config
    cat > /opt/album-store/.env << 'ENVFILE'
    PORT=80
    DATABASE_URL=postgres://${var.db_username}:${var.db_password}@${aws_db_instance.postgres.address}:5432/albumstore?sslmode=require
    S3_BUCKET=${aws_s3_bucket.photos.bucket}
    AWS_REGION=${var.aws_region}
    ENVFILE

    # Create systemd service
    cat > /etc/systemd/system/album-store.service << 'SVCFILE'
    [Unit]
    Description=Album Store API
    After=network.target

    [Service]
    Type=simple
    EnvironmentFile=/opt/album-store/.env
    ExecStart=/opt/album-store/server
    Restart=always
    RestartSec=3
    LimitNOFILE=65535

    [Install]
    WantedBy=multi-user.target
    SVCFILE

    systemctl daemon-reload
    systemctl enable album-store
  EOF

  tags = { Name = "album-store-app" }
}

# Elastic IP for stable public address
resource "aws_eip" "app" {
  instance = aws_instance.app.id
  domain   = "vpc"

  tags = { Name = "album-store-eip" }
}
