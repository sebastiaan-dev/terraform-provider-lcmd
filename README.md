# Terraform LCMD MicroServer Provider

_This template repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework). The template repository built on the [Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) can be found at [terraform-provider-scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding). See [Which SDK Should I Use?](https://developer.hashicorp.com/terraform/plugin/framework-benefits) in the Terraform documentation for additional information._

This repository provides a LCMD MicroServer [Terraform](https://www.terraform.io) provider, it is available in the [registry](https://registry.terraform.io/providers/sebastiaan-dev/lcmd/latest). Currently it supports installing and removing LPK applications.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24
- [Terraform LCMD Service](https://github.com/sebastiaan-dev/lcmd-terraform-service)

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

The `lcmd_lpk_build` resource builds an LPK from a local or git source, optionally publishes it, and exposes details such as the download URL and SHA. You can provide build-time environment variables and template rendering instructions via the optional `env` block:

```hcl
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

resource "lcmd_lpk_build" "example" {
  source {
    local {
      path = "./app"
    }
  }

  publish {
    enabled = true
    name    = "my-app"
  }

  env {
    template_extension = ".j2"
    variables = {
      API_URL = "https://api.example.com"
      SECRET  = var.backend_secret
    }
  }
}
```

All files beneath the source directory whose name ends with the configured template extension (defaults to `.tmpl`) are rendered using Go templates with the values from `env.variables`. The rendered content is written to a sibling file that shares the same name minus the template extension (for example, `config.yaml.tmpl` becomes `config.yaml`). If a template references a variable that is not defined, the resource raises a clear error pointing to the missing environment key. The provided variables are also exported to the build command's environment, so build tooling can reference them with standard shell expansion.

### Fetching NAS files

Use the `lcmd_file` data source to read certificate files or generated tokens from the NAS filesystem so you can reuse them in Terraform:

```hcl
data "lcmd_file" "nas_cert" {
  path = "/lzcapp/certs/nas-cert.pem"
}

output "nas_cert_pem" {
  value     = data.lcmd_file.nas_cert.content
  sensitive = true
}

resource "local_file" "nas_cert" {
  filename = "./nas-cert.pem"
  content  = base64decode(data.lcmd_file.nas_cert.content_base64)
}
```

The data source returns both UTF-8 content and a base64 representation for binary-safe workflows, along with the file size and SHA256 checksum to detect drift.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

## Publishing

To publish a new version of the provider, run `git tag vx.x.x` and `git push origin vx.x.x`.
