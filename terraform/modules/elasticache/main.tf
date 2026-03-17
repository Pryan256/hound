resource "aws_security_group" "redis" {
  name   = "hound-${var.environment}-redis"
  vpc_id = var.vpc_id

  ingress {
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  tags = { Name = "hound-${var.environment}-redis" }
}

resource "aws_elasticache_subnet_group" "main" {
  name       = "hound-${var.environment}"
  subnet_ids = var.subnet_ids
}

resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "hound-${var.environment}"
  description          = "Hound ${var.environment} Redis"

  node_type            = var.node_type
  num_cache_clusters   = 2  # Primary + 1 replica
  port                 = 6379

  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]

  at_rest_encryption_enabled  = true
  transit_encryption_enabled  = true
  automatic_failover_enabled  = true
  multi_az_enabled             = true

  snapshot_retention_limit = 7
  snapshot_window          = "03:30-04:30"
}

variable "environment" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "node_type" { type = string }

output "primary_endpoint" {
  value = aws_elasticache_replication_group.main.primary_endpoint_address
}
