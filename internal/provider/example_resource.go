// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	lzcsdk "gitee.com/linakesi/lzc-sdk/lang/go"
	common "gitee.com/linakesi/lzc-sdk/lang/go/common"
	"gitee.com/linakesi/lzc-sdk/lang/go/sys"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LpkResource{}
var _ resource.ResourceWithImportState = &LpkResource{}

func NewExampleResource() resource.Resource {
	return &LpkResource{}
}

// LpkResource defines the resource implementation.
type LpkResource struct {
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

func (r *LpkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lpk"
}

func (r *LpkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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

func (r *LpkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LpkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LpkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sdkCtx := lzcsdk.WithRealUID(ctx, r.client.User)
	boolVal := true

	ireq := &sys.InstallLPKRequest{
		LpkUrl:       data.LpkUrl.ValueString(),
		WaitUnitDone: &boolVal,
	}
	iresp, err := r.client.Gw.PkgManager.InstallLPK(sdkCtx, ireq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to install LPK, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "installed LPK package", map[string]any{
		"lpk_url": ireq.LpkUrl,
	})

	task := iresp.TaskInfo
	apps, err := r.client.Gw.PkgManager.QueryApplication(sdkCtx, &sys.QueryApplicationRequest{
		DeployIds: []string{*task.RealPkgId},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to query application, got error: %s", err))
		return
	}

	if len(apps.GetInfoList()) == 0 {
		resp.Diagnostics.AddError("No applications found", "No applications found")
		return
	}
	app := apps.GetInfoList()[0]

	data.LpkId = types.StringValue(*task.RealPkgId)
	data.Title = types.StringValue(app.GetTitle())
	data.Version = types.StringValue(app.GetVersion())
	data.Domain = types.StringValue(app.GetDomain())
	data.Appid = types.StringValue(app.GetAppid())
	data.Owner = types.StringValue(app.GetOwner())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LpkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LpkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uidsResp, err := r.client.Gw.Users.ListUIDs(ctx, &common.ListUIDsRequest{})
	if err != nil || len(uidsResp.GetUids()) == 0 {
		resp.Diagnostics.AddError("UID lookup failed", "ListUIDs returned no entries or error")
		return
	}
	sdkCtx := lzcsdk.WithRealUID(ctx, uidsResp.GetUids()[0])

	appid := state.Appid.ValueString()
	if appid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	apps, err := r.client.Gw.PkgManager.QueryApplication(sdkCtx, &sys.QueryApplicationRequest{
		DeployIds: []string{appid},
	})
	if err != nil {
		resp.Diagnostics.AddError("QueryApplication failed", err.Error())
		return
	}
	if len(apps.GetInfoList()) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	app := apps.GetInfoList()[0]
	state.LpkId = types.StringValue(state.LpkId.ValueString())
	state.Title = types.StringValue(app.GetTitle())
	state.Version = types.StringValue(app.GetVersion())
	state.Domain = types.StringValue(app.GetDomain())
	state.Appid = types.StringValue(app.GetAppid())
	state.Owner = types.StringValue(app.GetOwner())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LpkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LpkResourceModel
	var state LpkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sdkCtx := lzcsdk.WithRealUID(ctx, r.client.User)

	if plan.LpkUrl.ValueString() != state.LpkUrl.ValueString() {
		if state.Appid.ValueString() != "" {
			_, err := r.client.Gw.PkgManager.Uninstall(sdkCtx, &sys.UninstallRequest{
				Appid:     state.Appid.ValueString(),
				ClearData: false,
			})
			if err != nil {
				resp.Diagnostics.AddError("Uninstall failed", err.Error())
				return
			}
		}

		wait := true
		installResp, err := r.client.Gw.PkgManager.InstallLPK(sdkCtx, &sys.InstallLPKRequest{
			LpkUrl:       plan.LpkUrl.ValueString(),
			WaitUnitDone: &wait,
		})
		if err != nil {
			resp.Diagnostics.AddError("Install failed", err.Error())
			return
		}

		task := installResp.GetTaskInfo()
		apps, err := r.client.Gw.PkgManager.QueryApplication(sdkCtx, &sys.QueryApplicationRequest{
			DeployIds: []string{task.GetRealPkgId()},
		})
		if err != nil || len(apps.GetInfoList()) == 0 {
			resp.Diagnostics.AddError("QueryApplication failed", "no app returned after reinstall")
			return
		}

		app := apps.GetInfoList()[0]
		plan.LpkId = types.StringValue(task.GetRealPkgId())
		plan.Title = types.StringValue(app.GetTitle())
		plan.Version = types.StringValue(app.GetVersion())
		plan.Domain = types.StringValue(app.GetDomain())
		plan.Appid = types.StringValue(app.GetAppid())
		plan.Owner = types.StringValue(app.GetOwner())
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

func (r *LpkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LpkResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sdkCtx := lzcsdk.WithRealUID(ctx, r.client.User)

	request := &sys.UninstallRequest{
		Appid:     data.Appid.ValueString(),
		ClearData: data.Ephemeral.ValueBool(),
	}

	_, err := r.client.Gw.PkgManager.Uninstall(sdkCtx, request)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to uninstall LPK, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted LPK package", map[string]any{
		"lpk_url": data.LpkUrl.ValueString(),
	})
}

func (r *LpkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
