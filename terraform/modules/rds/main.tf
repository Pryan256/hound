resource "aws_db_subnet_group" "main" {
  name       = "hound-${var.environment}"
  subnet_ids = var.subnet_ids
  tags       = { Name = "hound-${var.environment}" }
}

resource "aws_security_group" "rds" {
  name   = "hound-${var.environment}-rds"
  vpc_id = var.vpc_id

  # Only allow inbound from within VPC (EKS nodes)
  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "hound-${var.environment}-rds" }
}

resource "aws_db_instance" "main" {
  identifier = "hound-${var.environment}"

  engine         = "postgres"
  engine_version = "16.3"
  instance_class = var.instance_class

  allocated_storage     = 50
  max_allocated_storage = 500  # Autoscaling up to 500GB
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = var.kms_key_arn

  db_name  = "hound"
  username = "hound"
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false  # Never public

  multi_az               = true   # Always Multi-AZ (SOC 2)
  deletion_protection    = true
  skip_final_snapshot    = false
  final_snapshot_identifier = "hound-${var.environment}-final"

  backup_retention_period = 30  # 30 days PITR
  backup_window           = "03:00-04:00"
  maintenance_window      = "sun:04:00-sun:05:00"

  performance_insights_enabled = true
  monitoring_interval          = 60
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  tags = { Name = "hound-${var.environment}" }

  lifecycle {
    prevent_destroy = true
  }
}

variable "environment" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "instance_class" { type = string }
variable "db_password" {
  type      = string
  sensitive = true
  default   = ""  # Passed from Secrets Manager in CI
}

output "endpoint" { value = aws_db_instance.main.endpoint }
output "db_name" { value = aws_db_instance.main.db_name }
