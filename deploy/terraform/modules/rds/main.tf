# ---------------------------------------------------------------------------
# Security group: Postgres from inside the VPC, plus any admin CIDRs given.
# A CIDR rule rather than a source-SG rule, because the ECS tasks' security
# group lives in the prod environment's state.
# ---------------------------------------------------------------------------
resource "aws_security_group" "this" {
  name        = "${var.project}-rds-sg"
  description = "Allow PostgreSQL inbound from within the VPC"
  vpc_id      = var.vpc_id

  ingress {
    description = "PostgreSQL from within the VPC"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  dynamic "ingress" {
    for_each = length(var.admin_cidr_blocks) > 0 ? [1] : []
    content {
      description = "PostgreSQL admin access from allowed CIDRs"
      from_port   = 5432
      to_port     = 5432
      protocol    = "tcp"
      cidr_blocks = var.admin_cidr_blocks
    }
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, { Name = "${var.project}-rds-sg" })
}

resource "aws_db_subnet_group" "this" {
  name        = "${var.project}-db-subnet-group"
  description = "Subnet group for the ${var.project} RDS instance"
  subnet_ids  = var.subnet_ids

  tags = merge(var.tags, { Name = "${var.project}-db-subnet-group" })
}

# RDS passwords can't contain / @ " or spaces; the rest is URL-encoded below.
resource "random_password" "master" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_db_instance" "this" {
  identifier        = var.db_identifier
  engine            = "postgres"
  engine_version    = var.db_engine_version
  instance_class    = var.db_instance_class
  allocated_storage = var.db_allocated_storage
  storage_type      = "gp3"

  db_name  = var.db_name
  username = var.db_username
  password = random_password.master.result

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.this.id]
  publicly_accessible    = length(var.admin_cidr_blocks) > 0

  storage_encrypted       = true
  deletion_protection     = var.deletion_protection
  backup_retention_period = var.backup_retention_period
  multi_az                = false

  # Keep a recovery snapshot even if protection is ever turned off first.
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.db_identifier}-final-snapshot"

  copy_tags_to_snapshot = true

  tags = merge(var.tags, { Name = var.db_identifier })
}

# ---------------------------------------------------------------------------
# The connection settings the service reads, as one Secrets Manager secret.
# ECS injects DATABASE_URL into the container from this secret's JSON key.
# The app connects as the master user: there is one service and one database.
# ---------------------------------------------------------------------------
resource "aws_secretsmanager_secret" "database" {
  name                    = "${var.project}-database"
  description             = "Postgres connection for ${var.project}. Managed by Terraform."
  recovery_window_in_days = 0

  tags = merge(var.tags, { Name = "${var.project}-database" })
}

resource "aws_secretsmanager_secret_version" "database" {
  secret_id = aws_secretsmanager_secret.database.id
  secret_string = jsonencode({
    DATABASE_URL = format(
      "postgres://%s:%s@%s:%d/%s?sslmode=require",
      var.db_username,
      urlencode(random_password.master.result),
      aws_db_instance.this.address,
      aws_db_instance.this.port,
      var.db_name,
    )
    host     = aws_db_instance.this.address
    port     = aws_db_instance.this.port
    dbname   = var.db_name
    username = var.db_username
    password = random_password.master.result
  })
}
