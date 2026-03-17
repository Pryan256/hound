resource "aws_kms_key" "rds" {
  description             = "Hound ${var.environment} — RDS encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = { Name = "hound-${var.environment}-rds" }
}

resource "aws_kms_alias" "rds" {
  name          = "alias/hound-${var.environment}-rds"
  target_key_id = aws_kms_key.rds.key_id
}

resource "aws_kms_key" "eks" {
  description             = "Hound ${var.environment} — EKS secrets encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = { Name = "hound-${var.environment}-eks" }
}

resource "aws_kms_alias" "eks" {
  name          = "alias/hound-${var.environment}-eks"
  target_key_id = aws_kms_key.eks.key_id
}

resource "aws_kms_key" "app" {
  description             = "Hound ${var.environment} — application-level envelope encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = { Name = "hound-${var.environment}-app" }
}

resource "aws_kms_alias" "app" {
  name          = "alias/hound-${var.environment}-app"
  target_key_id = aws_kms_key.app.key_id
}

variable "environment" {
  type = string
}

output "rds_key_arn" {
  value = aws_kms_key.rds.arn
}

output "eks_key_arn" {
  value = aws_kms_key.eks.arn
}

output "app_key_arn" {
  value = aws_kms_key.app.arn
}
