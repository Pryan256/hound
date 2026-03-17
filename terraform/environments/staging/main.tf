terraform {
  required_version = ">= 1.8"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    bucket = "hound-terraform-state"
    key    = "staging/terraform.tfstate"
    region = "us-east-1"
    encrypt = true
    dynamodb_table = "hound-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "hound"
      Environment = "staging"
      ManagedBy   = "terraform"
    }
  }
}

module "kms" {
  source      = "../../modules/kms"
  environment = "staging"
}

module "vpc" {
  source      = "../../modules/vpc"
  environment = "staging"
  cidr_block  = "10.0.0.0/16"
}

module "rds" {
  source         = "../../modules/rds"
  environment    = "staging"
  vpc_id         = module.vpc.vpc_id
  subnet_ids     = module.vpc.private_subnet_ids
  kms_key_arn    = module.kms.rds_key_arn
  instance_class = "db.t4g.medium"
}

module "elasticache" {
  source      = "../../modules/elasticache"
  environment = "staging"
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids
  node_type   = "cache.t4g.micro"
}

module "eks" {
  source         = "../../modules/eks"
  environment    = "staging"
  vpc_id         = module.vpc.vpc_id
  subnet_ids     = module.vpc.private_subnet_ids
  kms_key_arn    = module.kms.eks_key_arn
  instance_types = ["t3.medium"]
  min_size       = 2
  max_size       = 5
  desired_size   = 2
}
