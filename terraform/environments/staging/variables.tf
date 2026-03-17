variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "db_password" {
  description = "RDS master password"
  type        = string
  sensitive   = true
}
