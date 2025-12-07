# Copyright (c) HashiCorp, Inc.

resource "lcmd_lpk_build" "example" {
  source = {
    local = {
      path = "path/to/local/lpk/repository"
    }
    # OR
    git = {
      url     = "https://github.com/username/repository.git"
      ref     = "main"
      subpath = "path/to/lpk/repository"
    }
  }

  publish {
    enabled = true
    name    = "example"
  }

  env {
    template_extension = ".tmpl"
    variables = {
      ENV_EXAMPLE = "value"
    }
  }
}
