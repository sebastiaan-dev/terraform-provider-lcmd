# Copyright (c) HashiCorp, Inc.

data "lcmd_file" "example" {
  path = var.file_path
}

output "file_contents" {
  value     = data.lcmd_file.example.content
  sensitive = true
}

variable "file_path" {
  description = "Absolute path to a NAS file to fetch"
  type        = string
}
