terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "customer_id" { type = string }
variable "region" { type = string }
variable "indexer_count" { type = number }
variable "indexer_instance_type" { type = string }
variable "search_head_count" { type = number }
variable "search_head_instance_type" { type = string }
variable "local_storage_tb" { type = number }
variable "smartstore" { type = bool }
variable "ami_id" {
  type    = string
  default = "ami-00000000000000000" # placeholder machine image
}

locals {
  disk_gb_per_indexer = ceil(var.local_storage_tb * 1000 / var.indexer_count)
  tags                = { customer = var.customer_id, managed_by = "sizeright" }
}

resource "aws_instance" "indexer" {
  count         = var.indexer_count
  ami           = var.ami_id
  instance_type = var.indexer_instance_type
  root_block_device {
    volume_size = local.disk_gb_per_indexer
    encrypted   = true # encryption at rest
  }
  tags = merge(local.tags, { Name = "${var.customer_id}-idx-${count.index + 1}", role = "indexer" })
}

resource "aws_instance" "search_head" {
  count         = var.search_head_count
  ami           = var.ami_id
  instance_type = var.search_head_instance_type
  root_block_device {
    volume_size = 300
    encrypted   = true
  }
  tags = merge(local.tags, { Name = "${var.customer_id}-sh-${count.index + 1}", role = "search_head" })
}

resource "aws_s3_bucket" "smartstore" {
  count  = var.smartstore ? 1 : 0
  bucket = "sizeright-${lower(var.customer_id)}-smartstore"
  tags   = local.tags
}
