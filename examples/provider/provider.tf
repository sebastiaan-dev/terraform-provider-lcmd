# Copyright (c) HashiCorp, Inc.

provider "lcmd" {
  endpoint = var.lcmd_endpoint
  username = var.lcmd_username
  password = var.lcmd_password
  user     = var.lcmd_user
}

variable "lcmd_endpoint" {
  description = "Base URL of the NAS API"
  type        = string
}

variable "lcmd_username" {
  description = "API username"
  type        = string
}

variable "lcmd_password" {
  description = "API password"
  type        = string
  sensitive   = true
}

variable "lcmd_user" {
  description = "UID of the user that owns the applications"
  type        = string
}
