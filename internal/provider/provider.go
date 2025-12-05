// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &LcmdProvider{}
var _ provider.ProviderWithFunctions = &LcmdProvider{}
var _ provider.ProviderWithEphemeralResources = &LcmdProvider{}

// LcmdProvider defines the provider implementation.
type LcmdProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// LcmdProviderModel describes the provider data model.
type LcmdProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	User     types.String `tfsdk:"user"`
}

func (p *LcmdProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "lcmd"
	resp.Version = p.version
}

func (p *LcmdProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the NAS API",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "API username used for Basic auth",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "API password used for Basic auth",
				Required:            true,
				Sensitive:           true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "LZC UID that owns the applications",
				Required:            true,
			},
		},
	}
}

func (p *LcmdProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring HashiCups client")

	var data LcmdProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Endpoint.IsUnknown() || data.Endpoint.IsNull() || data.Username.IsUnknown() || data.Username.IsNull() || data.Password.IsUnknown() || data.Password.IsNull() || data.User.IsUnknown() || data.User.IsNull() {
		resp.Diagnostics.AddError("Missing configuration", "endpoint, username, password, and user must be provided")
		return
	}

	client, err := newAPIClient(data.Endpoint.ValueString(), data.Username.ValueString(), data.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to configure API client", err.Error())
		return
	}

	users, err := client.ListUsers(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list UIDs, got error: %s", err))
		return
	}
	uid := data.User.ValueString()
	if !containsUID(users, uid) {
		resp.Diagnostics.AddError("Invalid user", fmt.Sprintf("User %s not found", uid))
		return
	}
	client.User = uid

	resp.DataSourceData = client
	resp.ResourceData = client
}

func containsUID(users []apiUser, uid string) bool {
	return slices.ContainsFunc(users, func(u apiUser) bool { return u.UID == uid })
}

func (p *LcmdProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAppResource,
	}
}

func (p *LcmdProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *LcmdProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func (p *LcmdProvider) Functions(ctx context.Context) []func() function.Function {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LcmdProvider{
			version: version,
		}
	}
}
