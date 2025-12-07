# Copyright (c) HashiCorp, Inc.

provider "lcmd" {
  endpoint = var.lcmd_endpoint
  user     = var.lcmd_user
}

variable "lcmd_endpoint" {
  description = "Base URL of the NAS API"
  type        = string
}

variable "lcmd_user" {
  description = "UID of the user that owns the applications"
  type        = string
}
