# =============================================================================
# HW8 Main Terraform Configuration
# Based on HW5, extended with RDS MySQL + DynamoDB
# =============================================================================

# ---------- HW5 modules (unchanged) ----------

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ---------- HW8 NEW: RDS MySQL (STEP I) ----------

module "rds" {
  source = "./modules/rds"

  service_name           = var.service_name
  vpc_id                 = module.network.vpc_id
  subnet_ids             = module.network.subnet_ids
  ecs_security_group_ids = [module.network.security_group_id]
  db_password            = var.db_password
}

# ---------- HW8 NEW: DynamoDB (STEP II) ----------

module "dynamodb" {
  source       = "./modules/dynamodb"
  service_name = var.service_name
}

# ---------- ECS (updated with DB env vars) ----------

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  region             = var.aws_region

  # HW8 NEW: pass database connection info
  db_backend       = var.db_backend
  db_host          = module.rds.hostname
  db_port          = tostring(module.rds.port)
  db_name          = module.rds.db_name
  db_user          = "admin"
  db_password      = var.db_password
  dynamodb_table   = module.dynamodb.table_name
}

# ---------- Docker build & push ----------

resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context = "../src"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}
