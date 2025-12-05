// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppResource{}
var _ resource.ResourceWithImportState = &AppResource{}

func NewAppResource() resource.Resource {
	return &AppResource{}
}

// AppResource defines the resource implementation.
type AppResource struct {
	client *LcmdClient
}

// LpkResourceModel describes the resource data model.
type LpkResourceModel struct {
	Title     types.String `tfsdk:"title"`
	LpkUrl    types.String `tfsdk:"lpk_url"`
	LpkId     types.String `tfsdk:"lpk_id"`
	Appid     types.String `tfsdk:"appid"`
	Version   types.String `tfsdk:"version"`
	Domain    types.String `tfsdk:"domain"`
	Owner     types.String `tfsdk:"owner"`
	Ephemeral types.Bool   `tfsdk:"ephemeral"`
}

func (r *AppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *AppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "LPK application resource for managing LPK packages on the system",

		Attributes: map[string]schema.Attribute{
			"lpk_url": schema.StringAttribute{
				MarkdownDescription: "URL of the LPK package to install",
				Required:            true,
			},
			"lpk_id": schema.StringAttribute{
				MarkdownDescription: "ID of the LPK package",
				Computed:            true,
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "Title of the LPK package",
				Computed:            true,
			},
			"appid": schema.StringAttribute{
				MarkdownDescription: "Application ID",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Version of the LPK package",
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain of the LPK package",
				Computed:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the LPK package",
				Computed:            true,
			},
			"ephemeral": schema.BoolAttribute{
				MarkdownDescription: "Whether the LPK is ephemeral",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *AppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*LcmdClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *LcmdClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LpkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.InstallApp(ctx, data.LpkUrl.ValueString(), true, data.Ephemeral.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to install LPK, got error: %s", err))
		return
	}

	data.LpkId = stringOrNull(app.LpkID)
	data.Title = stringOrNull(app.Title)
	data.Version = stringOrNull(app.Version)
	data.Domain = stringOrNull(app.Domain)
	data.Appid = stringOrNull(app.AppID)
	data.Owner = stringOrNull(app.Owner)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LpkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Appid.IsNull() || state.Appid.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	app, err := r.client.GetApp(ctx, state.Appid.ValueString())
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("QueryApplication failed", err.Error())
		return
	}

	state.LpkId = stringOrNull(app.LpkID)
	state.Title = stringOrNull(app.Title)
	state.Version = stringOrNull(app.Version)
	state.Domain = stringOrNull(app.Domain)
	state.Appid = stringOrNull(app.AppID)
	state.Owner = stringOrNull(app.Owner)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LpkResourceModel
	var state LpkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.LpkUrl.ValueString() != state.LpkUrl.ValueString() {
		if !state.Appid.IsNull() && state.Appid.ValueString() != "" {
			if err := r.client.DeleteApp(ctx, state.Appid.ValueString(), false); err != nil {
				resp.Diagnostics.AddError("Uninstall failed", err.Error())
				return
			}
		}

		app, err := r.client.InstallApp(ctx, plan.LpkUrl.ValueString(), true, plan.Ephemeral.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError("Install failed", err.Error())
			return
		}

		plan.LpkId = stringOrNull(app.LpkID)
		plan.Title = stringOrNull(app.Title)
		plan.Version = stringOrNull(app.Version)
		plan.Domain = stringOrNull(app.Domain)
		plan.Appid = stringOrNull(app.AppID)
		plan.Owner = stringOrNull(app.Owner)
	} else {
		plan.LpkId = state.LpkId
		plan.Title = state.Title
		plan.Version = state.Version
		plan.Domain = state.Domain
		plan.Appid = state.Appid
		plan.Owner = state.Owner
	}

	plan.Ephemeral = types.BoolValue(plan.Ephemeral.ValueBool())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LpkResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Appid.IsNull() && data.Appid.ValueString() != "" {
		if err := r.client.DeleteApp(ctx, data.Appid.ValueString(), data.Ephemeral.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to uninstall LPK, got error: %s", err))
			return
		}
	}

	tflog.Trace(ctx, "deleted LPK package", map[string]any{
		"lpk_url": data.LpkUrl.ValueString(),
	})
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func stringOrNull(val string) types.String {
	if val == "" {
		return types.StringNull()
	}
	return types.StringValue(val)
}
