# Copyright (c) HashiCorp, Inc.

resource "lcmd_app" "example" {
  lpk_url   = var.example_lpk_url
  ephemeral = var.example_ephemeral
}

variable "example_lpk_url" {
  description = "URL of the LPK to deploy"
  type        = string
}

variable "example_ephemeral" {
  description = "Whether to clear app data when the resource is destroyed"
  type        = bool
  default     = false
}
