#!/bin/bash
# One-shot deploy script: build + upload + restart
# Usage: ./deploy.sh <EC2_IP> <SSH_KEY_PATH>
#   e.g.: ./deploy.sh 54.200.1.2 ~/.ssh/my-key.pem

set -e

EC2_IP=$1
KEY=$2

if [ -z "$EC2_IP" ] || [ -z "$KEY" ]; then
  echo "Usage: ./deploy.sh <EC2_IP> <SSH_KEY_PATH>"
  exit 1
fi

echo "==> Cross-compiling for Linux..."
cd "$(dirname "$0")"
GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

echo "==> Uploading binary to EC2..."
scp -i "$KEY" -o StrictHostKeyChecking=no server "ec2-user@${EC2_IP}:/opt/album-store/server"

echo "==> Restarting service..."
ssh -i "$KEY" -o StrictHostKeyChecking=no "ec2-user@${EC2_IP}" \
  "sudo chmod +x /opt/album-store/server && sudo systemctl restart album-store"

echo "==> Waiting 2 seconds..."
sleep 2

echo "==> Health check..."
curl -s "http://${EC2_IP}/health"
echo ""

echo "==> Done! Service is running at http://${EC2_IP}"
