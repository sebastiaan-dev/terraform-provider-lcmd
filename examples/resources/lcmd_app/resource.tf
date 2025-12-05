# Copyright (c) HashiCorp, Inc.

resource "lcmd_app" "example" {
  lpk_url   = var.lcmd_lpk_url
  ephemeral = var.lcmd_ephemeral
}

variable "lcmd_lpk_url" {
  description = "LPK package URL to install (https://…)"
  type        = string
}

variable "lcmd_ephemeral" {
  description = "Whether to clear app data when the resource is destroyed"
  type        = bool
  default     = false
}
