#!/bin/bash
# ============================================================
# AWS Deployment Script for Crash & Recovery Demo
# ============================================================
# Prerequisites:
#   - AWS CLI configured (aws configure + session token)
#   - Docker running
#   - Terraform installed
# ============================================================

set -e

REGION="us-west-2"
SEARCH_REPO="search-service"
REC_REPO="rec-service"

echo "============================================"
echo "  Step 1: Create ECR Repositories"
echo "============================================"

# Create ECR repos (ignore error if already exists)
aws ecr create-repository --repository-name $SEARCH_REPO --region $REGION 2>/dev/null || true
aws ecr create-repository --repository-name $REC_REPO --region $REGION 2>/dev/null || true

# Get account ID and ECR base URL
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_BASE="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

SEARCH_URI="${ECR_BASE}/${SEARCH_REPO}:latest"
REC_URI="${ECR_BASE}/${REC_REPO}:latest"

echo "Search image URI: $SEARCH_URI"
echo "Rec image URI:    $REC_URI"

echo ""
echo "============================================"
echo "  Step 2: Login to ECR"
echo "============================================"

aws ecr get-login-password --region $REGION | \
  docker login --username AWS --password-stdin $ECR_BASE

echo ""
echo "============================================"
echo "  Step 3: Build & Push Images"
echo "============================================"

# Build and push search service
echo "Building search-service..."
docker buildx build --platform linux/amd64 \
  -t $SEARCH_URI --push ./search-service/

# Build and push recommendation service
echo "Building rec-service..."
docker buildx build --platform linux/amd64 \
  -t $REC_URI --push ./recommendation-service/

echo ""
echo "============================================"
echo "  Step 4: Deploy with Terraform"
echo "============================================"

cd terraform

# Write terraform.tfvars
cat > terraform.tfvars << EOF
region             = "${REGION}"
search_image_uri   = "${SEARCH_URI}"
rec_image_uri      = "${REC_URI}"
resilience_enabled = false
EOF

echo "Created terraform.tfvars"
echo ""
echo "Initializing Terraform..."
terraform init

echo ""
echo "Deploying Phase 1 (BROKEN - no resilience)..."
terraform apply -auto-approve

echo ""
echo "============================================"
echo "  DEPLOYMENT COMPLETE!"
echo "============================================"
terraform output

echo ""
echo "Next steps:"
echo "  1. Wait ~2 min for services to become healthy"
echo "  2. Test: curl \$(terraform output -raw alb_dns)/health"
echo "  3. Run Locust baseline test"
echo "  4. Trigger failure: find rec service task IP, then:"
echo "     curl 'http://<REC_TASK_IP>:8081/mode?mode=slow'"
echo "  5. Run Locust again to observe crash"
echo ""
echo "To switch to Phase 2 (WITH resilience):"
echo "  cd terraform"
echo "  sed -i 's/resilience_enabled = false/resilience_enabled = true/' terraform.tfvars"
echo "  terraform apply -auto-approve"
