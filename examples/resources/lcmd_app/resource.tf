# Copyright (c) HashiCorp, Inc.

resource "lcmd_app" "zitadel" {
  lpk_url   = "https://example.com/file.lpk"
  ephemeral = var.zitadel_ephemeral
}

variable "zitadel_ephemeral" {
  description = "Whether to clear app data when the resource is destroyed"
  type        = bool
  default     = false
}
