# Examples

This directory contains examples that are mostly used for documentation, but can also be run/tested manually via the Terraform CLI.

The document generation tool looks for files in the following locations by default. All other *.tf files besides the ones mentioned below are ignored by the documentation tool. This is useful for creating examples that can run and/or are testable even if some parts are not relevant for the documentation.

* **provider/provider.tf** example file for the provider index page
* **data-sources/`full data source name`/data-source.tf** example file for the named data source page
* **resources/`full resource name`/resource.tf** example file for the named data source page

## Provider configuration

`examples/provider/provider.tf` shows the minimum configuration required to talk to the NAS service. The provider now only needs the API `endpoint` and the target `user` UID:

```hcl
provider "lcmd" {
  endpoint = var.lcmd_endpoint
  user     = var.lcmd_user
}
```

Use variables (or environment variables) to set those values so that the same configuration can target different NAS environments.

## App resource workflow

`examples/resources/lcmd_app/resource.tf` demonstrates an end-to-end workflow:

1. `resource "lcmd_lpk_build" "zitadel"` pulls a local Zitadel LPK source tree, renders any `*.tmpl` files with the provided `env.variables`, and uploads the artifact with `publish.enabled = true`.
2. `resource "lcmd_app" "zitadel"` installs the freshly built package using the `lpk_url` emitted by the build resource. Toggling the `zitadel_ephemeral` variable changes whether uninstalling the resource clears persisted app data.

Supplying variables such as `zitadel_source_path` and `zitadel_master_key` keeps secrets and file paths out of version control while still allowing the example to run unchanged.

## File data source

`examples/data-sources/lcmd_file/data-source.tf` shows how to fetch an arbitrary file from the NAS filesystem so that Terraform can output or persist it elsewhere. This is useful for downloading generated certificates or tokens after provisioning.
