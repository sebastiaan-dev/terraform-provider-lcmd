// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &FileDataSource{}

type FileDataSource struct {
	client *LcmdClient
}

type FileDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Path          types.String `tfsdk:"path"`
	Content       types.String `tfsdk:"content"`
	ContentBase64 types.String `tfsdk:"content_base64"`
	SHA256        types.String `tfsdk:"sha256"`
	Size          types.Int64  `tfsdk:"size"`
}

func NewFileDataSource() datasource.DataSource {
	return &FileDataSource{}
}

func (d *FileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (d *FileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a file from the NAS filesystem and returns its contents.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Internal identifier derived from path and checksum.",
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Absolute path to the file on the NAS.",
			},
			"content": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Raw file contents decoded as UTF-8 when possible.",
			},
			"content_base64": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "File contents encoded as base64 for binary-safe usage.",
			},
			"sha256": schema.StringAttribute{
				Computed:    true,
				Description: "Hex-encoded SHA256 checksum of the file contents.",
			},
			"size": schema.Int64Attribute{
				Computed:    true,
				Description: "Size of the file in bytes.",
			},
		},
	}
}

func (d *FileDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*LcmdClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *LcmdClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *FileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "")
		return
	}
	var data FileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Path.IsUnknown() || data.Path.IsNull() || data.Path.ValueString() == "" {
		resp.Diagnostics.AddError("Missing path", "path must be provided")
		return
	}
	apiResp, err := d.client.FetchFile(ctx, data.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Fetch error", err.Error())
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(apiResp.ContentBase64)
	if err != nil {
		resp.Diagnostics.AddError("Decode error", err.Error())
		return
	}
	data.ID = types.StringValue(buildFileID(data.Path.ValueString(), apiResp.SHA256))
	data.ContentBase64 = types.StringValue(apiResp.ContentBase64)
	data.Content = types.StringValue(string(decoded))
	data.SHA256 = types.StringValue(apiResp.SHA256)
	data.Size = types.Int64Value(apiResp.Size)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func buildFileID(path string, sha string) string {
	input := []byte(path + ":" + sha)
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}
