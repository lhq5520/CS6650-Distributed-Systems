terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# ── Data ──────────────────────────────────────────────────────────────────────

data "aws_caller_identity" "current" {}

# ── VPC ───────────────────────────────────────────────────────────────────────

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  tags = { Name = "${var.project_name}-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${var.project_name}-igw" }
}

# Public subnets (for ALB)
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = ["10.0.1.0/24", "10.0.2.0/24"][count.index]
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true
  tags = { Name = "${var.project_name}-public-${count.index + 1}" }
}

# Private subnets (for ECS)
resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = ["10.0.10.0/24", "10.0.11.0/24"][count.index]
  availability_zone = data.aws_availability_zones.available.names[count.index]
  tags = { Name = "${var.project_name}-private-${count.index + 1}" }
}

data "aws_availability_zones" "available" {}

# Route table for public subnets
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
  tags = { Name = "${var.project_name}-public-rt" }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# NAT Gateway so private subnets can reach AWS APIs
resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id
  tags          = { Name = "${var.project_name}-nat" }
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }
  tags = { Name = "${var.project_name}-private-rt" }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# ── Security Groups ───────────────────────────────────────────────────────────

resource "aws_security_group" "alb" {
  name   = "${var.project_name}-alb-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "ecs" {
  name   = "${var.project_name}-ecs-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ── ALB ───────────────────────────────────────────────────────────────────────

resource "aws_lb" "main" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id
}

resource "aws_lb_target_group" "order_service" {
  name        = "${var.project_name}-order-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/health"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.order_service.arn
  }
}

# ── SNS + SQS ─────────────────────────────────────────────────────────────────

resource "aws_sns_topic" "orders" {
  name = "${var.project_name}-order-processing-events"
}

resource "aws_sqs_queue" "orders" {
  name                       = "${var.project_name}-order-processing-queue"
  visibility_timeout_seconds = 30       # must be >= Lambda/worker processing time
  message_retention_seconds  = 345600   # 4 days
  receive_wait_time_seconds  = 20       # long polling
}

# Allow SNS to send messages to SQS
resource "aws_sqs_queue_policy" "orders" {
  queue_url = aws_sqs_queue.orders.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "sns.amazonaws.com" }
      Action    = "sqs:SendMessage"
      Resource  = aws_sqs_queue.orders.arn
      Condition = {
        ArnEquals = { "aws:SourceArn" = aws_sns_topic.orders.arn }
      }
    }]
  })
}

# Subscribe SQS to SNS
resource "aws_sns_topic_subscription" "sqs" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.orders.arn
}

# ── ECS Cluster ───────────────────────────────────────────────────────────────

resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"
}

# AWS Academy provides pre-built roles — we reference them instead of creating new ones
data "aws_iam_role" "ecs_task" {
  name = "LabRole"
}

data "aws_iam_role" "ecs_execution" {
  name = "LabRole"
}

# CloudWatch log group
resource "aws_cloudwatch_log_group" "order_service" {
  name              = "/ecs/${var.project_name}-order-service"
  retention_in_days = 7
}

# ── ECS Task Definition ───────────────────────────────────────────────────────

resource "aws_ecs_task_definition" "order_service" {
  family                   = "${var.project_name}-order-service"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  task_role_arn            = data.aws_iam_role.ecs_task.arn
  execution_role_arn       = data.aws_iam_role.ecs_execution.arn

  container_definitions = jsonencode([{
    name  = "order-service"
    image = var.ecr_image_uri
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    environment = [
      { name = "PORT",          value = "8080" },
      { name = "SNS_TOPIC_ARN", value = aws_sns_topic.orders.arn },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.orders.url },
      { name = "NUM_WORKERS",   value = tostring(var.num_workers) }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.order_service.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
    healthCheck = {
      command     = ["CMD-SHELL", "wget -qO- http://localhost:8080/health || exit 1"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 10
    }
  }])
}

# ── ECS Service ───────────────────────────────────────────────────────────────

resource "aws_ecs_service" "order_service" {
  name            = "${var.project_name}-order-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.order_service.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.ecs.id]
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.order_service.arn
    container_name   = "order-service"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]
}

# ── Lambda ──────────────────────────────────────────────────────────────────

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

resource "aws_lambda_function" "order_processor" {
  filename         = "../lambda/lambda.zip"
  function_name    = "${var.project_name}-order-processor"
  role             = data.aws_iam_role.lab_role.arn
  handler          = "bootstrap"
  runtime          = "provided.al2"
  source_code_hash = filebase64sha256("../lambda/lambda.zip")
  timeout          = 30

  environment {
    variables = {
      PROJECT = var.project_name
    }
  }
}

resource "aws_sns_topic_subscription" "lambda" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

resource "aws_lambda_permission" "sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.orders.arn
}