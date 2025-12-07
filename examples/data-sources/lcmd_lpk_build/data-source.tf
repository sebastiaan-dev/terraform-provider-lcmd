# Copyright (c) HashiCorp, Inc.

data "lcmd_lpk_build" "example" {
  source {
    local {
      path = "/local/path/to/lpk/repository"
    }
    git {
      url     = "https://github.com/your/repo.git"
      ref     = "main"
      subpath = "path/to/lpk"
    }
  }

  publish {
    enabled = true
    name    = "example"
  }

  env {
    template_extension = ".tmpl"
    variables = {
      SECRET_EXAMPLE = var.secret_example
    }
  }
}

variable "secret_example" {
  description = "Secret example"
  type        = string
  sensitive   = true
}
