#!/bin/bash
# Cleanup all AWS resources
set -e

REGION="us-west-2"

echo "Destroying Terraform resources..."
cd terraform
terraform destroy -auto-approve
cd ..

echo "Deleting ECR images and repos..."
aws ecr batch-delete-image --repository-name search-service --image-ids imageTag=latest --region $REGION 2>/dev/null || true
aws ecr batch-delete-image --repository-name rec-service --image-ids imageTag=latest --region $REGION 2>/dev/null || true
aws ecr delete-repository --repository-name search-service --force --region $REGION 2>/dev/null || true
aws ecr delete-repository --repository-name rec-service --force --region $REGION 2>/dev/null || true

echo "✓ All resources cleaned up!"
