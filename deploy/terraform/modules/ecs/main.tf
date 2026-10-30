locals {
  name   = "${var.project}-${var.environment}"
  region = data.aws_region.current.region
}

data "aws_region" "current" {}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${local.name}"
  retention_in_days = var.log_retention_days

  tags = merge(var.tags, { Name = "/ecs/${local.name}" })
}

# ---------------------------------------------------------------------------
# Execution role: used by ECS itself to pull the image, write logs, and read
# the database secret into the container's environment.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "execution" {
  name        = "${local.name}-ecs-execution-role"
  description = "Lets ECS pull images, write logs and read secrets for ${local.name}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "execution_secrets" {
  name = "${local.name}-read-secrets"
  role = aws_iam_role.execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid      = "ReadDatabaseSecret"
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = [var.database_secret_arn]
    }]
  })
}

# ---------------------------------------------------------------------------
# Task role: what the running service itself may do (read and write photos).
# ---------------------------------------------------------------------------
resource "aws_iam_role" "task" {
  name        = "${local.name}-ecs-task-role"
  description = "Runtime permissions for ${local.name}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy" "task" {
  name = "${local.name}-task-policy"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid      = "PhotosBucket"
      Effect   = "Allow"
      Action   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
      Resource = "arn:aws:s3:::${var.photos_bucket_name}/*"
    }]
  })
}

# ---------------------------------------------------------------------------
# Tasks' security group: only the shared ALB reaches them.
# ---------------------------------------------------------------------------
resource "aws_security_group" "tasks" {
  name        = "${local.name}-ecs-tasks-sg"
  description = "Inbound ${var.container_port} from the ALB only for ${local.name} tasks"
  vpc_id      = var.vpc_id

  ingress {
    description     = "App port from the ALB"
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [var.alb_security_group_id]
  }

  egress {
    description = "Allow all outbound (ECR, Secrets Manager, S3, RDS in the VPC)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, { Name = "${local.name}-ecs-tasks-sg" })
}

# ---------------------------------------------------------------------------
# Routing on the shared ALB: requests for host_header go to this target group.
# The ALB's HTTP listener already redirects every host to HTTPS.
# ---------------------------------------------------------------------------
resource "aws_lb_target_group" "this" {
  name_prefix = "mn-tg-"
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  deregistration_delay = 30

  health_check {
    path                = var.health_check_path
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
    timeout             = 5
  }

  tags = merge(var.tags, { Name = "${local.name}-tg" })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_listener_certificate" "this" {
  listener_arn    = var.https_listener_arn
  certificate_arn = var.certificate_arn
}

resource "aws_lb_listener_rule" "this" {
  listener_arn = var.https_listener_arn
  priority     = var.listener_rule_priority

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }

  condition {
    host_header {
      values = [var.host_header]
    }
  }

  tags = merge(var.tags, { Name = "${local.name}-rule" })

  depends_on = [aws_lb_listener_certificate.this]
}

# ---------------------------------------------------------------------------
# ECS cluster, task definition and service, on Fargate Spot
# ---------------------------------------------------------------------------
resource "aws_ecs_cluster" "this" {
  name = local.name

  tags = merge(var.tags, { Name = local.name })
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 1
  }
}

resource "aws_ecs_task_definition" "this" {
  family                   = local.name
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  network_mode             = "awsvpc"
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([{
    name      = "app"
    image     = "${var.ecr_repository_url}:${var.image_tag}"
    essential = true

    portMappings = [{ containerPort = var.container_port, protocol = "tcp" }]

    # The settings internal/platform/config reads.
    environment = [
      { name = "APP_ENV", value = var.environment },
      { name = "PORT", value = tostring(var.container_port) },
      { name = "AWS_REGION", value = local.region },
      { name = "PHOTOS_BUCKET", value = var.photos_bucket_name },
      { name = "PHOTOS_URL", value = var.photos_url },
    ]
    secrets = [
      { name = "DATABASE_URL", valueFrom = "${var.database_secret_arn}:DATABASE_URL::" },
    ]

    healthCheck = {
      command     = ["CMD-SHELL", "wget -q -O /dev/null http://localhost:${var.container_port}${var.health_check_path} || exit 1"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 30
    }

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.this.name
        "awslogs-region"        = local.region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])

  tags = merge(var.tags, { Name = local.name })
}

# One task. minimum_healthy_percent=0 lets ECS stop the old task before starting
# the new one; the circuit breaker rolls back a deploy whose tasks keep failing.
resource "aws_ecs_service" "this" {
  name            = "${local.name}-service"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = 1

  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 1
  }

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100
  health_check_grace_period_seconds  = 60

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = var.public_subnet_ids
    security_groups  = [aws_security_group.tasks.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.this.arn
    container_name   = "app"
    container_port   = var.container_port
  }

  # Tags on the service are copied onto every task it starts, so Fargate costs
  # show up under the project too.
  propagate_tags = "SERVICE"

  # The deploy workflow registers new task definitions; don't revert them.
  lifecycle {
    ignore_changes = [task_definition]
  }

  tags = merge(var.tags, { Name = "${local.name}-service" })

  depends_on = [aws_lb_listener_rule.this]
}
