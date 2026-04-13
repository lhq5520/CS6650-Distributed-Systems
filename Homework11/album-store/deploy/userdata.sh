#!/bin/bash
# EC2 User Data script for deploying album-store
# Instance: Amazon Linux 2023 / c6i.xlarge in us-west-2a

set -e

# Update system
yum update -y

# Install Go (for building on the instance, or just SCP the binary)
# Option 1: SCP pre-built binary (faster)
# Option 2: Build on instance
yum install -y golang git

# Create app directory
mkdir -p /opt/album-store
cd /opt/album-store

# Environment variables (replace with actual values)
cat > /opt/album-store/.env << 'ENVEOF'
PORT=80
DATABASE_URL=postgres://postgres:YOUR_PASSWORD@YOUR_RDS_ENDPOINT:5432/albumstore?sslmode=require
S3_BUCKET=YOUR_BUCKET_NAME
AWS_REGION=us-west-2
ENVEOF

# Create systemd service
cat > /etc/systemd/system/album-store.service << 'EOF'
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
EOF

# To deploy:
# 1. Cross-compile: GOOS=linux GOARCH=amd64 go build -o server ./cmd/server
# 2. SCP the binary: scp server ec2-user@<IP>:/opt/album-store/server
# 3. Edit .env with actual values
# 4. systemctl daemon-reload && systemctl enable album-store && systemctl start album-store

echo "Setup complete. Upload your binary and configure .env"
