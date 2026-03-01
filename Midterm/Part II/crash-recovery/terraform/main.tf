provider "aws" {
  region = var.region
}

# ==================== IAM ====================

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ==================== Networking ====================

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Security Group for ALB (public-facing, port 80)
resource "aws_security_group" "alb_sg" {
  name        = "${var.project_name}-alb-sg"
  description = "ALB security group"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "HTTP from anywhere"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "Recommendation service via ALB"
    from_port   = 8081
    to_port     = 8081
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

# Security Group for ECS tasks
resource "aws_security_group" "ecs_sg" {
  name        = "${var.project_name}-ecs-sg"
  description = "ECS tasks security group"
  vpc_id      = data.aws_vpc.default.id

  # Allow ALB to reach search service on 8080
  ingress {
    description     = "Search service from ALB"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb_sg.id]
  }

  # Allow search service to reach recommendation service on 8081
  # (internal traffic within same security group)
  ingress {
    description = "Rec service internal"
    from_port   = 8081
    to_port     = 8081
    protocol    = "tcp"
    self        = true
  }

  # Allow direct access to rec service for mode switching
  ingress {
    description = "Rec service mode control"
    from_port   = 8081
    to_port     = 8081
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

# ==================== ECS Cluster ====================

resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"
}

# ==================== CloudWatch Log Groups ====================

resource "aws_cloudwatch_log_group" "search" {
  name              = "/ecs/${var.project_name}/search"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "recommendation" {
  name              = "/ecs/${var.project_name}/recommendation"
  retention_in_days = 7
}

# ==================== ALB (for search service) ====================

resource "aws_lb" "search_alb" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb_sg.id]
  subnets            = data.aws_subnets.default.ids
}

resource "aws_lb_target_group" "search_tg" {
  name        = "${var.project_name}-search-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = data.aws_vpc.default.id
  target_type = "ip"

  health_check {
    enabled             = true
    path                = "/health"
    port                = "8080"
    protocol            = "HTTP"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 2
    matcher             = "200"
  }
}

resource "aws_lb_target_group" "recommendation_tg" {
  name        = "${var.project_name}-rec-tg"
  port        = 8081
  protocol    = "HTTP"
  vpc_id      = data.aws_vpc.default.id
  target_type = "ip"

  health_check {
    enabled             = true
    path                = "/health"
    port                = "8081"
    protocol            = "HTTP"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 2
    matcher             = "200"
  }
}

resource "aws_lb_listener" "search_listener" {
  load_balancer_arn = aws_lb.search_alb.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.search_tg.arn
  }
}

resource "aws_lb_listener" "recommendation_listener" {
  load_balancer_arn = aws_lb.search_alb.arn
  port              = 8081
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.recommendation_tg.arn
  }
}

# ==================== Task Definitions ====================

# Recommendation Service Task
resource "aws_ecs_task_definition" "recommendation" {
  family                   = "${var.project_name}-recommendation"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  task_role_arn            = data.aws_iam_role.lab_role.arn
  execution_role_arn       = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = "recommendation"
      image     = var.rec_image_uri
      essential = true
      portMappings = [
        {
          containerPort = 8081
          protocol      = "tcp"
        }
      ]
      environment = [
        { name = "FAILURE_MODE", value = "none" }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.recommendation.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "rec"
        }
      }
    }
  ])
}

# Search Service Task
resource "aws_ecs_task_definition" "search" {
  family                   = "${var.project_name}-search"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  task_role_arn            = data.aws_iam_role.lab_role.arn
  execution_role_arn       = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = "search"
      image     = var.search_image_uri
      essential = true
      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]
      environment = [
        { name = "RESILIENCE",       value = tostring(var.resilience_enabled) },
        { name = "REC_SERVICE_URL",  value = "http://${aws_lb.search_alb.dns_name}:8081" }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.search.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "search"
        }
      }
    }
  ])
}

# ==================== ECS Services ====================

# Recommendation Service
resource "aws_ecs_service" "recommendation" {
  name            = "${var.project_name}-recommendation"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.recommendation.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = data.aws_subnets.default.ids
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.recommendation_tg.arn
    container_name   = "recommendation"
    container_port   = 8081
  }

  depends_on = [aws_lb_listener.recommendation_listener]
}

# Search Service (behind ALB)
resource "aws_ecs_service" "search" {
  name            = "${var.project_name}-search"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.search.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = data.aws_subnets.default.ids
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.search_tg.arn
    container_name   = "search"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.search_listener]
}
